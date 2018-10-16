package amqp_kit

import (
	"log"
	"net/url"

	"github.com/go-kit/kit/endpoint"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type SubscribeInfo struct {
	Q    string
	Name string
	Key  string
	E    endpoint.Endpoint
	Dec  DecodeRequestFunc
	Enc  EncodeResponseFunc
	O    []SubscriberOption
}

type Config struct {
	Address     string //host:port
	User        string
	Password    string
	VirtualHost string
}

type Server struct {
	Conn *amqp.Connection
	qk   map[string]map[string]func(deliv *amqp.Delivery)
	subs []SubscribeInfo
}

func NewServer(s []SubscribeInfo, con *amqp.Connection) *Server {
	return &Server{
		Conn: con,
		subs: s,
		qk:   make(map[string]map[string]func(deliv *amqp.Delivery)),
	}
}

func (s *Server) Serve() error {
	ch, err := s.Conn.Channel()
	if err != nil {
		logrus.Error(`error create channel: %v`, err)
		return err
	}

	for _, si := range s.subs {
		sbs := NewSubscriber(si.E, si.Dec, si.Enc, si.O...)
		f := sbs.ServeDelivery(ch)
		child, ok := s.qk[si.Q]
		if !ok {
			child = map[string]func(deliv *amqp.Delivery){}
			s.qk[si.Q] = child
		}
		child[si.Key] = f
	}

	for _, sub := range s.subs {
		msgs, err := ch.Consume(sub.Q, sub.Name, false, false, false, false, nil)
		if err != nil {
			return err
		}

		go func(q string) {
			for d := range msgs {
				if _, exist := s.qk[q][d.RoutingKey]; !exist {
					d.Nack(false, false)
					log.Printf(`subscribe key not found %s`, d.RoutingKey)

					continue
				}

				s.qk[q][d.RoutingKey](&d)
			}
		}(sub.Q)
	}

	return nil
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

type PublisherServer struct {
	Conn *amqp.Connection
}
