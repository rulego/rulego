package aspect

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/metrics"
)

// MetricsAspect implements comprehensive metrics collection for rule engine execution.
// It tracks various performance indicators including success/failure counts, current
// active executions, and total processed messages.
//
// MetricsAspect implements comprehensive metric collection for rule engine execution.
// It tracks various performance metrics, including success/failure counts, current active executions, and total message processing.
//
// Features:
// Features:
//   - Real-time execution tracking
//   - Success/failure rate monitoring
//   - Concurrent execution counting
//   - Automatic metrics reset per instance
//   - Thread-safe atomic operations
//
// Metrics Collected:
// Collected metrics:
//   - TotalProcessed: Total number of messages processed
//   - SuccessCount: Number of successful executions
//   - FailureCount: Number of failed executions
//   - CurrentActive: Current number of active executions
//
// Usage:
// How to use:
//
//	// Create with default metrics instance
//	Created using the default metric instance
//	metricsAspect := NewMetricsAspect(nil)
//
//	// Create with custom metrics instance
//	Create using custom metric instances
//	customMetrics := metrics.NewEngineMetrics()
//	metricsAspect := NewMetricsAspect(customMetrics)
//
//	// Apply to rule engine
//	Applied to the rule engine
//	config := types.NewConfig().WithAspects(metricsAspect)
//	engine := rulego.NewRuleEngine(config)
//
//	// Access metrics data
//	Access metric data
//	metrics := metricsAspect.GetMetrics()
//	fmt.Printf("Success rate: %.2f%%", metrics.GetSuccessRate())
type MetricsAspect struct {
	metrics *metrics.EngineMetrics // Engine metrics instance
}

var _ types.StartAspect = (*MetricsAspect)(nil)
var _ types.EndAspect = (*MetricsAspect)(nil)
var _ types.CompletedAspect = (*MetricsAspect)(nil)

// NewMetricsAspect creates a new metrics collection aspect with the specified
// metrics instance. If no metrics instance is provided, a new one is created.
//
// NewMetricsAspect uses the specified metric instance to create a new metric collection face.
// If no metric instance is provided, a new instance will be created.
//
// Parameters:
// Parameters:
//   - m: Engine metrics instance, or nil to create a new one
//     m: Engine metric instance, or nil to create a new instance
//
// Returns:
// Returns:
//   - *MetricsAspect: Configured metrics aspect
//     *MetricsAspect: Configured metrics aspect
func NewMetricsAspect(m *metrics.EngineMetrics) *MetricsAspect {
	if m == nil {
		m = metrics.NewEngineMetrics()
	}
	return &MetricsAspect{
		metrics: m,
	}
}

// Order returns the execution order of this aspect. Lower values execute earlier.
// Metrics aspect has order 20, executing after control aspects but before logging.
//
// Order returns the execution order of this aspect. The lower the value, the earlier it is executed.
// MetricsAspect has order 20, so it runs after control aspects but before logging.
func (a *MetricsAspect) Order() int {
	return 20
}

// New creates a new instance of the metrics aspect for each rule engine.
// Each new instance resets the metrics to start with clean counters.
//
// New: Create a new instance of the metric face for each rule engine.
// Each new instance resets the metric to start from a clean counter.
func (a *MetricsAspect) New() types.Aspect {
	if a.metrics == nil {
		a.metrics = metrics.NewEngineMetrics()
	}
	a.metrics.Reset()
	return &MetricsAspect{
		metrics: a.metrics,
	}
}

// PointCut determines which nodes this aspect applies to.
// Returns true for all nodes to collect comprehensive metrics.
//
// PointCut determines which nodes this section is applied to.
// Returns true for all nodes to collect comprehensive metrics.
func (a *MetricsAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// Start is called at the beginning of rule processing. It increments both
// the current active execution counter and the total processed counter.
//
// Start is called when rule processing begins. It simultaneously increases the currently active execution counter and the total processing counter.
//
// Metrics Updated:
// Updated metrics:
//   - CurrentActive: Incremented by 1
//   - TotalProcessed: Incremented by 1
func (a *MetricsAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	a.metrics.IncrementCurrent()
	a.metrics.IncrementTotal()
	return msg, nil
}

// End is called at the end of rule processing. It updates success or failure
// counters based on whether an error occurred during execution.
//
// End is called when rule processing is complete. It updates the success or failure counter based on whether errors occur during execution.
//
// Metrics Updated:
// Updated metrics:
//   - SuccessCount: Incremented if no error
//   - FailureCount: Incremented if error occurred
func (a *MetricsAspect) End(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	if err != nil {
		a.metrics.IncrementFailed()
	} else {
		a.metrics.IncrementSuccess()
	}
	return msg
}

// Completed is called when rule processing is fully completed. It decrements
// the current active execution counter to reflect completion.
//
// Completed is called when rule processing is fully finished. It reduces the currently active execution counter to reflect completion.
//
// Metrics Updated:
// Updated metrics:
//   - CurrentActive: Decremented by 1
func (a *MetricsAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	a.metrics.DecrementCurrent()
	return msg
}

// GetMetrics returns the current metrics instance containing all collected
// performance data. This allows external systems to monitor rule engine performance.
//
// GetMetrics returns the current metric instance containing all the collected performance data.
// This allows external systems to monitor the performance of the rule engine.
//
// Returns:
// Returns:
//   - *metrics.EngineMetrics: Current metrics data
func (a *MetricsAspect) GetMetrics() *metrics.EngineMetrics {
	return a.metrics
}
