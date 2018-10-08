package amqp_kit

import (
	"github.com/streadway/amqp"
)

type Publisher struct {
	ch Channel
}

// NewPublisher constructs a usable Publisher for a single remote method.
func NewPublisher(ch Channel) *Publisher {
	return &Publisher{ch: ch}
}

func (p Publisher) Publish(exchange, key, corID string, body []byte) (err error) {
	pub := amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: corID,
		Body:          body,
		DeliveryMode:  amqp.Persistent,
	}

	err = p.ch.Publish(exchange, key, false, false, pub)
	if err != nil {
		return err
	}

	return err
}
