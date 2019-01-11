package amqp_kit

import (
	"context"
	"testing"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

type pubSuite struct {
	suite.Suite
	dsn string
}

func (s *pubSuite) SetupSuite() {
	s.dsn = MakeDsn(&Config{
		rabbitTestAddr,
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
