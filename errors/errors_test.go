package errors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewErrorWithCode(t *testing.T) {
	e := NewErrorWithCode(`test message`, `test_message`, http.StatusBadRequest)
	assert.Equal(t, e.Code, `test_message`)
	assert.Equal(t, e.StatusCode, http.StatusBadRequest)
	assert.Equal(t, e.Message, `test message`)
}

func TestWrapErrorWithCode(t *testing.T) {
	e := NewErrorWithCode(`test message`, `test_message`, http.StatusInternalServerError)
	e1 := WrapErrorWithCode(e, `test message 1`)

	assert.Equal(t, e1.Message, `test message 1`)
	assert.Equal(t, e.Message, `test message`)
	assert.Equal(t, e1.StatusCode, http.StatusInternalServerError)
	assert.Equal(t, e1.Code, `test_message`)
}

func TestErrorWithCode_Error(t *testing.T) {
	e := NewErrorWithCode(`test message`, `test_message`, http.StatusBadRequest)
	assert.Equal(t, `test message`, e.Error())
}

func TestEncodeError(t *testing.T) {
	req, err := http.NewRequest("GET", "/test-url", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	fn := func(w http.ResponseWriter, r *http.Request) {
		e := NewErrorWithCode(`test message`, `test_message`, http.StatusAlreadyReported)
		EncodeError(e, w)
	}

	handler := http.HandlerFunc(fn)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusAlreadyReported)
	assert.JSONEq(t, rr.Body.String(), `{"code":"test_message", "message": "test message"}`)

	fn = func(w http.ResponseWriter, r *http.Request) {
		e := fmt.Errorf(`test message 1`)
		EncodeError(e, w)
	}

	handler = http.HandlerFunc(fn)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusInternalServerError)
	assert.JSONEq(t, rr.Body.String(), `{"code":"", "message": "test message 1"}`)

	fn = func(w http.ResponseWriter, r *http.Request) {
		e := fmt.Errorf(`test message 2`)
		CtxEncodeError(req.Context(), e, w)
	}

	handler = http.HandlerFunc(fn)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusInternalServerError)
	assert.JSONEq(t, rr.Body.String(), `{"code":"", "message": "test message 2"}`)
}
