package amqp_kit

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/space307/go-utils/errors"
	"github.com/streadway/amqp"
)

// Channel is a channel interface to make testing possible
// It is highly recommended to use *amqp.Channel as the interface implementation
type Channel interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWail bool, args amqp.Table) (<-chan amqp.Delivery, error)
}

// Subscriber wraps an endpoint and provides a handler for AMQP Delivery messages
type Subscriber struct {
	e            endpoint.Endpoint
	dec          DecodeRequestFunc
	enc          EncodeResponseFunc
	before       []RequestFunc
	after        []SubscriberResponseFunc
	errorEncoder ErrorEncoder
}

// DecodeRequestFunc extracts a user-domain request object from
// an AMQP Delivery object. It is designed to be used in AMQP Subscribers.
type DecodeRequestFunc func(context.Context, *amqp.Delivery) (request interface{}, err error)

// EncodeResponseFunc encodes the passed reponse object to
// an AMQP channel for publishing. It is designed to be used in AMQP Subscribers
type EncodeResponseFunc func(context.Context,
	*amqp.Delivery, Channel, *amqp.Publishing, interface{}) error

// NewSubscriber constructs a new subscriber, which provides a handler
// for AMQP Delivery messages
func NewSubscriber(e endpoint.Endpoint, dec DecodeRequestFunc, enc EncodeResponseFunc, options ...SubscriberOption) *Subscriber {
	s := &Subscriber{
		e:            e,
		dec:          dec,
		enc:          enc,
		errorEncoder: ReplyAndAckErrorWithCodeEncoder,
	}
	for _, option := range options {
		option(s)
	}
	return s
}

// SubscriberOption sets an optional parameter for subscribers.
type SubscriberOption func(*Subscriber)

// SubscriberBefore functions are executed on the publisher delivery object
// before the request is decoded
func SubscriberBefore(before ...RequestFunc) SubscriberOption {
	return func(s *Subscriber) { s.before = append(s.before, before...) }
}

// SubscriberAfter functions are executed on the subscriber reply after the
// endpoint is invoked, but before anything is published to the reply.
func SubscriberAfter(after ...SubscriberResponseFunc) SubscriberOption {
	return func(s *Subscriber) { s.after = append(s.after, after...) }
}

// SubscriberErrorEncoder is used to encode errors to the subscriber reply
// whenever they're encountered in the processing of a request. Clients can
// use this to provide custom error formatting. By default,
// errors will be published with the DefaultErrorEncoder.
func SubscriberErrorEncoder(ee ErrorEncoder) SubscriberOption {
	return func(s *Subscriber) { s.errorEncoder = ee }
}

// ServeDelivery handles AMQP Delivery messages
// It is strongly recommended to use *amqp.Channel as the
// Channel interface implementation
func (s Subscriber) ServeDelivery(ch Channel) func(deliv *amqp.Delivery) {
	return func(deliv *amqp.Delivery) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pub := amqp.Publishing{
			ContentType: "application/json",
		}

		for _, f := range s.before {
			ctx = f(ctx, &pub)
		}

		request, err := s.dec(ctx, deliv)

		if err != nil {
			s.errorEncoder(ctx, err, deliv, ch, &pub)
			return
		}

		response, err := s.e(ctx, request)
		if err != nil {
			s.errorEncoder(ctx, err, deliv, ch, &pub)
			return
		}

		for _, f := range s.after {
			ctx = f(ctx, deliv, ch, &pub)
		}

		if err := s.enc(ctx, deliv, ch, &pub, response); err != nil {
			s.errorEncoder(ctx, err, deliv, ch, &pub)
			return
		}
	}

}

// EncodeJSONResponse marshals the response as JSON and sends the response
// to the given channel as a reply.
func EncodeJSONResponse(ctx context.Context, deliv *amqp.Delivery, ch Channel, pub *amqp.Publishing, response interface{}) error {
	if pub.CorrelationId == "" {
		pub.CorrelationId = deliv.CorrelationId
	}

	replyExchange := getPublishExchange(ctx)
	replyTo := getPublishKey(ctx)
	if replyTo == "" {
		replyTo = deliv.ReplyTo
	}

	b, err := json.Marshal(response)
	if err != nil {
		return err
	}
	pub.Body = b

	err = ch.Publish(replyExchange, replyTo, false, false, *pub)

	return err
}

// EncodeNopResponse is a response function that does nothing.
func EncodeNopResponse(_ context.Context, _ *amqp.Delivery, _ Channel, _ *amqp.Publishing, _ interface{}) error {
	return nil
}

// ErrorEncoder is responsible for encoding an error to the subscriber reply.
// Users are encouraged to use custom ErrorEncoders to encode errors to
// their replies, and will likely want to pass and check for their own error
// types.
type ErrorEncoder func(ctx context.Context, err error, deliv *amqp.Delivery, ch Channel, pub *amqp.Publishing)

// SingleNackRequeueErrorEncoder issues a Nack to the delivery with multiple flag set as false
// and requeue flag set as true. It does not reply the message.
func SingleNackRequeueErrorEncoder(ctx context.Context,
	err error, deliv *amqp.Delivery, ch Channel, pub *amqp.Publishing) {
	deliv.Nack(false, true)
	duration := getNackSleepDuration(ctx)
	time.Sleep(duration)
}

func ReplyErrorWithCodeEncoder(ctx context.Context, err error, deliv *amqp.Delivery, ch Channel, pub *amqp.Publishing) {

	if pub.CorrelationId == "" {
		pub.CorrelationId = deliv.CorrelationId
	}

	replyExchange := getPublishExchange(ctx)
	replyTo := getPublishKey(ctx)
	if replyTo == "" {
		replyTo = deliv.ReplyTo
	}

	e, ok := err.(*errors.ErrorWithCode)
	if !ok {
		e = errors.NewErrorWithCode(err.Error(), ``, http.StatusInternalServerError)
	}
	response := e

	b, err := json.Marshal(response)
	if err != nil {
		return
	}
	pub.Body = b

	ch.Publish(replyExchange, replyTo, false, false, *pub)
}

func ReplyAndAckErrorWithCodeEncoder(ctx context.Context, err error, deliv *amqp.Delivery, ch Channel, pub *amqp.Publishing) {
	ReplyErrorWithCodeEncoder(ctx, err, deliv, ch, pub)
	deliv.Ack(false)
}
