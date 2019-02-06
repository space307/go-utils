package amqp_kit

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/opentracing/opentracing-go"
	otext "github.com/opentracing/opentracing-go/ext"
	"github.com/streadway/amqp"
)

// Rename parent span if decoder finished with error.
// Also set an error in tag.
func DecodeWithTrace(next DecodeRequestFunc, operationName string) DecodeRequestFunc {
	return func(ctx context.Context, r *amqp.Delivery) (i interface{}, err error) {
		i, err = next(ctx, r)
		if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil && err != nil {
			parentSpan.SetOperationName(operationName)
			parentSpan.SetTag("decode_error", err.Error())
		}

		return i, err
	}
}

// Set operation name for parent span, started in subscriber.go.
// If span not found, start new span with given tracer.
func TraceEndpoint(tracer opentracing.Tracer, operationName string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			parentSpan := opentracing.SpanFromContext(ctx)
			if parentSpan != nil {
				parentSpan.SetOperationName(operationName)
			} else {
				parentSpan = tracer.StartSpan(operationName)
				defer parentSpan.Finish()
			}

			otext.SpanKindRPCServer.Set(parentSpan)
			ctx = opentracing.ContextWithSpan(ctx, parentSpan)

			return next(ctx, request)
		}
	}
}
