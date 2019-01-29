package tracing

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/space307/go-utils/amqp-kit"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	logop "github.com/opentracing/opentracing-go/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

type Client struct {
	amqp_kit.Publisher
	http.Client
	ServiceName string
}

type Tracer struct {
	closer io.Closer
	Tracer opentracing.Tracer
}

//execute http-request with tracing
func (c *Client) DoWithTracing(ctx context.Context, req *http.Request) (resp *http.Response, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, `request:`+req.URL.Path)
	defer span.Finish()

	ext.PeerService.Set(span, c.ServiceName)
	ext.SpanKind.Set(span, ext.SpanKindRPCServerEnum)

	span.Tracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)

	resp, err = c.Do(req)
	if err != nil {
		span.LogFields(logop.Error(err))
		return resp, err
	}

	ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))

	return resp, nil
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
