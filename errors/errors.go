package errors

import (
	"context"
	"encoding/json"
	"net/http"
)

// ErrorWithCode struct contain message, code message and http status code
type ErrorWithCode struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// Error returns error message.
func (e *ErrorWithCode) Error() string {
	return e.Message
}

// NewErrorWithCode creates a new object with a message , message code and http status code.
func NewErrorWithCode(message string, code string, statusCode int) *ErrorWithCode {
	return &ErrorWithCode{Message: message, Code: code, StatusCode: statusCode}
}

// WrapErrorWithCode use for rewrite message for base ErrorWithCode struct
func WrapErrorWithCode(e *ErrorWithCode, errMessage string) *ErrorWithCode {
	return &ErrorWithCode{Message: errMessage, Code: e.Code, StatusCode: e.StatusCode}
}

// EncodeError write json body and status code to responseWriter
func EncodeError(err error, w http.ResponseWriter) {
	e, ok := err.(*ErrorWithCode)
	if !ok {
		e = NewErrorWithCode(err.Error(), ``, http.StatusInternalServerError)
	}

	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(e)
}

// CtxEncodeError use EncodeError with context
func CtxEncodeError(_ context.Context, err error, w http.ResponseWriter) {
	EncodeError(err, w)
}
