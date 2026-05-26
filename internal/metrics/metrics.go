package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventsConsumed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trending_events_consumed_total",
		Help: "Total events consumed from broker",
	})
	EventsAccepted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trending_events_accepted_total",
		Help: "Events that passed dedup and time filters",
	})
	EventsDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trending_events_dropped_total",
		Help: "Events dropped, by reason",
	}, []string{"reason"})

	TopRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trending_top_requests_total",
		Help: "HTTP requests to /top",
	})
	TopLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "trending_top_latency_seconds",
		Help:    "Latency of /top",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 14),
	})
	SnapshotBuildSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "trending_snapshot_build_seconds",
		Help:    "Time to rebuild top snapshot",
		Buckets: prometheus.DefBuckets,
	})
)
