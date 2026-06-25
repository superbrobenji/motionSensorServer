package mesh

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetricsHandler_ReturnsPrometheusFormat verifies that MetricsHandler
// returns a 200 response with Prometheus text format content.
func TestMetricsHandler_ReturnsPrometheusFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain Content-Type, got %s", ct)
	}
}

// TestInstrumentHandler_RecordsRequestCount verifies that InstrumentHandler
// increments the mesh_http_requests_total counter after a request.
func TestInstrumentHandler_RecordsRequestCount(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := InstrumentHandler("/test-endpoint", inner)

	req := httptest.NewRequest("GET", "/test-endpoint", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Scrape metrics and look for our counter
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(metricsW, metricsReq)

	body, _ := io.ReadAll(metricsW.Body)
	if !strings.Contains(string(body), "mesh_http_requests_total") {
		t.Errorf("expected mesh_http_requests_total in metrics output")
	}
}

// TestInstrumentHandler_RecordsDuration verifies that InstrumentHandler
// records the mesh_http_request_duration_seconds histogram.
func TestInstrumentHandler_RecordsDuration(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := InstrumentHandler("/duration-test", inner)

	req := httptest.NewRequest("GET", "/duration-test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(metricsW, metricsReq)

	body, _ := io.ReadAll(metricsW.Body)
	if !strings.Contains(string(body), "mesh_http_request_duration_seconds") {
		t.Errorf("expected mesh_http_request_duration_seconds in metrics output")
	}
}

// TestRecordKafkaWrite_RecordsSuccessAndError verifies that RecordKafkaWrite
// increments the mesh_kafka_writes_total counter for both success and error outcomes.
func TestRecordKafkaWrite_RecordsSuccessAndError(t *testing.T) {
	RecordKafkaWrite("test-topic", nil)
	RecordKafkaWrite("test-topic", fmt.Errorf("write failed"))

	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(metricsW, metricsReq)

	body, _ := io.ReadAll(metricsW.Body)
	s := string(body)
	if !strings.Contains(s, "mesh_kafka_writes_total") {
		t.Errorf("expected mesh_kafka_writes_total in metrics output")
	}
	if !strings.Contains(s, `result="success"`) {
		t.Errorf("expected result=\"success\" label in metrics output")
	}
	if !strings.Contains(s, `result="error"`) {
		t.Errorf("expected result=\"error\" label in metrics output")
	}
}

// TestSetSerialConnected_UpdatesGauge verifies that SetSerialConnected
// is reflected in the mesh_serial_connected gauge.
func TestSetSerialConnected_UpdatesGauge(t *testing.T) {
	SetSerialConnected(true)

	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(metricsW, metricsReq)

	body, _ := io.ReadAll(metricsW.Body)
	s := string(body)
	if !strings.Contains(s, "mesh_serial_connected") {
		t.Errorf("expected mesh_serial_connected in metrics output")
	}
	if !strings.Contains(s, "mesh_serial_connected 1") {
		t.Errorf("expected mesh_serial_connected 1 after SetSerialConnected(true)")
	}

	SetSerialConnected(false)
	metricsReq2 := httptest.NewRequest("GET", "/metrics", nil)
	metricsW2 := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(metricsW2, metricsReq2)

	body2, _ := io.ReadAll(metricsW2.Body)
	s2 := string(body2)
	if !strings.Contains(s2, "mesh_serial_connected 0") {
		t.Errorf("expected mesh_serial_connected 0 after SetSerialConnected(false)")
	}
}

// TestMetricsEndpoint_ExemptFromAuth verifies that the /metrics endpoint
// is accessible without authentication even when an API key is set.
func TestMetricsEndpoint_ExemptFromAuth(t *testing.T) {
	ms := newTestMeshServer(t)
	api := NewAPIServer(ms, "secret-key", nil)

	req := httptest.NewRequest("GET", "/metrics", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected /metrics to be accessible without auth (200), got %d", w.Code)
	}
}

// TestMetricsEndpoint_AuthStillProtectsOtherRoutes verifies that the auth
// middleware still applies to non-metrics routes when an API key is set.
func TestMetricsEndpoint_AuthStillProtectsOtherRoutes(t *testing.T) {
	ms := newTestMeshServer(t)
	api := NewAPIServer(ms, "secret-key", nil)

	req := httptest.NewRequest("GET", "/status", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected /status to require auth (401), got %d", w.Code)
	}
}
