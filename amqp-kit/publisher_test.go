package amqp_kit

import (
	"context"
	"testing"

	"github.com/opentracing-contrib/go-amqp/amqptracer"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

type pubSuite struct {
	suite.Suite
	dsn string
}

func (s *pubSuite) SetupSuite() {
	s.dsn = MakeDsn(&Config{
		"127.0.0.1:5672",
		"guest",
		"guest",
		"",
	})
}

func (s *pubSuite) TearDownSuite() {}

func TestPubSuite(t *testing.T) {
	suite.Run(t, new(pubSuite))
}

func (s *pubSuite) TestSuccessfulPublisher() {
	conn, err := amqp.Dial(s.dsn)
	s.Require().NoError(err)

	ch, err := conn.Channel()
	s.NoError(err)

	err = Declare(ch, `foo`, `test-p`,
		[]string{`key.request.test`, `key.response.test`, `key.response.unknown`})
	s.NoError(err)

	dec1 := make(chan *amqp.Delivery)
	dec2 := make(chan *amqp.Delivery)

	subs := []SubscribeInfo{
		{
			Q:    `test-p`,
			Name: ``,
			Key:  `key.request.test`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				res := struct {
					Foo string `json:"foo"`
				}{Foo: "bar"}
				return res, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `key.request.test`)
				dec1 <- delivery

				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
				SubscriberBefore(
					SetPublishExchange(`foo`),
					SetPublishKey(`key.response.test`),
				),
			},
		},
		{
			Q:    `test-p`,
			Name: ``,
			Key:  `key.response.test`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `key.response.test`)
				dec2 <- delivery

				return delivery.Body, nil
			},
			Enc: EncodeNopResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
			},
		},
	}

	ser := NewServer(subs, conn)

	err = ser.Serve()
	s.NoError(err)

	ch, err = conn.Channel()
	s.NoError(err)

	pub := NewPublisher(ch)
	err = pub.Publish("foo", "key.request.test", `cor_1`, []byte(`{"f":"b"}`))
	s.NoError(err)

	d := <-dec1
	s.Equal(d.Body, []byte(`{"f":"b"}`))

	d = <-dec2
	s.Equal(d.Body, []byte(`{"foo":"bar"}`))

	err = pub.Publish("foo", "key.response.test", `cor_2`, []byte(`{"f2":"b2"}`))
	s.NoError(err)

	d = <-dec2
	s.Equal(d.Body, []byte(`{"f2":"b2"}`))

	err = pub.Publish("foo", "key.response.unknown", `cor_3`, []byte(`{"f3":"b3"}`))
	s.NoError(err)
}

func (s *pubSuite) TestPublishWithTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	beforeSpan := opentracing.GlobalTracer().StartSpan("to_pub_inject").(*mocktracer.MockSpan)
	beforeCtx := opentracing.ContextWithSpan(context.Background(), beforeSpan)

	conn, err := amqp.Dial(s.dsn)
	s.Require().NoError(err)

	ch, err := conn.Channel()
	s.NoError(err)

	//declare and serve
	err = Declare(ch, `foo`, `test-trace`,
		[]string{`test.key`})
	s.NoError(err)

	dec1 := make(chan *amqp.Delivery)

	subs := []SubscribeInfo{
		{
			Q:    `test-trace`,
			Name: ``,
			Key:  `test.key`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				res := struct {
					Foo string `json:"foo"`
				}{Foo: "bar"}
				return res, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `test.key`)
				dec1 <- delivery

				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
				SubscriberBefore(
					SetPublishExchange(`foo`),
					SetPublishKey(`key.response.test`),
				),
			},
		},
	}

	ser := NewServer(subs, conn)

	err = ser.Serve()
	s.NoError(err)

	//publish
	pub := NewPublisher(ch)
	err = pub.PublishWithTracing(beforeCtx, "foo", "test.key", "ID1", []byte("test-msg"))
	s.NoError(err)

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Require().Len(finishedSpans, 1)

	//extract
	d := <-dec1
	s.Equal(d.Body, []byte(`test-msg`))

	spCtx, _ := amqptracer.Extract(d.Headers)
	sp := opentracing.StartSpan(
		"ConsumeMessage",
		opentracing.FollowsFrom(spCtx),
	)
	sp.Finish()

	// Update the context with the span for the subsequent reference.
	ctx := opentracing.ContextWithSpan(context.Background(), sp)
	s.NotNil(ctx)

	finishedSpan := finishedSpans[0]
	s.Contains(finishedSpan.OperationName, "test.key")

	//finish all
	beforeSpan.Finish()
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Require().Len(finishedSpans, 3)
	//the same trace ID
	s.Equal(finishedSpans[0].SpanContext.TraceID, finishedSpans[1].SpanContext.TraceID)
	s.Equal(finishedSpans[0].SpanContext.TraceID, finishedSpans[2].SpanContext.TraceID)

	// spans:  1(consume)<-0(publish)<-2(root)
	s.Equal(finishedSpans[0].ParentID, finishedSpans[2].SpanContext.SpanID)
	s.Equal(finishedSpans[1].ParentID, finishedSpans[0].SpanContext.SpanID)
}
