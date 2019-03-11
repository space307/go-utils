package amqp_kit

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/endpoint"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

const defaultReconnectAfterDuration = 500 * time.Millisecond

// Publisher interface use for publish amqp - message
type Publisher interface {
	Publish(exchange, key, corID string, body []byte) error
	PublishWithTracing(ctx context.Context, exchange, key, corID string, body []byte) error
}

// Client struct contains amqp - connection/reconnection and methods for pub/sub amqp message
type Client struct {
	conn           *connection
	connLock       sync.RWMutex
	subs           []SubscribeInfo
	config         *Config
	stopClientChan chan struct{}
}

// SubscriberInfo struct use for describe consumer for amqp
type SubscribeInfo struct {
	Name     string
	Queue    string
	Key      string
	Exchange string
	Workers  int
	E        endpoint.Endpoint
	Dec      DecodeRequestFunc
	Enc      EncodeResponseFunc
	O        []SubscriberOption
}

// Config struct initialize config for Client struct
type Config struct {
	Address                string
	User                   string
	Password               string
	VirtualHost            string
	ChannelPoolSize        int
	ChannelRetryCount      int
	ReconnectAfterDuration time.Duration
}

// New AMQP Client with connection
func New(cfg *Config) (*Client, error) {
	ser := &Client{
		config:         cfg,
		stopClientChan: make(chan struct{}),
	}

	if err := ser.reconnect(); err != nil {
		return nil, err
	}

	return ser, nil
}

// MakeDsn - creates dsn from config
func MakeDsn(c *Config) string {
	u := url.URL{Scheme: "amqp", User: url.UserPassword(c.User, c.Password), Host: c.Address, Path: "/" + c.VirtualHost}
	return u.String()
}

func (si *SubscribeInfo) keyName() string {
	return strings.Replace(si.Queue, "_", ".", -1)
}

func (c *Client) reconnect() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	conn := newConnection(c.config, c)
	err := conn.connect()
	if err != nil {
		return err
	}
	c.conn = conn

	return nil
}

func (c *Client) getConnection() *connection {
	c.connLock.RLock()
	defer c.connLock.RUnlock()

	return c.conn
}

func (c *Client) onCloseWithErr(conn *connection, err error) {
	log.Warnf("AMQP: connection closed, err %v", err)

	afterDuration := c.config.ReconnectAfterDuration
	if afterDuration == 0 {
		afterDuration = defaultReconnectAfterDuration
	}

	for {
		select {
		case <-c.stopClientChan:
			return
		default:
			time.Sleep(afterDuration)
			err = c.reconnect()
			if err != nil {
				log.Warnf("AMQP: reconnection err %v", err)
			} else {
				return
			}
		}
	}
}

// Serve start consumers and listen amqp - messages
func (c *Client) Serve(si []SubscribeInfo) (err error) {
	subscribers := make(map[string]*SubscribeInfo)
	for _, si := range si {
		if _, ok := subscribers[si.Queue]; ok {
			return fmt.Errorf("amqp_kit: duplicate queue entry: '%s' ", si.Queue)
		}
		s := si

		if s.Workers == 0 {
			s.Workers = 1
		}

		if s.Key == "" {
			s.Key = s.keyName()
		}

		subscribers[si.Queue] = &s
	}

	for q, sub := range subscribers {
		for i := 0; i < sub.Workers; i++ {
			go func(si *SubscribeInfo) {
				for {
					select {
					case <-c.stopClientChan:
						log.Errorf(`stop client chan receiver for q: %s, n: %s, sub_exchange: %s`, si.Queue,
							si.Name, si.Exchange)
						return
					default:
						if err := c.receive(si); err != nil {
							log.Errorf(`Receive for q: %s, name: %s, err: %s`, si.Queue, si.Name, err)
						}
					}
					time.Sleep(1 * time.Second)
				}
			}(sub)
		}

		t := time.Now().Add(5 * time.Second)
		for {
			conn := c.getConnection()
			ch, err := conn.getChan()
			if err != nil {
				return fmt.Errorf("AMQP: Channel err: %s", err.Error())
			}

			if time.Now().After(t) {
				return fmt.Errorf(`worker wait timeout`)
			}

			// the channel will be closed if error
			qu, err := ch.c.QueueInspect(q)
			if err != nil {
				continue
			}
			conn.putChan(ch)

			if qu.Consumers == sub.Workers {
				break
			}
		}
	}

	return nil
}

func (c *Client) receive(si *SubscribeInfo) error {
	conn := c.getConnection()
	ch, err := conn.getChan()
	if err != nil {
		return fmt.Errorf("AMQP: Channel err: %s", err.Error())
	}
	defer conn.putChan(ch)

	if err = DeclareAndBind(ch.c, si.Exchange, si.Queue, si.Key, 1); err != nil {
		return fmt.Errorf("AMQP: Declare and bind err: %s", err.Error())
	}

	msgs, err := ch.c.Consume(si.Queue, si.Name, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("Channel consume err: %s ", err.Error())
	}
	fun := NewSubscriber(si.E, si.Dec, si.Enc, si.O...).ServeDelivery(ch.c)

	for d := range msgs {
		if si.Key != d.RoutingKey {
			log.Errorf(`error routing key, expected: %s, real: %s`, si.Key, d.RoutingKey)
			_ = d.Ack(false)
			continue
		}

		fun(&d)
	}

	return fmt.Errorf("Close channel error ")
}

// DeclareAndBind create exchange, queue and create bind by key
func DeclareAndBind(ch *amqp.Channel, exchange, queue, key string, qos int) error {
	err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	if err := ch.Qos(qos, 0, false); err != nil {
		return err
	}

	return ch.QueueBind(queue, key, exchange, false, nil)
}

// Publish publishing some message to given exchange with key and correlationID
func (c *Client) Publish(exchange, key, corID string, body []byte) error {
	pub := amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}

	return c.send(exchange, key, &pub)
}

func (c *Client) send(exchange, key string, pub *amqp.Publishing) error {
	// add retry
	conn := c.getConnection()
	channel, err := conn.getChan()
	if err != nil {
		return fmt.Errorf("AMQP: Channel err: %s", err.Error())
	}
	defer conn.putChan(channel)

	if err = channel.c.Publish(exchange, key, false, false, *pub); err != nil {
		channel.err = err
		return fmt.Errorf("AMQP: Exchange Publish err: %s", err.Error())
	}

	return nil
}

// GetAMQPConnection get simple amqp.Connection
func (c *Client) GetAMQPConnection() *amqp.Connection {
	conn := c.getConnection()

	return conn.amqpConn
}

// Ping is health - check for amqp connection
func (c *Client) Ping() error {
	conn := c.getConnection()
	ch, err := conn.amqpConn.Channel()
	if err != nil {
		return fmt.Errorf("AMQP: Channel create err: %s", err.Error())
	}

	err = ch.Close()
	if err != nil {
		return fmt.Errorf("AMQP: Channel close err: %s", err.Error())
	}

	return nil
}

// Close closes the existed connection
func (c *Client) Close() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	close(c.stopClientChan)
	if c.conn != nil {
		return c.conn.close()
	}

	return nil
}
