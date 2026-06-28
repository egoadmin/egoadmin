package logincrypto

import (
	"time"

	"github.com/gotomicro/ego/core/emetric"
)

const (
	metricCodeOK    = "OK"
	metricCodeError = "Error"
)

var (
	componentHandleCounter = emetric.CounterVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "logincrypto",
		Name:      "handle_total",
		Help:      "Total number of logincrypto operations.",
		Labels:    []string{"name", "operation", "code"},
	}.Build()

	componentHandleHistogram = emetric.HistogramVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "logincrypto",
		Name:      "handle_seconds",
		Help:      "Latency of logincrypto operations.",
		Labels:    []string{"name", "operation"},
	}.Build()
)

func (c *Component) observe(operation string, begin time.Time, err error) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	code := metricCodeOK
	if err != nil {
		code = metricCodeError
	}
	componentHandleCounter.Inc(c.name, operation, code)
	componentHandleHistogram.Observe(time.Since(begin).Seconds(), c.name, operation)
}
