package amqp_kit

import (
	"context"
	"encoding/json"

	"github.com/go-kit/kit/endpoint"
	"github.com/opentracing-contrib/go-amqp/amqptracer"
	"github.com/opentracing/opentracing-go"
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

		//extract tracing headers
		spCtx, _ := amqptracer.Extract(deliv.Headers)
		sp := opentracing.StartSpan(
			"ConsumeMessage",
			opentracing.FollowsFrom(spCtx),
		)
		defer sp.Finish()

		// Update the context with the span for the subsequent reference.
		ctx = opentracing.ContextWithSpan(ctx, sp)

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
