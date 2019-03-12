package api

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-kit/kit/endpoint"
	kit_log "github.com/go-kit/kit/log"
	opentrackingpkg "github.com/go-kit/kit/tracing/opentracing"
	kit_http "github.com/go-kit/kit/transport/http"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/space307/go-utils/v3/metrics"
)

// AddMetrics is functions used to adds metrics and tracing middleware to all Handlers
func AddMetrics(cfg *Config) error {
	var operationName string

	// find all slashes and variables in handler path
	re := regexp.MustCompile(`(\/\{.+\})|\/`)
	handlers := cfg.Handlers
	trm := metrics.NewMetrics()
	tracer := opentracing.GlobalTracer()
	logger := kit_log.NewNopLogger()

	for index, handler := range handlers {
		if handler.Name == "" {
			s := re.ReplaceAllString(handler.Path, "_")
			operationName = strings.Trim(s, "_")
		} else {
			operationName = handler.Name
		}

		if operationName == "" {
			return fmt.Errorf("Field \"Name\" can't be empty for path \"%s\"", handler.Path)
		}

		handler.E = endpoint.Chain(
			metrics.MetricsMiddleware(trm, operationName),
			opentrackingpkg.TraceServer(tracer, operationName),
		)(handler.E)

		handler.Dec = metrics.MetricsDecodeWrapper(trm, operationName, handler.Dec)

		req := opentrackingpkg.HTTPToContext(tracer, operationName, logger)
		handler.O = append(handler.O, kit_http.ServerBefore(req))

		cfg.Handlers[index] = handler
	}

	return nil
}
