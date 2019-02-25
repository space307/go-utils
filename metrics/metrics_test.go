package metrics

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsChainBuilder(t *testing.T) {
	metricsBuilder := MetricsChainBuilder(NewMetrics())

	req, err := http.NewRequest("", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	httpHandler := metricsBuilder("test_metrics")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Millisecond)
	}))

	// Simulate HTTP request processing.
	httpHandler.ServeHTTP(httptest.NewRecorder(), req)

	// Call metrics handler.
	metricsRespRecorder := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(metricsRespRecorder, req)

	metricsRespBody, err := ioutil.ReadAll(metricsRespRecorder.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(metricsRespBody), `request_latency_ms_bucket{error="unknown",method="test_metrics",le="0.005"} 0.0`) {
		t.Errorf("request latency is smaller than %f seconds", 0.005)
	}

	if !strings.Contains(string(metricsRespBody), `request_latency_ms_bucket{error="unknown",method="test_metrics",le="0.01"} 0.0`) {
		t.Errorf("request latency is smaller than %f seconds", 0.01)
	}

	if !strings.Contains(string(metricsRespBody), `request_latency_ms_bucket{error="unknown",method="test_metrics",le="0.025"} 1.0`) {
		t.Errorf("request latency is more than %f seconds", 0.025)
	}

	if !strings.Contains(string(metricsRespBody), `request_count{error="unknown",method="test_metrics",valid="unknown"} 1.0`) {
		t.Errorf("request count is not equal to 1")
	}
}
