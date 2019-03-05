package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/metrics"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httpgk "github.com/go-kit/kit/transport/http"
	"github.com/prometheus/client_golang/prometheus"
)

// Represents any unknown string field value.
const fieldValueUnknown = "unknown"

var requestCount = kitprometheus.NewCounterFrom(prometheus.CounterOpts{
	Name: "request_count",
	Help: "Number of requests received",
}, []string{"method", "error", "valid"})

var requestLatency = kitprometheus.NewHistogramFrom(prometheus.HistogramOpts{
	Name: "request_latency_ms",
	Help: "Duration of requests in ms",
}, []string{"method", "error"})

type aliveMetric struct {
	aliveTime metrics.Gauge
}

func NewAliveMetric() *aliveMetric {
	return &aliveMetric{
		aliveTime: kitprometheus.NewGaugeFrom(prometheus.GaugeOpts{
			Name: "heartbeat",
			Help: "Unix time in seconds when this service was alive",
		}, []string{}),
	}
}

func (am *aliveMetric) Update() {
	am.aliveTime.Set(float64(time.Now().Unix()))
}

type MetricsStorage struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
}

func NewMetrics() *MetricsStorage {
	return &MetricsStorage{requestCount: requestCount, requestLatency: requestLatency}
}

func (ms *MetricsStorage) counterAdd(labels []string) {
	ms.requestCount.With(labels...).Add(1)
}

func (ms *MetricsStorage) latencyAdd(labels []string, begin time.Time) {
	ms.requestLatency.With(labels...).Observe(time.Since(begin).Seconds())
}

func (ms *MetricsStorage) CounterSet(method string, count float64) {
	labels := []string{`method`, method, `error`, `false`, `valid`, `true`}
	ms.requestCount.With(labels...).Add(count)
}

func MetricsDecodeWrapper(m *MetricsStorage, method string, d httpgk.DecodeRequestFunc) httpgk.DecodeRequestFunc {
	return func(ctx context.Context, request *http.Request) (i interface{}, err error) {
		defer func(begin time.Time) {
			if err != nil {
				labels := []string{`method`, method, `error`, `true`, `valid`, `false`}
				m.counterAdd(labels)
			}
		}(time.Now())

		i, err = d(ctx, request)
		return i, err
	}
}

func MetricsMiddleware(m *MetricsStorage, method string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (i interface{}, err error) {
			defer func(begin time.Time) {
				labels := []string{`method`, method, `error`, fmt.Sprint(err != nil), `valid`, `true`}
				m.counterAdd(labels)
				m.latencyAdd(labels[0:3], begin)
			}(time.Now())

			return next(ctx, request)
		}
	}
}

// Returns a function which generates metrics http handler middleware.
// It is useful for alice chains (see https://github.com/justinas/alice).
func MetricsChainBuilder(m *MetricsStorage) func(method string) func(handler http.Handler) http.Handler {
	return func(method string) func(handler http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func(begin time.Time) {
					labels := []string{"method", method, "error", fieldValueUnknown, "valid", fieldValueUnknown}
					m.counterAdd(labels)
					m.latencyAdd(labels[0:3], begin)
				}(time.Now())

				next.ServeHTTP(w, r)
			})
		}
	}
}
