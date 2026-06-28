package idgen

import (
	"time"

	"github.com/gotomicro/ego/core/emetric"
)

const (
	metricStatusOK      = "OK"
	metricStatusError   = "Error"
	metricStatusSkipped = "Skipped"
	metricStatusStale   = "Stale"
	metricNameAll       = "_all"
)

var (
	idgenGeneratedCounter = emetric.CounterVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "generated_total",
		Help:      "Total number of generated IDs.",
		Labels:    []string{"component", "name", "operation", "status"},
	}.Build()

	idgenSegmentFetchCounter = emetric.CounterVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "segment_fetch_total",
		Help:      "Total number of idgen segment fetches.",
		Labels:    []string{"component", "name", "status"},
	}.Build()

	idgenSegmentFetchHistogram = emetric.HistogramVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "segment_fetch_seconds",
		Help:      "Latency of idgen segment fetches.",
		Labels:    []string{"component", "name"},
	}.Build()

	idgenSegmentRemainingGauge = emetric.GaugeVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "segment_remaining",
		Help:      "Remaining IDs in the current local segment.",
		Labels:    []string{"component", "name"},
	}.Build()

	idgenSegmentStepGauge = emetric.GaugeVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "segment_step",
		Help:      "Current effective idgen segment step.",
		Labels:    []string{"component", "name"},
	}.Build()

	idgenMachineRenewCounter = emetric.CounterVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "machine_lease_renew_total",
		Help:      "Total number of idgen machine lease renew operations.",
		Labels:    []string{"component", "status"},
	}.Build()

	idgenPrefetchCounter = emetric.CounterVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "prefetch_total",
		Help:      "Total number of idgen segment prefetch attempts.",
		Labels:    []string{"component", "name", "status"},
	}.Build()

	idgenMachineLeaseStatusGauge = emetric.GaugeVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "machine_lease_status",
		Help:      "Current idgen machine lease status. 1 means valid, 0 means unavailable.",
		Labels:    []string{"component"},
	}.Build()

	idgenHealthStatusGauge = emetric.GaugeVecOpts{
		Namespace: emetric.DefaultNamespace,
		Subsystem: "idgen",
		Name:      "health_status",
		Help:      "Current idgen health status. 1 means healthy, 0 means unhealthy.",
		Labels:    []string{"component"},
	}.Build()
)

func (c *Component) metricName(name string) string {
	if c == nil || c.config == nil || c.config.EnableNameMetricLabel {
		return name
	}
	return metricNameAll
}

func (c *Component) observeGenerate(name string, operation string, err error) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	status := metricStatusOK
	if err != nil {
		status = metricStatusError
	}
	idgenGeneratedCounter.Inc(c.name, c.metricName(name), operation, status)
}

func (c *Component) observeSegmentFetch(name string, begin time.Time, err error) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	status := metricStatusOK
	if err != nil {
		status = metricStatusError
	}
	metricName := c.metricName(name)
	idgenSegmentFetchCounter.Inc(c.name, metricName, status)
	idgenSegmentFetchHistogram.Observe(time.Since(begin).Seconds(), c.name, metricName)
}

func (c *Component) observeRemaining(name string, remaining int64) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	idgenSegmentRemainingGauge.Set(float64(remaining), c.name, c.metricName(name))
}

func (c *Component) observeStep(name string, step int64) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	idgenSegmentStepGauge.Set(float64(step), c.name, c.metricName(name))
}

func observeMachineRenew(componentName string, err error) {
	status := metricStatusOK
	if err != nil {
		status = metricStatusError
	}
	idgenMachineRenewCounter.Inc(componentName, status)
}

func (c *Component) observePrefetch(name string, status string) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	if status == "" {
		status = metricStatusOK
	}
	idgenPrefetchCounter.Inc(c.name, c.metricName(name), status)
}

func observeMachineLeaseStatus(componentName string, valid bool) {
	if valid {
		idgenMachineLeaseStatusGauge.Set(1, componentName)
		return
	}
	idgenMachineLeaseStatusGauge.Set(0, componentName)
}

func (c *Component) observeHealth(err error) {
	if c == nil || c.config == nil || !c.config.EnableMetrics {
		return
	}
	if err == nil {
		idgenHealthStatusGauge.Set(1, c.name)
		return
	}
	idgenHealthStatusGauge.Set(0, c.name)
}
