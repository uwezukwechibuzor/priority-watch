package chain

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ClientMetrics holds Prometheus metrics for the RPC client.
type ClientMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	retriesTotal    *prometheus.CounterVec
}

// NewClientMetrics registers and returns metrics in reg.
// All metrics are namespaced under "pw_rpc".
func NewClientMetrics(reg prometheus.Registerer) *ClientMetrics {
	m := &ClientMetrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pw",
			Subsystem: "rpc",
			Name:      "requests_total",
			Help:      "Total RPC requests by method and status (ok|error).",
		}, []string{"method", "status"}),

		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "pw",
			Subsystem: "rpc",
			Name:      "request_duration_seconds",
			Help:      "RPC request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		retriesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pw",
			Subsystem: "rpc",
			Name:      "retries_total",
			Help:      "Total RPC retry attempts by method.",
		}, []string{"method"}),
	}

	reg.MustRegister(m.requestsTotal, m.requestDuration, m.retriesTotal)
	return m
}

func (m *ClientMetrics) incRequest(method, status string) {
	m.requestsTotal.WithLabelValues(method, status).Inc()
}

func (m *ClientMetrics) observeDuration(method string, secs float64) {
	m.requestDuration.WithLabelValues(method).Observe(secs)
}

func (m *ClientMetrics) incRetry(method string) {
	m.retriesTotal.WithLabelValues(method).Inc()
}
