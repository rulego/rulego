package aspect

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/metrics"
)

// MetricsAspect implements comprehensive metrics collection for rule engine execution.
// It tracks various performance indicators including success/failure counts, current
// active executions, and total processed messages.
//
// MetricsAspect 实现了规则引擎执行的全面指标收集。
// 它跟踪各种性能指标，包括成功/失败计数、当前活跃执行数和总处理消息数。
//
// Features:
// 功能特性：
//   - Real-time execution tracking  实时执行跟踪
//   - Success/failure rate monitoring  成功/失败率监控
//   - Concurrent execution counting  并发执行计数
//   - Automatic metrics reset per instance  每实例自动指标重置
//   - Thread-safe atomic operations  线程安全的原子操作
//
// Metrics Collected:
// 收集的指标：
//   - TotalProcessed: Total number of messages processed  总处理消息数
//   - SuccessCount: Number of successful executions  成功执行次数
//   - FailureCount: Number of failed executions  失败执行次数
//   - CurrentActive: Current number of active executions  当前活跃执行数
//
// Usage:
// 使用方法：
//
//	// Create with default metrics instance
//	// 使用默认指标实例创建
//	metricsAspect := NewMetricsAspect(nil)
//
//	// Create with custom metrics instance
//	// 使用自定义指标实例创建
//	customMetrics := metrics.NewEngineMetrics()
//	metricsAspect := NewMetricsAspect(customMetrics)
//
//	// Apply to rule engine
//	// 应用到规则引擎
//	config := types.NewConfig().WithAspects(metricsAspect)
//	engine := rulego.NewRuleEngine(config)
//
//	// Access metrics data
//	// 访问指标数据
//	metrics := metricsAspect.GetMetrics()
//	fmt.Printf("Success rate: %.2f%%", metrics.GetSuccessRate())
type MetricsAspect struct {
	metrics *metrics.EngineMetrics // Engine metrics instance  引擎指标实例
}

var _ types.StartAspect = (*MetricsAspect)(nil)
var _ types.EndAspect = (*MetricsAspect)(nil)
var _ types.CompletedAspect = (*MetricsAspect)(nil)

// NewMetricsAspect creates a new metrics collection aspect with the specified
// metrics instance. If no metrics instance is provided, a new one is created.
//
// NewMetricsAspect 使用指定的指标实例创建新的指标收集切面。
// 如果未提供指标实例，则会创建一个新实例。
//
// Parameters:
// 参数：
//   - m: Engine metrics instance, or nil to create a new one
//     m：引擎指标实例，或 nil 以创建新实例
//
// Returns:
// 返回：
//   - *MetricsAspect: Configured metrics aspect
//     *MetricsAspect：配置好的指标切面
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
// Order 返回此切面的执行顺序。值越低，执行越早。
// 指标切面的顺序为 20，在控制切面之后但在日志记录之前执行。
func (a *MetricsAspect) Order() int {
	return 20
}

// New creates a new instance of the metrics aspect for each rule engine.
// Each new instance resets the metrics to start with clean counters.
//
// New 为每个规则引擎创建指标切面的新实例。
// 每个新实例重置指标以从干净的计数器开始。
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
// PointCut 确定此切面应用于哪些节点。
// 对所有节点返回 true 以收集全面的指标。
func (a *MetricsAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// Start is called at the beginning of rule processing. It increments both
// the current active execution counter and the total processed counter.
//
// Start 在规则处理开始时调用。它同时增加当前活跃执行计数器和总处理计数器。
//
// Metrics Updated:
// 更新的指标：
//   - CurrentActive: Incremented by 1  当前活跃数增加 1
//   - TotalProcessed: Incremented by 1  总处理数增加 1
func (a *MetricsAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	a.metrics.IncrementCurrent()
	a.metrics.IncrementTotal()
	return msg, nil
}

// End is called at the end of rule processing. It updates success or failure
// counters based on whether an error occurred during execution.
//
// End 在规则处理结束时调用。它根据执行期间是否发生错误更新成功或失败计数器。
//
// Metrics Updated:
// 更新的指标：
//   - SuccessCount: Incremented if no error  如果没有错误则成功计数增加
//   - FailureCount: Incremented if error occurred  如果发生错误则失败计数增加
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
// Completed 在规则处理完全完成时调用。它减少当前活跃执行计数器以反映完成。
//
// Metrics Updated:
// 更新的指标：
//   - CurrentActive: Decremented by 1  当前活跃数减少 1
func (a *MetricsAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	a.metrics.DecrementCurrent()
	return msg
}

// GetMetrics returns the current metrics instance containing all collected
// performance data. This allows external systems to monitor rule engine performance.
//
// GetMetrics 返回包含所有收集的性能数据的当前指标实例。
// 这允许外部系统监控规则引擎性能。
//
// Returns:
// 返回：
//   - *metrics.EngineMetrics: Current metrics data  当前指标数据
func (a *MetricsAspect) GetMetrics() *metrics.EngineMetrics {
	return a.metrics
}
