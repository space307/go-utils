package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/space307/go-utils/amqp-kit"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
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

func (ts *testSuite) TestPublishWithTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	beforeSpan := opentracing.GlobalTracer().StartSpan("to_pub_inject").(*mocktracer.MockSpan)
	beforeCtx := opentracing.ContextWithSpan(context.Background(), beforeSpan)

	err := ts.client.PublishWithTracing(beforeCtx, "exchange", "test.key", "ID1", []byte("test"))
	ts.NoError(err)

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	ts.Require().Len(finishedSpans, 1)

	finishedSpan := finishedSpans[0]
	ts.Contains(finishedSpan.OperationName, "test.key")

	//finish all
	beforeSpan.Finish()
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	ts.Require().Len(finishedSpans, 2)
	ts.Equal(finishedSpans[0].SpanContext.TraceID, finishedSpans[1].SpanContext.TraceID)
	ts.Equal(finishedSpans[0].ParentID, finishedSpans[1].SpanContext.SpanID)
}

func (ts *testSuite) Test2CustomTracer() {
	obs := &testObserver{}
	tracerRoot, err := CreateTracer("newService", "", config.ContribObserver(obs))
	ts.Require().NoError(err)
	opentracing.SetGlobalTracer(tracerRoot.Tracer)
	defer tracerRoot.Close()

	beforeSpan := opentracing.GlobalTracer().StartSpan("to_pub_inject")
	beforeCtx := opentracing.ContextWithSpan(context.Background(), beforeSpan)

	tracer, err := CreateTracer("newService", "", config.ContribObserver(obs))
	ts.Require().NoError(err)
	sp := tracer.CreateSpan(beforeCtx, "insert")
	sp.Finish()

	beforeSpan.Finish()

	finishedSpans := obs.spans
	ts.Require().Len(finishedSpans, 2)
	second := strings.Split(fmt.Sprintf("%s", finishedSpans[1]), ":")
	first := strings.Split(fmt.Sprintf("%s", finishedSpans[0]), ":")

	ts.Equal(first[0], first[1])

	ts.Equal(first[0], second[0])
	ts.Equal(first[0], second[2])
	ts.NotEqual(first[0], second[1])
}

func (ts *testSuite) SetupSuite() {
	dsn := amqp_kit.MakeDsn(&amqp_kit.Config{
		"127.0.0.1:5672",
		"guest",
		"guest",
		"",
	})
	conn, err := amqp.Dial(dsn)
	ts.Require().NoError(err)

	ch, err := conn.Channel()
	ts.NoError(err)
	pub := amqp_kit.NewPublisher(ch)

	ts.client = &Client{ServiceName: serviceName, Publisher: *pub}
	ts.testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
}

func (ts *testSuite) TearDownSuite() {
	ts.testServer.Close()
}

func TestEventsSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}

//observer from tests
type testObserver struct {
	spans []opentracing.Span
}

type testSpanObserver struct {
	operationName string
	tags          map[string]interface{}
	finished      bool
	span          opentracing.Span
}

func (o *testObserver) OnStartSpan(sp opentracing.Span, operationName string, options opentracing.StartSpanOptions) (jaeger.ContribSpanObserver, bool) {
	tags := make(map[string]interface{})
	for k, v := range options.Tags {
		tags[k] = v
	}

	o.spans = append(o.spans, sp)

	return &testSpanObserver{
		operationName: operationName,
		tags:          tags,
		span:          sp,
	}, true
}

func (o *testSpanObserver) OnSetOperationName(operationName string) {
	o.operationName = operationName
}

func (o *testSpanObserver) OnSetTag(key string, value interface{}) {
	o.tags[key] = value
}

func (o *testSpanObserver) OnFinish(options opentracing.FinishOptions) {
	o.finished = true
}
