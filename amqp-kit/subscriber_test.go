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
	dsn  string
	conn *amqp.Connection
}

func (s *subsSuite) SetupSuite() {
	var err error
	s.dsn = MakeDsn(&Config{
		Address:  rabbitTestAddr,
		User:     "guest",
		Password: "guest"},
	)

	s.conn, err = amqp.Dial(s.dsn)
	s.Require().NoError(err)
}

func (s *subsSuite) TearDownSuite() {
	s.conn.Close()
}

func TestSubscriberSuite(t *testing.T) {
	suite.Run(t, new(subsSuite))
}

func (s *subsSuite) TestSubscriberTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	ch, err := s.conn.Channel()
	s.NoError(err)

	err = Declare(ch, `exc`, `test-s`, []string{`key.request.test_2`})
	s.NoError(err)
	ctx := context.Background()
	dec1 := make(chan []byte)
	subs := []SubscribeInfo{
		{
			Q:   `test-s`,
			Key: `key.request.test_2`,
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
				s.Equal(delivery.RoutingKey, `key.request.test_2`)

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

	ser := NewServer(subs, s.conn)
	err = ser.Serve()
	s.NoError(err)

	ch, err = s.conn.Channel()
	s.NoError(err)
	pub := NewPublisher(ch)

	// success
	err = pub.PublishWithTracing(ctx, "exc", "key.request.test_2", `cor_1`, []byte(`{"f1":"b1"}`))
	s.NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d, []byte(`{"f1":"b1"}`))
	case <-time.After(5 * time.Second):
	}

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 2)
	s.Equal(finishedSpans[0].SpanContext.TraceID, finishedSpans[1].SpanContext.TraceID)

	opentracing.GlobalTracer().(*mocktracer.MockTracer).Reset()

	// decode error
	err = pub.PublishWithTracing(ctx, "exc", "key.request.test_2", `errDecodeID`, []byte(`{"f1":"b1"}`))
	s.NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d, []byte(`{"f1":"b1"}`))
	case <-time.After(5 * time.Second):
	}

	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 1)

	// endpoint error
	opentracing.GlobalTracer().(*mocktracer.MockTracer).Reset()

	err = pub.PublishWithTracing(ctx, "exc", "key.request.test_2", `errEndpointID`, []byte(`{"f1":"b1"}`))
	s.NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d, []byte(`endpoint_error`))
	case <-time.After(5 * time.Second):
	}

	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 2)
	s.Equal(finishedSpans[1].Tag(tagError), "endpoint_error_resp")

	err = ser.Stop()
	s.Require().NoError(err)
}
