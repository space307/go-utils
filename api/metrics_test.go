package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestAddMetrics(t *testing.T) {
	a := assert.New(t)

	e := func(ctx context.Context, request interface{}) (response interface{}, err error) {
		return nil, err
	}

	config := Config{
		Handlers: []PathInfo{
			PathInfo{
				Method: "GET",
				Path:   "/getreq",
				E:      e,
				Enc:    EncodeJSONResponse,
				Dec:    decodeTestRequest,
			},
		},
	}

	err := AddMetrics(&config)
	a.NoError(err)
	a.Len(config.Handlers[0].O, 1)

	config = Config{
		Handlers: []PathInfo{
			PathInfo{
				Method: "GET",
				Path:   "/",
				E:      e,
				Enc:    EncodeJSONResponse,
				Dec:    decodeTestRequest,
			},
		},
	}

	err = AddMetrics(&config)
	a.Errorf(err, "Field \"Name\" can't be empty for path \"/\"")
}

func TestServerMetrics(t *testing.T) {
	opentracing.SetGlobalTracer(mocktracer.New())

	a := assert.New(t)
	e := func(ctx context.Context, request interface{}) (response interface{}, err error) {
		return nil, err
	}
	enc := func(_ context.Context, w http.ResponseWriter, response interface{}) error {
		return nil
	}

	dec := func(_ context.Context, r *http.Request) (interface{}, error) {
		return nil, nil
	}

	config := Config{
		Addr: ":1515",
		Handlers: []PathInfo{
			PathInfo{
				Method: "GET",
				Path:   "/getreq",
				E:      e,
				Enc:    enc,
				Dec:    dec,
			},
			PathInfo{
				Method: "GET",
				Name:   "custom_name",
				Path:   "/name",
				E:      e,
				Enc:    enc,
				Dec:    dec,
			},
			PathInfo{
				Method: "GET",
				Path:   "/articles/{id:[0-9]+}",
				E:      e,
				Enc:    enc,
				Dec:    dec,
			},
		},
	}

	err := AddMetrics(&config)
	a.NoError(err)

	server := NewServer(&config)
	metricsServer := httptest.NewServer(promhttp.Handler())

	go func() {
		a.NoError(server.Serve())
	}()
	defer func() {
		a.NoError(server.Stop())
	}()

	time.Sleep(time.Millisecond * 300) // wait for the server to start serving.

	_, err = http.Get("http://127.0.0.1:1515/getreq")
	if !a.NoError(err) {
		return
	}

	_, err = http.Get("http://127.0.0.1:1515/name")
	if !a.NoError(err) {
		return
	}

	_, err = http.Get("http://127.0.0.1:1515/articles/10")
	if !a.NoError(err) {
		return
	}

	resp, err := http.Get(fmt.Sprintf("%s", metricsServer.URL))
	if !a.NoError(err) {
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	resp.Body.Close()

	a.Contains(buf.String(), `request_count{error="false",method="getreq",valid="true"} 1`)
	a.Contains(buf.String(), `request_count{error="false",method="custom_name",valid="true"} 1`)
	a.Contains(buf.String(), `request_count{error="false",method="articles",valid="true"} 1`)

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	a.Len(finishedSpans, 3)
	a.Equal("getreq", finishedSpans[0].OperationName)
	a.Equal("custom_name", finishedSpans[1].OperationName)
	a.Equal("articles", finishedSpans[2].OperationName)
}
