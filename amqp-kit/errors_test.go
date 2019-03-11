package amqp_kit

import (
	"context"
	"net/http"
	"testing"
	"time"

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
	config *Config
}

func (s *errSuite) SetupSuite() {
	s.config = &Config{Address: rabbitTestAddr, User: "guest", Password: "guest"}
}

func (s *errSuite) TearDownSuite() {}

func TestErrSuite(t *testing.T) {
	suite.Run(t, new(errSuite))
}

func (s *errSuite) TestErrResponse() {
	dec1 := make(chan *amqp.Delivery)
	dec2 := make(chan *amqp.Delivery)

	subs := []SubscribeInfo{
		{
			Queue:    `request`,
			Exchange: `error`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				res := Response{
					Data: struct {
						Foo string `json:"foo"`
					}{Foo: "bar"},
				}
				return res, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, `request`)
				dec1 <- delivery

				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
				SubscriberBefore(
					SetPublishExchange(`error`),
					SetPublishKey(`response`),
				),
			},
		},
		{
			Queue:    `request_err`,
			Exchange: `error`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, NewError(`err-message`, `err_message`, http.StatusBadRequest)
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, `request.err`)
				return delivery.Body, nil
			},
			Enc: EncodeJSONResponse,
			O: []SubscriberOption{
				SubscriberAfter(
					SetAckAfterEndpoint(false),
				),
				SubscriberBefore(
					SetPublishExchange(`error`),
					SetPublishKey(`response`),
				),
			},
		},
		{
			Queue:    `response`,
			Exchange: `error`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `response`)
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

	cl, err := New(s.config)
	s.Require().NoError(err)

	err = cl.Serve(subs)
	s.Require().NoError(err)

	err = cl.Publish("error", "request", `cor_1`, []byte(`{"f":"b"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d.Body, []byte(`{"f":"b"}`))
	case <-time.After(5 * time.Second):
		s.Fail("timeout. waiting answer on dec1")
	}

	select {
	case d := <-dec2:
		s.Equal(d.Body, []byte(`{"data":{"foo":"bar"}}`))
	case <-time.After(5 * time.Second):
		s.Fail("timeout. waiting answer on dec2")
	}

	err = cl.Publish("error", "request.err", `cor_2`, []byte(`{"f":"b1"}`))
	s.NoError(err)

	select {
	case d := <-dec2:
		s.EqualValues(d.Body, []byte(`{"error":{"code":"err_message","message":"err-message","status_code":400}}`))
	case <-time.After(5 * time.Second):
		s.Fail("timeout. waiting answer on dec2")
	}

	s.Require().NoError(cl.Close())
}
