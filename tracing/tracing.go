package tracing

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

type Tracer struct {
	closer io.Closer
	Tracer opentracing.Tracer
}

// create custom Tracer
func CreateTracer(serviceName, agentAddress string, opts ...jaegercfg.Option) (*Tracer, error) {
	jcfg := jaegercfg.Configuration{
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  agentAddress,
		},
	}

	jcfg.ServiceName = serviceName
	tracer, closer, err := jcfg.NewTracer(opts...)
	if err != nil {
		log.Printf("Could not initialize custom jaeger Tracer: %s", err.Error())
		return nil, err
	}

	return &Tracer{
		closer: closer,
		Tracer: tracer,
	}, nil
}

func (t *Tracer) CreateSpan(ctx context.Context, operationName string) opentracing.Span {
	var opts opentracing.SpanReference
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		opts = opentracing.ChildOf(parentSpan.Context())
	}

	span := t.Tracer.StartSpan(operationName, opts)

	return span
}

func (t *Tracer) Close() error {
	return t.closer.Close()
}
