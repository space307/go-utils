package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/suite"
)

const serviceName = "test"

type testSuite struct {
	suite.Suite
	testServer *httptest.Server
	client     *Client
}

func (ts *testSuite) TestDoWithTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	beforeSpan := opentracing.GlobalTracer().StartSpan("to_inject").(*mocktracer.MockSpan)
	defer beforeSpan.Finish()

	req, err := http.NewRequest("", ts.testServer.URL, nil)
	ts.NoError(err)

	beforeCtx := opentracing.ContextWithSpan(context.Background(), beforeSpan)
	_, err = ts.client.DoWithTracing(beforeCtx, req)
	ts.NoError(err)

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	ts.Len(finishedSpans, 1)

	finishedSpan := finishedSpans[0]
	ts.Equal(serviceName, finishedSpan.Tag(string(ext.PeerService)))

	beforeContext := beforeSpan.Context().(mocktracer.MockSpanContext)
	ts.Equal(beforeContext.SpanID, finishedSpan.ParentID)

	ts.testServer.Close()
	_, err = ts.client.DoWithTracing(beforeCtx, req)
	ts.Error(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	ts.Len(finishedSpans, 2)
	ts.Len(finishedSpans[1].Logs(), 1)
}

func (ts *testSuite) SetupSuite() {
	ts.client = &Client{ServiceName: serviceName}
	ts.testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
}

func (ts *testSuite) TearDownSuite() {
	ts.testServer.Close()
}

func TestTracingSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}
