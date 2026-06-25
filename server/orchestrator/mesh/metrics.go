package mesh

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mesh_http_requests_total",
		Help: "Total HTTP requests by endpoint and status code.",
	}, []string{"endpoint", "method", "status"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mesh_http_request_duration_seconds",
		Help:    "HTTP request latency by endpoint.",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint", "method"})

	kafkaWritesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mesh_kafka_writes_total",
		Help: "Total Kafka write attempts by topic and result.",
	}, []string{"topic", "result"})

	serialConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mesh_serial_connected",
		Help: "1 if the serial connection to the mesh master is active, 0 otherwise.",
	})
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, kafkaWritesTotal, serialConnected)
}

// MetricsHandler returns the Prometheus HTTP handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// InstrumentHandler wraps an http.Handler with request count and duration metrics.
func InstrumentHandler(endpoint string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.status)
		httpRequestsTotal.WithLabelValues(endpoint, r.Method, status).Inc()
		httpRequestDuration.WithLabelValues(endpoint, r.Method).Observe(duration)
	})
}

// RecordKafkaWrite records a Kafka write result.
func RecordKafkaWrite(topic string, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	kafkaWritesTotal.WithLabelValues(topic, result).Inc()
}

// SetSerialConnected updates the serial connection gauge.
func SetSerialConnected(connected bool) {
	if connected {
		serialConnected.Set(1)
	} else {
		serialConnected.Set(0)
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
