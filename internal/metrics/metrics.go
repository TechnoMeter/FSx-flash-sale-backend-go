package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestTotal counts all HTTP requests by path and status code.
	RequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "status"},
	)

	// RequestDuration measures latency of HTTP requests.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"path"},
	)

	// InventoryStock is the current remaining stock.
	InventoryStock = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "flash_sale_inventory_stock",
			Help: "Current remaining inventory count",
		},
	)

	// StreamQueueLength is the number of messages in the Redis Stream (XLEN).
	StreamQueueLength = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "flash_sale_stream_queue_length",
			Help: "Number of pending messages in the Redis Stream",
		},
	)

	// PendingMessages is the count of messages pending acknowledgement (XPENDING).
	PendingMessages = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "flash_sale_pending_messages",
			Help: "Number of unacknowledged messages in the consumer group",
		},
	)

	// WorkerProcessedTotal counts successfully persisted orders.
	WorkerProcessedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "flash_sale_worker_processed_total",
			Help: "Total number of orders successfully persisted by the worker",
		},
	)

	// CircuitBreakerState: 0 = closed, 1 = open, 2 = half-open.
	CircuitBreakerState = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "flash_sale_circuit_breaker_state",
			Help: "Circuit breaker state: 0=closed, 1=open, 2=half-open",
		},
	)

	// ReconcilerCorrectionsTotal counts how many times the reconciler fixed a mismatch.
	ReconcilerCorrectionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "flash_sale_reconciler_corrections_total",
			Help: "Total number of inventory corrections performed by the reconciler",
		},
	)
)

// RegisterAll is a no-op because promauto registers automatically.
func RegisterAll() {
	// All metrics are already registered.
}