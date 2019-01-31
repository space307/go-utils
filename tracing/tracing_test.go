package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

func (ts *testSuite) TestCustomTracer() {
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
