package tracing

import (
	"context"
	"net/http"

	"github.com/space307/go-utils/amqp-kit"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	logop "github.com/opentracing/opentracing-go/log"
)

type Client struct {
	amqp_kit.Publisher
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
		return resp, err
	}

	ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))

	return resp, nil
}
