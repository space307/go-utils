package amqp_kit

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/opentracing/opentracing-go"
	otext "github.com/opentracing/opentracing-go/ext"
)

// Extract
func TraceEndpoint(tracer opentracing.Tracer, operationName string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			spCtx := ctx.Value(ampqCtx).(*opentracing.SpanContext)
			sp := tracer.StartSpan(operationName, opentracing.FollowsFrom(*spCtx))
			defer sp.Finish()

			otext.SpanKindRPCServer.Set(sp)
			ctx = opentracing.ContextWithSpan(ctx, sp)
			i, err := next(ctx, request)
			if err != nil {
				sp.SetTag("error", err.Error())
			}

			return i, err
		}
	}
}
