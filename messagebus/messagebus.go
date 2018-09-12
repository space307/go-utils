package messagebus

import (
	"github.com/streadway/amqp"
)

const (
	attemptsHeader = "x-process-attempts"
	maxAttempts    = 3
)

type mqChannel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	Qos(prefetchCount, prefetchSize int, global bool) error
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

// MessageBus
type MessageBus struct {
	conn      *amqp.Connection
	ch        mqChannel
	exchanges map[string]struct{}
	appName   string
}

// Dial initialize connection to amqp
func Dial(mqDsn string) (*MessageBus, error) {
	conn, err := amqp.Dial(mqDsn)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	mb := &MessageBus{
		conn:      conn,
		ch:        ch,
		exchanges: make(map[string]struct{}),
	}

	return mb, nil
}

// SetName - sets current connection name
func (mb *MessageBus) SetName(name string) {
	mb.appName = name
}

// Produce - sends message to given `exchange` with given `key`
func (mb *MessageBus) Produce(exchange, key string, body []byte) (err error) {

	if _, ok := mb.exchanges[exchange]; !ok {
		if err = mb.ch.ExchangeDeclare(
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
		mb.exchanges[exchange] = struct{}{}
	}

	err = mb.ch.Publish(
		exchange,
		key,
		true, // mandatory
		false,
		amqp.Publishing{
			AppId:        mb.appName,
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		},
	)
	return
}

func tryReenqueue(d *amqp.Delivery) {
	var attempt int32

	val, ok := d.Headers[attemptsHeader]
	if ok {
		attempt = val.(int32)
	}

	attempt++

	maxReached := attempt > maxAttempts

	if d.Headers == nil {
		d.Headers = make(amqp.Table)
	}

	d.Headers[attemptsHeader] = attempt
	d.Reject(!maxReached)
}

// Consume start consuming given `exchange` with given `queue` (binded with given `keys`)
func (mb *MessageBus) Consume(exchange, queue string, keys []string, handler func(key string, body []byte) error) error {

	if err := mb.ch.ExchangeDeclare(
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

	q, err := mb.ch.QueueDeclare(
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

	if err := mb.ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	); err != nil {
		return err
	}

	for _, key := range keys {
		if err := mb.ch.QueueBind(
			q.Name,   // queue name
			key,      // routing key
			exchange, // exchange
			false,
			nil,
		); err != nil {
			return err
		}
	}

	msgs, err := mb.ch.Consume(
		q.Name,     // queue
		mb.appName, // consumer
		false,      // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return err
	}

	for d := range msgs {
		switch err = handler(d.RoutingKey, d.Body); err {
		case nil:
			d.Ack(false)
		default:
			tryReenqueue(&d)
		}
	}

	return nil
}

func (mb *MessageBus) Close() error {
	mb.ch.Close()
	return mb.conn.Close()
}
