// Copyright 2017 Aleksandr Demakin. All rights reserved.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testRequest struct {
	Data string `json:"data"`
}

func decodeTestRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request testRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func TestServer(t *testing.T) {
	a := assert.New(t)
	e := func(ctx context.Context, request interface{}) (response interface{}, err error) {
		tr := request.(testRequest)
		return testRequest{Data: tr.Data + "_resp"}, err
	}
	config := Config{
		Addr:   ":1313",
		Prefix: "/api/v1",
		Handlers: []PathInfo{
			PathInfo{
				Method: "GET",
				Path:   "/getreq",
				E:      e,
				Enc:    EncodeJSONResponse,
				Dec:    decodeTestRequest,
			},
			PathInfo{
				Method: "POST",
				Path:   "/postreq",
				E:      e,
				Enc:    EncodeJSONResponse,
				Dec:    decodeTestRequest,
			},
		},
	}
	server := NewServer(&config)
	go func() {
		a.NoError(server.Serve())
	}()
	defer func() {
		a.NoError(server.Stop())
	}()
	time.Sleep(time.Millisecond * 300) // wait for the server to start serving.
	resp, err := http.Get("http://127.0.0.1:1313/api/v1/404")
	if !a.NoError(err) {
		return
	}
	if !a.Equal(http.StatusNotFound, resp.StatusCode) {
		return
	}
	req := testRequest{Data: "req"}
	data, err := json.Marshal(req)
	if !a.NoError(err) {
		return
	}
	request, err := http.NewRequest("GET", "http://127.0.0.1:1313/api/v1/getreq", bytes.NewReader(data))
	if !a.NoError(err) {
		return
	}
	resp, err = http.DefaultClient.Do(request)
	if !a.NoError(err) {
		return
	}
	if !a.Equal(http.StatusOK, resp.StatusCode) {
		return
	}
	if !a.NoError(json.NewDecoder(resp.Body).Decode(&req)) {
		return
	}
	if !a.Equal("req_resp", req.Data) {
		return
	}
	request, err = http.NewRequest("POST", "http://127.0.0.1:1313/api/v1/postreq", bytes.NewReader(data))
	if !a.NoError(err) {
		return
	}
	resp, err = http.DefaultClient.Do(request)
	if !a.NoError(err) {
		return
	}
	if !a.Equal(http.StatusOK, resp.StatusCode) {
		return
	}
	if !a.NoError(json.NewDecoder(resp.Body).Decode(&req)) {
		return
	}
	if !a.Equal("req_resp", req.Data) {
		return
	}
}
