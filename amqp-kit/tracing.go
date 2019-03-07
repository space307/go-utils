package amqp_kit

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/opentracing-contrib/go-amqp/amqptracer"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

const tagError = "error"

type amqpSpanCtx string

var amqpCtx amqpSpanCtx

//publish message to AMQP with tracing and span context
func (c *Client) PublishWithTracing(ctx context.Context, exchange, key, corID string, body []byte) (err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, `publish_key: `+key)
	defer span.Finish()

	ext.SpanKind.Set(span, ext.SpanKindProducerEnum)
	span.SetTag("key", key)
	span.SetTag("exchange", exchange)
	span.SetTag("corID", corID)

	pub := amqp.Publishing{
		Headers:       amqp.Table{},
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}

	// Inject the span context into the AMQP header.
	if err := amqptracer.Inject(span, pub.Headers); err != nil {
		log.Printf("publish: error inject headers: %s", err)
	}

	return c.send(exchange, key, &pub)
}

// Get context value spanContext and start Span with given operationName.
// Set an error as tag if raised.
func TraceEndpoint(tracer opentracing.Tracer, operationName string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			var sp opentracing.Span
			if spCtx, ok := ctx.Value(amqpCtx).(*opentracing.SpanContext); ok {
				sp = tracer.StartSpan(operationName, opentracing.FollowsFrom(*spCtx))
				defer sp.Finish()

				ext.SpanKindRPCServer.Set(sp)
				ctx = opentracing.ContextWithSpan(ctx, sp)
			}

			i, err := next(ctx, request)
			if err != nil && sp != nil {
				sp.SetTag(tagError, err.Error())
			}

			return i, err
		}
	}
}
