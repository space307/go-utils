package amqp_kit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/suite"
)

const (
	rabbitTestAddr = "127.0.0.1:5672"
)

type apiSuite struct {
	suite.Suite
	config *Config
}

func (s *apiSuite) SetupSuite() {
	s.config = &Config{Address: rabbitTestAddr, User: "guest", Password: "guest"}
}

func (s *apiSuite) TearDownSuite() {

}

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}

func (s *apiSuite) TestServe() {
	dec1 := make(chan *amqp.Delivery)
	dec2 := make(chan *amqp.Delivery)

	subs := []SubscribeInfo{
		{
			Queue:    `test_a`,
			Exchange: `exc`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, "test.a")
				dec1 <- delivery
				return nil, nil
			},
			Enc: EncodeJSONResponse,
			O:   []SubscriberOption{SubscriberAfter(SetAckAfterEndpoint(true))},
		},
	}

	subs1 := []SubscribeInfo{
		{
			Queue:    `test_b`,
			Exchange: `exc`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, "test.b")
				dec2 <- delivery
				return nil, nil
			},
			Enc: EncodeJSONResponse,
			O:   []SubscriberOption{SubscriberAfter(SetAckAfterEndpoint(true))},
		},
	}

	cl, err := New(s.config)
	s.Require().NoError(err)

	err = cl.Serve(subs)
	s.Require().NoError(err)

	err = cl.Publish("exc", "test.a 1", `cor_1`, []byte(`{"f1":"b1"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d.Body, []byte(`{"f1":"b1"}`))
	case <-time.After(5 * time.Second):
	}

	err = cl.Publish("exc", "test.a", `cor_2`, []byte(`{"f2":"b2"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec1:
		s.Equal(d.Body, []byte(`{"f2":"b2"}`))
	case <-time.After(5 * time.Second):
	}

	err = cl.Close()
	s.Require().NoError(err)

	// ---
	cl, err = New(&Config{Address: rabbitTestAddr, User: "guest", Password: "guest"})
	s.Require().NoError(err)

	err = cl.Serve(subs1)
	s.Require().NoError(err)

	err = cl.Publish("exc", "test.b", `cor_3`, []byte(`{"f3":"b3"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec2:
		s.Equal(d.Body, []byte(`{"f3":"b3"}`))
	case <-time.After(5 * time.Second):
		s.Fail("timeout. waiting answer for cor_3")
	}

	// close connection
	err = cl.conn.amqpConn.Close()
	s.Require().NoError(err)

	time.Sleep(5 * time.Second)

	err = cl.Publish("exc", "test.b", `cor_3`, []byte(`{"f3":"b3"}`))
	s.Require().NoError(err)

	select {
	case d := <-dec2:
		s.Equal(d.Body, []byte(`{"f3":"b3"}`))
	case <-time.After(10 * time.Second):
		s.Fail("timeout. waiting answer for cor_3")
	}

	err = cl.Close()
	s.Require().NoError(err)
}

func (s *apiSuite) TestDuplicateQueue() {
	subs := []SubscribeInfo{
		{
			Queue:    `dup`,
			Exchange: `exc`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				return nil, nil
			},
			Enc: EncodeJSONResponse,
		},
		{
			Queue:    `dup`,
			Exchange: `exc`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				return nil, nil
			},
			Enc: EncodeJSONResponse,
		},
	}

	cl, err := New(s.config)
	s.Require().NoError(err)

	err = cl.Serve(subs)
	s.EqualError(err, fmt.Errorf("amqp_kit: duplicate queue entry: '%s' ", `dup`).Error())
}

func (s *apiSuite) TestServeMany() {
	dec1 := make(chan *amqp.Delivery, 2)
	dec2 := make(chan *amqp.Delivery, 1)

	sleepTime := time.Millisecond * 500
	checkTime := time.Millisecond * 600

	subs := []SubscribeInfo{
		{
			Workers:  2,
			Queue:    `many_a`,
			Exchange: `exc-many`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, `many.a`)
				time.Sleep(sleepTime)
				dec1 <- delivery
				return nil, nil
			},
			Enc: EncodeJSONResponse,
			O:   []SubscriberOption{SubscriberAfter(SetAckAfterEndpoint(true))},
		},
		{
			Queue:    `many_b`,
			Exchange: `exc-many`,
			E: func(ctx context.Context, request interface{}) (response interface{}, err error) {
				return nil, nil
			},
			Dec: func(i context.Context, delivery *amqp.Delivery) (request interface{}, err error) {
				s.Require().Equal(delivery.RoutingKey, `many.b`)
				time.Sleep(sleepTime)
				dec2 <- delivery
				return nil, nil
			},
			Enc: EncodeJSONResponse,
			O:   []SubscriberOption{SubscriberAfter(SetAckAfterEndpoint(true))},
		},
	}

	cl, err := New(&Config{Address: rabbitTestAddr, User: "guest", Password: "guest"})
	s.Require().NoError(err)

	err = cl.Serve(subs)
	s.Require().NoError(err)

	// error
	err = cl.Publish("not_exists_ex", "not.exists.key", "", []byte(`{"fa1":"b1"}`))

	err = cl.Publish("exc-many", "many.a", "", []byte(`{"fa1":"b1"}`))
	s.NoError(err)

	err = cl.Publish("exc-many", "many.b", "", []byte(`{"fb1":"b1"}`))
	s.NoError(err)

	err = cl.Publish("exc-many", "many.a", "", []byte(`{"fa1":"b2"}`))
	s.NoError(err)

	var ca, cb int

	t := time.After(checkTime)
	var res bool

	for {
		select {
		case <-dec1:
			ca++
		case <-dec2:
			cb++
		case <-t:
			s.Require().Equal(2, ca)
			s.Require().Equal(1, cb)
			res = true
		}

		if res {
			break
		}
	}

	err = cl.Close()
	s.Require().NoError(err)
}
