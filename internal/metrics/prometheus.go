package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	RequestDuration  prometheus.Histogram
	RateLimitRejects prometheus.Counter
	CacheHits        prometheus.Counter
	CacheMisses      prometheus.Counter
	ProxyErrors      prometheus.Counter
}

func New() *Metrics {
	return &Metrics{
		RequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		}),
		RateLimitRejects: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_rate_limit_rejects_total",
			Help: "Total number of rate limit rejects",
		}),
		CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_cache_hits_total",
			Help: "Total number of cache hits",
		}),
		CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_cache_misses_total",
			Help: "Total number of cache misses",
		}),
		ProxyErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_proxy_errors_total",
			Help: "Total number of proxy errors",
		}),
	}
}
