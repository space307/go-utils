package amqp_kit

import (
	"context"
	"net/http"
	"testing"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestNewError(t *testing.T) {
	e := NewError(`test message`, `test_message`, http.StatusBadRequest)
	assert.Equal(t, e.Code, `test_message`)
	assert.Equal(t, e.StatusCode, http.StatusBadRequest)
	assert.Equal(t, e.Message, `test message`)
}

func TestWrapErrorWithCode(t *testing.T) {
	e := NewError(`test message`, `test_message`, http.StatusInternalServerError)
	e1 := WrapError(e, `test message 1`)

	assert.Equal(t, e1.Message, `test message 1`)
	assert.Equal(t, e.Message, `test message`)
	assert.Equal(t, e1.StatusCode, http.StatusInternalServerError)
	assert.Equal(t, e1.Code, `test_message`)
}

func TestErrorWithCode_Error(t *testing.T) {
	e := NewError(`test message`, `test_message`, http.StatusBadRequest)
	assert.Equal(t, `test message`, e.Error())
}

type errSuite struct {
	suite.Suite
	dsn string
}

func (s *errSuite) SetupSuite() {
	s.dsn = MakeDsn(&Config{
		"127.0.0.1:5672",
		"guest",
		"guest",
		"",
	})
}

func (s *errSuite) TearDownSuite() {}

func TestErrSuite(t *testing.T) {
	suite.Run(t, new(errSuite))
}

func (s *errSuite) TestErrResponse() {
	conn, err := amqp.Dial(s.dsn)
	s.NoError(err)

	ch, err := conn.Channel()
	s.Require().NoError(err)

	err = Declare(ch, `test`, `test`,
		[]string{`key.request.test`, `key.request-err.test`, `key.response.test`})
	s.Require().NoError(err)

	dec1 := make(chan *amqp.Delivery)
	dec2 := make(chan *amqp.Delivery)

	subs := []SubscribeInfo{
		{
			Q:    `test`,
			Name: ``,
			Key:  `key.request.test`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				res := Response{
					Data: struct {
						Foo string `json:"foo"`
					}{Foo: "bar"},
				}
				return res, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, `key.request.test`)
				dec1 <- delivery

				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
				SubscriberBefore(
					SetPublishExchange(`test`),
					SetPublishKey(`key.response.test`),
				),
			},
		},
		{
			Q:    `test`,
			Name: ``,
			Key:  `key.request-err.test`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, NewError(`err-message`, `err_message`, http.StatusBadRequest)
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, `key.request-err.test`)
				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
				SubscriberBefore(
					SetPublishExchange(`test`),
					SetPublishKey(`key.response.test`),
				),
			},
		},
		{
			Q:    `test`,
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
	s.Require().NoError(err)

	pub := NewPublisher(ch)
	err = pub.Publish("test", "key.request.test", `cor_1`, []byte(`{"f":"b"}`))
	s.NoError(err)

	d := <-dec1
	s.Equal(d.Body, []byte(`{"f":"b"}`))

	d = <-dec2
	s.Equal(d.Body, []byte(`{"data":{"foo":"bar"}}`))

	err = pub.Publish("test", "key.request-err.test", `cor_2`, []byte(`{"f":"b1"}`))
	s.NoError(err)

	d = <-dec2
	s.EqualValues(d.Body, []byte(`{"error":{"code":"err_message","message":"err-message","status_code":400}}`))
}
