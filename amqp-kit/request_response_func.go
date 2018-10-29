package amqp_kit

import (
	"context"
	"github.com/streadway/amqp"
)

// RequestFunc may take information from a publisher request and put it into a
// request context. In Subscribers, RequestFuncs are executed prior to invoking
// the endpoint.
type RequestFunc func(context.Context, *amqp.Publishing) context.Context

// SubscriberResponseFunc may take information from a request context and use it to
// manipulate a Publisher. SubscriberResponseFuncs are only executed in
// subscribers, after invoking the endpoint but prior to publishing a reply.
type SubscriberResponseFunc func(context.Context, *amqp.Delivery, Channel, *amqp.Publishing) context.Context

// SetPublishExchange returns a RequestFunc that sets the Exchange field
// of AMQP Publish call
func SetPublishExchange(publishExchange string) RequestFunc {
	return func(ctx context.Context, pub *amqp.Publishing,
	) context.Context {
		return context.WithValue(ctx, ContextKeyExchange, publishExchange)
	}
}

// SetPublishKey returns a RequestFunc that sets the Key field
// of AMQP Publish call
func SetPublishKey(publishKey string) RequestFunc {
	return func(ctx context.Context, pub *amqp.Publishing,
	) context.Context {
		return context.WithValue(ctx, ContextKeyPublishKey, publishKey)
	}
}

// SetCorrelationID returns a RequestFunc that sets the CorrelationId field
// of an AMQP Publishing
func SetCorrelationID(cid string) RequestFunc {
	return func(ctx context.Context, pub *amqp.Publishing,
	) context.Context {
		pub.CorrelationId = cid
		return ctx
	}
}

// SetAckAfterEndpoint returns a SubscriberResponseFunc that prompts the service
// to Ack the Delivery object after successfully evaluating the endpoint,
// and before it encodes the response.
// It is designed to be used by Subscribers
func SetAckAfterEndpoint(multiple bool) SubscriberResponseFunc {
	return func(ctx context.Context, deliv *amqp.Delivery, ch Channel, pub *amqp.Publishing) context.Context {
		deliv.Ack(multiple)
		return ctx
	}
}

func getPublishExchange(ctx context.Context) string {
	if exchange := ctx.Value(ContextKeyExchange); exchange != nil {
		return exchange.(string)
	}
	return ""
}

func getPublishKey(ctx context.Context) string {
	if publishKey := ctx.Value(ContextKeyPublishKey); publishKey != nil {
		return publishKey.(string)
	}
	return ""
}

type contextKey int

const (
	// ContextKeyExchange is the value of the reply Exchange in
	// amqp.Publish
	ContextKeyExchange contextKey = iota
	// ContextKeyPublishKey is the value of the ReplyTo field in
	// amqp.Publish
	ContextKeyPublishKey
)
