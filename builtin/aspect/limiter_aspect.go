package aspect

import (
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
)

// ConcurrencyLimiterAspect implements a concurrency limiter using atomic operations
// to restrict the number of concurrent rule executions. This aspect prevents
// system overload by controlling parallel processing.
//
// ConcurrencyLimiterAspect 使用原子操作实现并发限制器，限制并发规则执行的数量。
// 此切面通过控制并行处理来防止系统过载。
//
// Features:
// 功能特性：
//   - Atomic operations for thread-safe counting  原子操作确保线程安全计数
//   - Compare-and-swap (CAS) for consistent state  比较并交换（CAS）确保状态一致性
//   - Configurable maximum concurrent executions  可配置的最大并发执行数量
//   - Automatic cleanup on completion  完成时自动清理
//   - Returns ErrConcurrencyLimitReached when limit exceeded  超过限制时返回 ErrConcurrencyLimitReached
//
// Usage:
// 使用方法：
//
//	// Create aspect with maximum 100 concurrent executions
//	// 创建最大 100 个并发执行的切面
//	limiter := NewConcurrencyLimiterAspect(100)
//	config := types.NewConfig().WithAspects(limiter)
//	engine := rulego.NewRuleEngine(config)
type ConcurrencyLimiterAspect struct {
	Max          int64 // Maximum number of concurrent executions  最大并发执行数量
	currentCount int64 // Current number of concurrent executions  当前并发执行数量
}

var _ types.StartAspect = (*ConcurrencyLimiterAspect)(nil)
var _ types.CompletedAspect = (*ConcurrencyLimiterAspect)(nil)

// NewConcurrencyLimiterAspect creates a new concurrency limiter aspect with the specified
// maximum number of concurrent executions. This factory function initializes the aspect
// with proper configuration.
//
// NewConcurrencyLimiterAspect 创建具有指定最大并发执行数量的新并发限制切面。
// 此工厂函数使用适当的配置初始化切面。
//
// Parameters:
// 参数：
//   - max: Maximum number of concurrent rule executions allowed
//     max：允许的最大并发规则执行数量
//
// Returns:
// 返回：
//   - *ConcurrencyLimiterAspect: Configured concurrency limiter aspect
//     *ConcurrencyLimiterAspect：配置好的并发限制切面
func NewConcurrencyLimiterAspect(max int) *ConcurrencyLimiterAspect {
	return &ConcurrencyLimiterAspect{
		Max: int64(max),
	}
}

// Order returns the execution priority of this aspect. Lower values execute earlier.
// This aspect has order 10, making it one of the first aspects to execute.
//
// Order 返回此切面的执行优先级。值越低，执行越早。
// 此切面的顺序为 10，使其成为最先执行的切面之一。
func (a *ConcurrencyLimiterAspect) Order() int {
	return 10
}

// New creates a new instance of the aspect for each rule engine instance.
// Each instance maintains its own concurrency counter starting from zero.
//
// New 为每个规则引擎实例创建切面的新实例。
// 每个实例维护自己的并发计数器，从零开始。
func (a *ConcurrencyLimiterAspect) New() types.Aspect {
	return &ConcurrencyLimiterAspect{
		Max:          a.Max,
		currentCount: 0,
	}
}

// PointCut determines which nodes this aspect applies to.
// Returns true for all nodes, applying concurrency limiting globally.
//
// PointCut 确定此切面应用于哪些节点。
// 对所有节点返回 true，全局应用并发限制。
func (a *ConcurrencyLimiterAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// Start is called at the beginning of rule processing. It implements a thread-safe
// concurrency check using atomic operations and compare-and-swap to ensure the
// current execution count doesn't exceed the maximum limit.
//
// Start 在规则处理开始时调用。它使用原子操作和比较并交换实现线程安全的并发检查，
// 确保当前执行计数不超过最大限制。
//
// Algorithm:
// 算法：
//  1. Load current count atomically  原子加载当前计数
//  2. Check if limit would be exceeded  检查是否会超过限制
//  3. Use CAS to increment if within limit  如果在限制内则使用 CAS 增加
//  4. Retry if CAS fails due to concurrent modification  如果由于并发修改导致 CAS 失败则重试
//
// Returns:
// 返回：
//   - types.RuleMsg: The original message unchanged  原始消息不变
//   - error: ErrConcurrencyLimitReached if limit exceeded, nil otherwise
//     error：如果超过限制则返回 ErrConcurrencyLimitReached，否则为 nil
func (a *ConcurrencyLimiterAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	// 使用原子操作确保检查和增加操作的原子性
	for {
		current := atomic.LoadInt64(&a.currentCount)
		if current >= a.Max {
			return msg, types.ErrConcurrencyLimitReached
		}
		// 尝试原子地增加计数器，如果成功则退出循环
		if atomic.CompareAndSwapInt64(&a.currentCount, current, current+1) {
			break
		}
		// 如果CAS失败，说明有其他goroutine修改了计数器，重试
	}
	return msg, nil
}

// Completed is called when rule processing is finished. It atomically decrements
// the current execution count, allowing new executions to proceed.
//
// Completed 在规则处理完成时调用。它原子地减少当前执行计数，允许新的执行继续。
//
// This method ensures proper cleanup and maintains accurate concurrency tracking.
// 此方法确保正确清理并维护准确的并发跟踪。
func (a *ConcurrencyLimiterAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	atomic.AddInt64(&a.currentCount, -1)
	return msg
}

// incrementCurrent atomically increments the current execution count.
// This is an internal helper method for testing purposes.
//
// incrementCurrent 原子地增加当前执行计数。
// 这是用于测试目的的内部辅助方法。
func (a *ConcurrencyLimiterAspect) incrementCurrent() {
	atomic.AddInt64(&a.currentCount, 1)
}

// decrementCurrent atomically decrements the current execution count.
// This is an internal helper method for testing purposes.
//
// decrementCurrent 原子地减少当前执行计数。
// 这是用于测试目的的内部辅助方法。
func (a *ConcurrencyLimiterAspect) decrementCurrent() {
	atomic.AddInt64(&a.currentCount, -1)
}
