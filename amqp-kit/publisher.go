package amqp_kit

import (
	"context"

	"github.com/opentracing-contrib/go-amqp/amqptracer"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/streadway/amqp"
)

type Publisher struct {
	ch          Channel
	ServiceName string
}

// NewPublisher constructs a usable Publisher for a single remote method.
func NewPublisher(ch Channel) *Publisher {
	return &Publisher{ch: ch}
}

func (p Publisher) Publish(exchange, key, corID string, body []byte) (err error) {
	pub := amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}

	err = p.ch.Publish(exchange, key, false, false, pub)
	if err != nil {
		return err
	}

	return err
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
		return err
	}

	return p.ch.Publish(exchange, key, false, false, msg)
}
