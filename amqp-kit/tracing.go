package amqp_kit

import (
	"context"

	"github.com/go-kit/kit/endpoint"
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

// set operation name for parent span, started in subscriber.go
// if span not found, start new span
func TraceEndpoint(tracer opentracing.Tracer, operationName string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			var opts opentracing.SpanReference
			parentSpan := opentracing.SpanFromContext(ctx)
			if parentSpan != nil {
				parentSpan.SetOperationName(operationName)
			} else {
				parentSpan = tracer.StartSpan(operationName, opts)
			}

			defer parentSpan.Finish()

			otext.SpanKindRPCServer.Set(parentSpan)
			ctx = opentracing.ContextWithSpan(ctx, parentSpan)

			return next(ctx, request)
		}
	}
}
