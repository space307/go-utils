package amqp_kit

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-kit/kit/endpoint"
	"github.com/streadway/amqp"
)

type SubscribeInfo struct {
	Q        string
	Name     string
	Exchange string
	Key      string
	Workers  int
	E        endpoint.Endpoint
	Dec      DecodeRequestFunc
	Enc      EncodeResponseFunc
	O        []SubscriberOption
}

func (si *SubscribeInfo) QueueName() string {
	if si.Q != "" {
		return si.Q
	}
	return strings.Replace(si.Key, ".", "_", -1)
}

type Config struct {
	Address     string //host:port
	User        string
	Password    string
	VirtualHost string
}

type subscriber struct {
	ex, key string
	num     int
	h       func(deliv *amqp.Delivery)
}

type Server struct {
	Conn *amqp.Connection
	ch   *amqp.Channel
	subs []SubscribeInfo
}

func NewServer(s []SubscribeInfo, con *amqp.Connection) *Server {
	return &Server{
		Conn: con,
		subs: s,
	}
}

func (s *Server) Serve() (err error) {

	if s.ch, err = s.Conn.Channel(); err != nil {
		return
	}

	subscribers := make(map[string]*subscriber)

	for _, si := range s.subs {
		q := si.QueueName()

		if _, ok := subscribers[q]; ok {
			return fmt.Errorf("amqp_kit: duplicate queue entry: '%s' ", q)
		}

		workers := 1
		if si.Workers > 0 {
			workers = si.Workers
		}

		subscribers[q] = &subscriber{
			ex:  si.Exchange,
			key: si.Key,
			num: workers,
			h:   NewSubscriber(si.E, si.Dec, si.Enc, si.O...).ServeDelivery(s.ch),
		}
	}

	for queue, sub := range subscribers {

		msgs, err := s.ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			return err
		}

		/*
			TODO: declare here
		*/

		for i := 0; i < sub.num; i++ {

			go func(ch <-chan amqp.Delivery, sub *subscriber) {
				var (
					d  amqp.Delivery
					ok bool
				)
				for {
					select {
					case d, ok = <-ch:
					}
					if !ok {
						// TODO:
						return
					}
					sub.h(&d)
				}
			}(msgs, sub)
		}
	}

	return
}

func (s *Server) reconnect(count int, exchange, queue string, keys ...string) (ch *amqp.Channel, err error) {

	ch, err = s.Conn.Channel()
	if err != nil {
		return
	}

	if err = ch.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	); err != nil {
		return
	}

	var q amqp.Queue

	if q, err = ch.QueueDeclare(
		queue, // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return
	}

	if err = ch.Qos(
		count, // prefetch count
		0,     // prefetch size
		false, // global
	); err != nil {
		return err
	}

	for _, key := range keys {
		if err = ch.QueueBind(
			q.Name,   // queue name
			key,      // routing key
			exchange, // exchange
			false,
			nil,
		); err != nil {
			return
		}
	}

	return ch, nil
}

func (s *Server) Stop() error {
	return s.ch.Close()
}

// MakeDsn - creates dsn from config
func MakeDsn(c *Config) string {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(c.User, c.Password),
		Host:   c.Address,
		Path:   "/" + c.VirtualHost,
	}

	return u.String()
}

/*
func Declare(ch *amqp.Channel, exchange string, queue string, keys []string) error {
	if err := ch.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	); err != nil {
		return err
	}

	q, err := ch.QueueDeclare(
		queue, // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	if err := ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	); err != nil {
		return err
	}

	for _, key := range keys {
		if err := ch.QueueBind(
			q.Name,   // queue name
			key,      // routing key
			exchange, // exchange
			false,
			nil,
		); err != nil {
			return err
		}
	}

	return nil
}
*/
