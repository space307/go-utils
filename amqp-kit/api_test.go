package amqp_kit

import (
	"context"
	"testing"
	"time"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

type apiSuite struct {
	suite.Suite
	dsn  string
	conn *amqp.Connection
}

func (s *apiSuite) SetupSuite() {
	var err error
	s.dsn = MakeDsn(&Config{
		Address:  "127.0.0.1:5672",
		User:     "guest",
		Password: "guest"},
	)

	s.conn, err = amqp.Dial(s.dsn)
	s.Require().NoError(err)
}

func (s *apiSuite) TearDownSuite() {
	s.conn.Close()
}

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}

func (s *apiSuite) TestServe() {
	ch, err := s.conn.Channel()
	s.NoError(err)

	err = Declare(ch, `exc`, `test-a`, []string{`key.request.test`})
	s.NoError(err)

	dec1 := make(chan *amqp.Delivery)
	dec2 := make(chan *amqp.Delivery)

	subs := []SubscribeInfo{
		{
			Q:   `test-a`,
			Key: `key.request.test`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `key.request.test`)
				dec1 <- delivery
				return nil, nil
			},
			Enc: EncodeJSONResponse,
			O:   []SubscriberOption{SubscriberAfter(SetAckAfterEndpoint(true))},
		},
	}

	subs1 := []SubscribeInfo{
		{
			Q:   `test-a`,
			Key: `key.request.test`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Equal(delivery.RoutingKey, `key.request.test`)
				dec2 <- delivery
				return nil, nil
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
	err = pub.Publish("exc", "key.request.test", `cor_1`, []byte(`{"f1":"b1"}`))
	s.NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d.Body, []byte(`{"f1":"b1"}`))
	case <-time.After(5 * time.Second):
	}

	err = ser.Stop()
	s.Require().NoError(err)

	err = pub.Publish("exc", "key.request.test", `cor_2`, []byte(`{"f2":"b2"}`))
	s.NoError(err)

	select {
	case d := <-dec1:
		s.Empty(d)
	case <-time.After(5 * time.Second):
	}

	ser = NewServer(subs1, s.conn)
	err = ser.Serve()
	s.NoError(err)

	err = pub.Publish("exc", "key.request.test", `cor_3`, []byte(`{"f3":"b3"}`))
	s.NoError(err)

	select {
	case d := <-dec2:
		s.Equal(d.Body, []byte(`{"f2":"b2"}`))
	case <-time.After(5 * time.Second):
		s.Fail("timeout. waiting answer for cor_3")
	}

	select {
	case d := <-dec2:
		s.Equal(d.Body, []byte(`{"f3":"b3"}`))
	case <-time.After(5 * time.Second):
		s.Fail("timeout. waiting answer for cor_3")
	}
}
