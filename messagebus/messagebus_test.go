package messagebus

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/streadway/amqp"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mqChannelMock struct {
	mock.Mock
}

func (m *mqChannelMock) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	res := m.Called(name, kind, durable, autoDelete, internal, noWait, args)
	return res.Error(0)
}

func (m *mqChannelMock) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	res := m.Called(name, durable, autoDelete, exclusive, noWait, args)
	return res.Get(0).(amqp.Queue), res.Error(1)
}

func (m *mqChannelMock) Qos(prefetchCount, prefetchSize int, global bool) error {
	res := m.Called(prefetchCount, prefetchSize, global)
	return res.Error(0)
}

func (m *mqChannelMock) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	res := m.Called(name, key, exchange, noWait, args)
	return res.Error(0)
}

func (m *mqChannelMock) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	res := m.Called(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	return res.Get(0).(<-chan amqp.Delivery), res.Error(1)
}

func (m *mqChannelMock) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	res := m.Called(exchange, key, mandatory, immediate, msg)
	return res.Error(0)
}

func (m *mqChannelMock) Close() error {
	res := m.Called()
	return res.Error(0)
}

type mqTestSuite struct {
	suite.Suite
	mbDsn string
}

func (s *mqTestSuite) SetupSuite() {
	s.mbDsn = MakeDsn(&Config{
		"127.0.0.1:5672",
		"guest",
		"guest",
		"",
	})
}

func (s *mqTestSuite) TearDownSuite() {
}

func TestMqTestSuite(t *testing.T) {
	suite.Run(t, new(mqTestSuite))
}

func (s *mqTestSuite) TestConsumeProduce() {
	pmq, err := Dial(s.mbDsn)
	s.Require().NoError(err)

	defer pmq.Close()

	pmq.SetName("producer")

	rc := make(chan []byte, 1)
	lock := make(chan struct{})

	go func() {
		cmq, err := Dial(s.mbDsn)
		s.Require().NoError(err)

		defer cmq.Close()

		cmq.SetName("consumer")

		handler := func(key string, body []byte) error {
			rc <- body
			return nil
		}

		close(lock)

		if err = cmq.Consume("test-ex", "test-q", []string{"any.*"}, handler); err != nil {
			s.T().Logf("error on consume: %v", err)
		}
	}()

	<-lock

	// We shall sleep here, becase we need to wait, until consumer starts
	time.Sleep(time.Second)

	body := []byte("hello")

	err = pmq.Produce("test-ex", "any.key", body)
	s.Require().NoError(err)

	rv := <-rc

	i := bytes.Compare(rv, body)

	s.Require().Equal(i, 0)
}

func (s *mqTestSuite) TestConsumerRetry() {

	pmq, err := Dial(s.mbDsn)
	s.Require().NoError(err)

	defer pmq.Close()

	pmq.SetName("producer")

	rc := make(chan []byte, 1)
	lock := make(chan struct{})

	myErr := fmt.Errorf("general error")

	var try int

	go func() {
		cmq, err := Dial(s.mbDsn)
		s.Require().NoError(err)

		defer cmq.Close()

		cmq.SetName("consumer")

		handler := func(key string, body []byte) error {
			if try < 2 {
				try++
				return myErr
			}
			rc <- body
			return nil
		}

		close(lock)

		if err = cmq.Consume("test-ex-2", "test-q-2", []string{"any.*"}, handler); err != nil {
			s.T().Logf("error on consume: %v", err)
		}
	}()

	<-lock

	// We shall sleep here, becase we need to wait, until consumer starts
	time.Sleep(time.Second)

	body := []byte("hello retry")

	err = pmq.Produce("test-ex-2", "any.key", body)
	s.Require().NoError(err)

	rv := <-rc

	i := bytes.Compare(rv, body)

	s.Require().Equal(i, 0)
	s.Require().Equal(try, 2)
}

func (s *mqTestSuite) TestConsumerErrors() {

	myErr := fmt.Errorf("fotal eror")

	chmock := &mqChannelMock{}

	chmock.
		On("ExchangeDeclare", "ex", "topic", true, false, false, false, amqp.Table(nil)).
		Return(myErr).
		Once()

	chmock.
		On("ExchangeDeclare", "ex", "topic", true, false, false, false, amqp.Table(nil)).
		Return(nil)

	chmock.
		On("QueueDeclare", "qu", true, false, false, false, amqp.Table(nil)).
		Return(amqp.Queue{}, myErr).
		Once()

	chmock.
		On("QueueDeclare", "qu", true, false, false, false, amqp.Table(nil)).
		Return(amqp.Queue{Name: "qu"}, nil)

	chmock.
		On("Qos", 1, 0, false).
		Return(myErr).
		Once()

	chmock.
		On("Qos", 1, 0, false).
		Return(nil)

	chmock.
		On("QueueBind", "qu", "test.*", "ex", false, amqp.Table(nil)).
		Return(myErr).
		Once()

	chmock.
		On("QueueBind", "qu", "test.*", "ex", false, amqp.Table(nil)).
		Return(nil)

	chmock.
		On("Consume", "qu", "", false, false, false, false, amqp.Table(nil)).
		Return(make(<-chan amqp.Delivery), myErr).
		Once()

	mq := &MessageBus{
		ch: chmock,
	}

	for i := 0; i < 5; i++ {
		err := mq.Consume("ex", "qu", []string{"test.*"}, nil)
		s.EqualError(err, myErr.Error())
	}
}

func (s *mqTestSuite) TestProducerErrors() {

	myErr := fmt.Errorf("fotal eror")

	chmock := &mqChannelMock{}

	chmock.
		On("ExchangeDeclare", "ex", "topic", true, false, false, false, amqp.Table(nil)).
		Return(myErr).
		Once()

	chmock.
		On("ExchangeDeclare", "ex", "topic", true, false, false, false, amqp.Table(nil)).
		Return(nil)

	chmock.
		On("Publish", "ex", "test.me", true, false, mock.AnythingOfType("amqp.Publishing")).
		Return(myErr).
		Once()

	mq := &MessageBus{
		ch:        chmock,
		exchanges: make(map[string]struct{}),
	}

	for i := 0; i < 2; i++ {
		err := mq.Produce("ex", "test.me", []byte{})
		s.EqualError(err, myErr.Error())
	}
}
