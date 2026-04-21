package httpapi

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Low-cardinality business outcome counters for critical workflows (409 conflict responses).
var businessWorkflowConflictTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "business_workflow_conflict_total",
		Help: "Count of conflict outcomes on invoice and time-entry workflow endpoints.",
	},
	[]string{"domain", "action"},
)

func recordWorkflowConflict(domain, action string) {
	businessWorkflowConflictTotal.WithLabelValues(domain, action).Inc()
}
