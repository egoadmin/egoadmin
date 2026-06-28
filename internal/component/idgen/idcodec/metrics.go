package idcodec

import (
	"time"

	"github.com/gotomicro/ego/core/emetric"
)

const (
	metricStatusOK    = "OK"
	metricStatusError = "Error"
)

var (
	idcodecOperationCounter = emetric.CounterVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idcodec",
		Name:      "operation_total",
		Help:      "Total number of idcodec operations.",
		Labels:    []string{"component", "operation", "status"},
	}.Build()

	idcodecOperationHistogram = emetric.HistogramVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idcodec",
		Name:      "operation_seconds",
		Help:      "Latency of idcodec operations.",
		Labels:    []string{"component", "operation"},
	}.Build()
)

func (c *Component) observe(operation string, begin time.Time, err error) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	status := metricStatusOK
	if err != nil {
		status = metricStatusError
	}
	idcodecOperationCounter.Inc(c.name, operation, status)
	idcodecOperationHistogram.Observe(time.Since(begin).Seconds(), c.name, operation)
}
