package tracing

import (
	"context"
	"log"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	logop "github.com/opentracing/opentracing-go/log"
)

type Client struct {
	http.Client
	ServiceName string
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
		log.Printf("http: request error: %s", err)
	}

	if resp != nil {
		ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))
	}

	return resp, err
}
