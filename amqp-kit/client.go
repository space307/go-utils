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

const defaultReconnectAfterDuration = 2 * time.Second

type Publisher interface {
	Publish(exchange, key, corID string, body []byte) (err error)
	PublishWithTracing(ctx context.Context, exchange, key, corID string, body []byte) (err error)
}

type Client struct {
	conn           *connection
	connLock       sync.RWMutex
	subs           []SubscribeInfo
	config         *Config
	stopClientChan chan struct{}
}

type SubscribeInfo struct {
	Name        string
	Queue       string
	Workers     int
	SubExchange string
	PubExchange string
	E           endpoint.Endpoint
	Dec         DecodeRequestFunc
	Enc         EncodeResponseFunc
	O           []SubscriberOption
}

type Config struct {
	Address                string
	User                   string
	Password               string
	VirtualHost            string
	ChannelPoolSize        int
	ChannelRetryCount      int
	ReconnectAfterDuration time.Duration
}

func New(cfg *Config) (*Client, error) {
	ser := &Client{
		config:         cfg,
		stopClientChan: make(chan struct{}),
	}

	return ser, ser.init()
}

// MakeDsn - creates dsn from config
func MakeDsn(c *Config) string {
	u := url.URL{Scheme: "amqp", User: url.UserPassword(c.User, c.Password), Host: c.Address, Path: "/" + c.VirtualHost}
	return u.String()
}

func (si *SubscribeInfo) KeyName() string {
	return strings.Replace(si.Queue, "_", ".", -1)
}

func (c *Client) init() error {
	err := c.reconnect()
	if err != nil {
		return err
	}

	return nil
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

	for {
		select {
		case <-c.stopClientChan:
			return
		default:
			afterDuration := c.config.ReconnectAfterDuration
			if afterDuration.Seconds() == 0 {
				afterDuration = defaultReconnectAfterDuration
			}
			<-time.After(afterDuration)
			err = c.reconnect()
			if err != nil {
				log.Warnf("AMQP: reconnection err %v", err)
			} else {
				return
			}
		}
	}
}

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

		subscribers[si.Queue] = &s
	}

	log.Info(`subscribers checked`)

	for _, sub := range subscribers {
		for i := 0; i < sub.Workers; i++ {
			go func(si *SubscribeInfo) {
				for {
					select {
					case <-c.stopClientChan:
						log.Errorf(`stop client chan receiver for q: %s, n: %s, sub_exchange: %s`, si.Queue,
							si.Name, si.SubExchange)
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
	}

	return
}

func (c *Client) receive(si *SubscribeInfo) error {
	conn := c.getConnection()
	ch, err := conn.getChan()
	if err != nil {
		return fmt.Errorf("AMQP: Channel err: %s", err.Error())
	}
	defer conn.putChan(ch)

	if err = DeclareAndBind(ch.c, si.SubExchange, si.Queue, si.KeyName(), 1); err != nil {
		return fmt.Errorf("AMQP: Declare and bind err: %s", err.Error())
	}

	msgs, err := ch.c.Consume(si.Queue, si.Name, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf(" Channel consume err: %s", err.Error())
	}
	fun := NewSubscriber(si.E, si.Dec, si.Enc, si.O...).ServeDelivery(ch.c)

	var d amqp.Delivery
	var ok bool

	for {
		select {
		case d, ok = <-msgs:
		}
		if !ok {
			return fmt.Errorf(" Close channel error ")
		}

		fun(&d)
	}
}

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

	if err := ch.QueueBind(queue, key, exchange, false, nil); err != nil {
		return err
	}

	return nil
}

// Publish publishing some message to given exchange with key and correlationID
func (c *Client) Publish(exchange, key, corID string, body []byte) (err error) {
	pub := amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}

	return c.send(exchange, key, &pub)
}

func (c *Client) send(exchange, key string, pub *amqp.Publishing) (err error) {
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
