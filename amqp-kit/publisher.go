package amqp_kit

import (
	"context"
	"log"

	"github.com/opentracing-contrib/go-amqp/amqptracer"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/streadway/amqp"
)

type Publisher struct {
	ch          Channel
	conn        *amqp.Connection
	ServiceName string
}

// NewPublisher constructs a usable Publisher for a single remote method.
func NewPublisher(conn *amqp.Connection) (*Publisher, error) {

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	pub := &Publisher{
		ch:   ch,
		conn: conn,
	}

	return pub, nil
}

func (p *Publisher) send(exchange, key string, pub *amqp.Publishing) (err error) {

	var ch *amqp.Channel

	for try := 0; try < 3; try++ {
		if err = p.ch.Publish(exchange, key, false, false, *pub); err == nil {
			return
		}
		aerr, ok := err.(amqp.Error)
		if !ok {
			return
		}
		if aerr.Code != amqp.ChannelError {
			return
		}
		if ch, err = p.conn.Channel(); err != nil {
			return
		}
		p.ch = ch
	}
	return
}

func (p *Publisher) Publish(exchange, key, corID string, body []byte) (err error) {
	msg := amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}
	return p.send(exchange, key, &msg)
}

//publish message to AMQP with tracing and span context
func (p *Publisher) PublishWithTracing(ctx context.Context, exchange, key, corID string, body []byte) (err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, `publish_key: `+key)
	defer span.Finish()

	ext.SpanKind.Set(span, ext.SpanKindProducerEnum)
	span.SetTag("key", key)
	span.SetTag("exchange", exchange)
	span.SetTag("corID", corID)

	msg := amqp.Publishing{
		Headers:       amqp.Table{},
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}

	// Inject the span context into the AMQP header.
	if err := amqptracer.Inject(span, msg.Headers); err != nil {
		log.Printf("publish: error inject headers: %s", err)
	}

	return p.send(exchange, key, &msg)
}
