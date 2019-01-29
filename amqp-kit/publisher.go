package amqp_kit

import (
	"context"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	logop "github.com/opentracing/opentracing-go/log"
	"github.com/streadway/amqp"
)

type Publisher struct {
	ch Channel
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

//publish message to AMQP with tracing
func (c *Publisher) PublishWithTracing(ctx context.Context, exchange, key, corID string, body []byte) (err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, `publish_key: `+key)
	defer span.Finish()

	ext.SpanKind.Set(span, ext.SpanKindProducerEnum)

	var headers = make(http.Header)
	headers.Add("exchange", exchange)
	headers.Add("key", key)
	headers.Add("corID", corID)
	span.Tracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(headers),
	)

	err = c.Publish(exchange, key, corID, body)
	if err != nil {
		span.LogFields(logop.Error(err))
		ext.Error.Set(span, true)

		return err
	}

	return nil
}
