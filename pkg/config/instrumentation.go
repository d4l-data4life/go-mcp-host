package config

import (
	"github.com/gesundheitscloud/go-svc/pkg/prom"
)

var (
	// LatencyBuckets define buckets for histogram of HTTP request/reply latency metric - in seconds
	LatencyBuckets = []float64{.0001, .001, .01, .1, .25, .5, 1, 2.5, 5, 10}
	// SizeBuckets define buckets for histogram of HTTP request/reply size metric - in bytes
	SizeBuckets = []float64{16, 32, 64, 128, 256, 512, 1024, 5120, 20480, 102400, 512000, 1000000, 10000000}
	// DefaultInstrumentOptions hold options (API-path-specific) for HTTP instrumenter - record request size and response size
	DefaultInstrumentOptions = []prom.Option{prom.WithReqSize, prom.WithRespSize}
	// DefaultInstrumentInitOptions hold initialization options (API-handler-specific) for HTTP instrumenter - definitions of histogram buckets
	DefaultInstrumentInitOptions = []prom.InitOption{
		prom.WithLatencyBuckets(LatencyBuckets),
		prom.WithSizeBuckets(SizeBuckets),
	}
)
