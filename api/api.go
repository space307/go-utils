// Copyright 2017 Aleksandr Demakin. All rights reserved.

// Package api implements a simple REST api server.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/kit/endpoint"
	kit_http "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

const (
	cDefaultReadTimeout  = 10 * time.Second
	cDefaultWriteTimeout = 10 * time.Second
)

// ErrorWithCode contains a message and code, which will be used as a http response code.
type ErrorWithCode struct {
	code     int
	message  string
	response interface{}
}

// NewErrorWithCode creates a new object with a message and code.
func NewErrorWithCode(message string, code int) *ErrorWithCode {
	return &ErrorWithCode{message: message, code: code}
}

// NewErrorWithCodeResponse creates a new object with a message, code and response
func NewErrorWithCodeResponse(message string, code int, resp interface{}) *ErrorWithCode {
	return &ErrorWithCode{message: message, code: code, response: resp}
}

// StatusCode returns error code.
func (e *ErrorWithCode) StatusCode() int {
	return e.code
}

// Error returns error message.
func (e *ErrorWithCode) Error() string {
	return e.message
}

// Error returns marshal response
func (e *ErrorWithCode) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.response)
}

// PathInfo represents information about path, method, and request handlers to serve this path.
type PathInfo struct {
	Method string
	Name   string
	Path   string
	E      endpoint.Endpoint
	Dec    kit_http.DecodeRequestFunc
	Enc    kit_http.EncodeResponseFunc
	O      []kit_http.ServerOption
}

// Config used to pass settings and handlers to the server.
type Config struct {
	Addr                      string
	Prefix                    string
	Handlers                  []PathInfo
	ReadTimeout, WriteTimeout time.Duration
}

// Server is a REST api server.
type Server struct {
	cfg    *Config
	server *http.Server
	m      sync.RWMutex
}

// NewServer creates a new server object for the given config.
func NewServer(cfg *Config) *Server {
	s := &Server{cfg: cfg}
	r := mux.NewRouter().StrictSlash(true)
	router := r.PathPrefix(s.cfg.Prefix).Subrouter()
	for _, s := range s.cfg.Handlers {
		srv := kit_http.NewServer(s.E, s.Dec, s.Enc, s.O...)
		router.Handle(s.Path, srv).Methods(s.Method)
	}
	s.m.Lock()

	// override timeouts
	readTimeout := cDefaultReadTimeout
	writeTimeout := cDefaultWriteTimeout

	if cfg.ReadTimeout > 0 {
		readTimeout = cfg.ReadTimeout
	}

	if cfg.WriteTimeout > 0 {
		writeTimeout = cfg.WriteTimeout
	}

	s.server = &http.Server{
		Addr:           s.cfg.Addr,
		Handler:        r,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	s.m.Unlock()

	return s
}

// Serve launches processing loop.
func (s *Server) Serve() error {
	s.m.RLock()
	srv := s.server
	s.m.RUnlock()
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed { // s.Stop() was called, not a error.
		err = nil
	}
	return err
}

// Stop gracefully shutdowns http server.
func (s *Server) Stop() error {
	s.m.RLock()
	srv := s.server
	s.m.RUnlock()

	return srv.Shutdown(context.TODO())
}

// StopNow immediately shutdowns http server.
func (s *Server) StopNow() error {
	s.m.RLock()
	srv := s.server
	s.m.RUnlock()

	return srv.Close()
}

// RequestVars returns request's path variables.
func RequestVars(r *http.Request) map[string]string {
	return mux.Vars(r)
}

// EncodeJSONResponse writes json response into http.ResponseWriter.
func EncodeJSONResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}
