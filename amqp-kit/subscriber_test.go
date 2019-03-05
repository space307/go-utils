package amqp_kit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

type subsSuite struct {
	suite.Suite
	config *Config
}

func (s *subsSuite) SetupSuite() {
	s.config = &Config{Address: rabbitTestAddr, User: "guest", Password: "guest"}
}

func (s *subsSuite) TearDownSuite() {}

func TestSubscriberSuite(t *testing.T) {
	suite.Run(t, new(subsSuite))
}

func (s *subsSuite) TestSubscriberTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	ctx := context.Background()
	dec1 := make(chan []byte)
	subs := []SubscribeInfo{
		{
			Queue:       `test`,
			SubExchange: `subscriber-test`,
			E: endpoint.Chain(
				TraceEndpoint(tracer, `test_endpoint`),
			)(func(ctx context.Context, request interface{}) (response interface{}, err error) {
				dec1 <- request.([]byte)

				if string(request.([]byte)) == "endpoint_error" {
					return nil, fmt.Errorf("endpoint_error_resp")
				}

				return nil, nil
			}),
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `test`)

				if delivery.CorrelationId == "errDecodeID" {
					return nil, fmt.Errorf("decode error")
				}

				if delivery.CorrelationId == "errEndpointID" {
					return []byte("endpoint_error"), nil
				}
				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O:   []SubscriberOption{SubscriberAfter(SetAckAfterEndpoint(true))},
		},
	}

	cl, err := New(s.config)
	err = cl.Serve(subs)
	s.Require().NoError(err)
	time.Sleep(5 * time.Second)

	// success
	err = cl.PublishWithTracing(ctx, "subscriber-test", "test", `cor_1`, []byte(`{"f1":"b1"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d, []byte(`{"f1":"b1"}`))
	case <-time.After(5 * time.Second):
	}

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Require().Len(finishedSpans, 2)
	s.Equal(finishedSpans[0].SpanContext.TraceID, finishedSpans[1].SpanContext.TraceID)

	opentracing.GlobalTracer().(*mocktracer.MockTracer).Reset()

	// decode error
	err = cl.PublishWithTracing(ctx, "subscriber-test", "test", `errDecodeID`, []byte(`{"f1":"b1"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d, []byte(`{"f1":"b1"}`))
	case <-time.After(5 * time.Second):
	}

	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 1)

	// endpoint error
	opentracing.GlobalTracer().(*mocktracer.MockTracer).Reset()

	err = cl.PublishWithTracing(ctx, "subscriber-test", "test", `errEndpointID`, []byte(`{"f1":"b1"}`))
	s.NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d, []byte(`endpoint_error`))
	case <-time.After(5 * time.Second):
	}

	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 2)
	s.Equal(finishedSpans[1].Tag(tagError), "endpoint_error_resp")

	err = cl.Close()
	s.Require().NoError(err)
}
