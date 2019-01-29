package amqp_kit

import (
	"net/url"

	"github.com/go-kit/kit/endpoint"
	"github.com/streadway/amqp"
)

type SubscribeInfo struct {
	Q       string
	Name    string
	Key     string
	Workers int
	E       endpoint.Endpoint
	Dec     DecodeRequestFunc
	Enc     EncodeResponseFunc
	O       []SubscriberOption
}

type Config struct {
	Address     string //host:port
	User        string
	Password    string
	VirtualHost string
}

type subscriber struct {
	k string
	h func(deliv *amqp.Delivery)
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

	subscribers := make(map[string][]*subscriber)
	workersnum := make(map[string]int)

	for _, si := range s.subs {
		subs, ok := subscribers[si.Q]
		if !ok {
			subs = []*subscriber{}
		}
		workers := 1
		if si.Workers > 0 {
			workers = si.Workers
		}
		workersnum[si.Q] = workers
		subscribers[si.Q] = append(subs, &subscriber{
			k: si.Key,
			h: NewSubscriber(si.E, si.Dec, si.Enc, si.O...).ServeDelivery(s.ch),
		})
	}

	for queue, subs := range subscribers {
		msgs, err := s.ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			return err
		}

		num := workersnum[queue]

		for i := 0; i < num; i++ {
			go func(ch <-chan amqp.Delivery, sbs []*subscriber) {
				var (
					d         amqp.Delivery
					ok, found bool
				)

				for {
					select {
					case d, ok = <-ch:
					}
					if !ok {
						return
					}
					found = false

					for _, sub := range sbs {
						if found = d.RoutingKey == sub.k; found {
							sub.h(&d)
							break
						}
					}

					if !found {
						d.Nack(false, false)
					}
				}
			}(msgs, subs)
		}
	}

	return
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
