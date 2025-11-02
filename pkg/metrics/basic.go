package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// Metric definitions
// Ensure that his follows best practices for naming: https://prometheus.io/docs/practices/naming/
var (
	metricNamePrefix = "d4l_GO_SVC_TEMPLATE"
)

// AddBuildInfoMetric adds a static metric with the build information
func AddBuildInfoMetric() {
	err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: metricNamePrefix,
			Name:      "build_info",
			Help:      "A metric with a constant '1' value labeled by version, branch, commit, build date, and goversion.",
			ConstLabels: prometheus.Labels{
				"version":   config.Version,
				"branch":    config.Branch,
				"commit":    config.Commit,
				"goversion": config.GoVersion,
			},
		},
		func() float64 { return 1 },
	))
	if err != nil {
		logging.LogErrorf(err, "Error registering buld info metric")
	}
}
