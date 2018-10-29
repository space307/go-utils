package amqp_kit

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/streadway/amqp"
)

// Error struct contain message, code message and http status code for amqp response
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

// Error returns error message.
func (e *Error) Error() string {
	return e.Message
}

// NewError creates a new object with a message , message code and http status code.
func NewError(message string, code string, statusCode int) *Error {
	return &Error{Message: message, Code: code, StatusCode: statusCode}
}

// WrapError use for rewrite message for base amqp Error struct
func WrapError(e *Error, errMessage string) *Error {
	return &Error{Message: errMessage, Code: e.Code, StatusCode: e.StatusCode}
}

// ErrorEncoder is responsible for encoding an error to the subscriber reply.
// Users are encouraged to use custom ErrorEncoders to encode errors to
// their replies, and will likely want to pass and check for their own error
// types.
type ErrorEncoder func(ctx context.Context, err error, d *amqp.Delivery, ch Channel, pub *amqp.Publishing)

// ReplyErrorWithCodeEncoder base decoder for error response
func ReplyErrorWithCodeEncoder(ctx context.Context, err error, d *amqp.Delivery, ch Channel, pub *amqp.Publishing) {

	if pub.CorrelationId == "" {
		pub.CorrelationId = d.CorrelationId
	}

	replyExchange := getPublishExchange(ctx)
	replyTo := getPublishKey(ctx)
	if replyTo == "" {
		replyTo = d.ReplyTo
	}

	e, ok := err.(*Error)
	if !ok {
		e = NewError(err.Error(), ``, http.StatusInternalServerError)
	}

	b, err := json.Marshal(Response{Error: e})
	if err != nil {
		return
	}
	pub.Body = b

	ch.Publish(replyExchange, replyTo, false, false, *pub)
}

// ReplyAndAckErrorWithCodeEncoder call ReplyErrorWithCodeEncoder method and Ack delivery message
func ReplyAndAckErrorWithCodeEncoder(ctx context.Context, err error, d *amqp.Delivery, ch Channel, pub *amqp.Publishing) {
	ReplyErrorWithCodeEncoder(ctx, err, d, ch, pub)
	d.Ack(false)
}

// Response base response object with data and error field
type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *Error      `json:"error,omitempty"`
}
