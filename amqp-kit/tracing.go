package amqp_kit

import (
	"context"

	"github.com/opentracing-contrib/go-amqp/amqptracer"
	"github.com/opentracing/opentracing-go"
	otext "github.com/opentracing/opentracing-go/ext"
	"github.com/streadway/amqp"
)

func DecodeWithTrace(next DecodeRequestFunc, operationName string) DecodeRequestFunc {
	return func(ctx context.Context, r *amqp.Delivery) (i interface{}, err error) {
		//extract tracing headers
		spCtx, _ := amqptracer.Extract(r.Headers)
		sp := opentracing.StartSpan(
			operationName,
			opentracing.FollowsFrom(spCtx),
		)
		defer sp.Finish()

		// Update the context with the span for the subsequent reference.
		otext.SpanKindRPCServer.Set(sp)
		ctx = opentracing.ContextWithSpan(ctx, sp)

		return next(ctx, r)
	}
}
