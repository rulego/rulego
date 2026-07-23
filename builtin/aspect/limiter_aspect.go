package aspect

import (
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
)

// ConcurrencyLimiterAspect implements a concurrency limiter using atomic operations
// to restrict the number of concurrent rule executions. This aspect prevents
// system overload by controlling parallel processing.
//
// ConcurrencyLimiterAspect uses atomic operations to implement a concurrency limiter, limiting the number of executions of concurrency rules.
// This section prevents system overload by controlling parallel processing.
//
// Features:
// Features:
//   - Atomic operations for thread-safe counting
//   - Compare-and-swap (CAS) for consistent state
//   - Configurable maximum concurrent executions
//   - Automatic cleanup on completion
//   - Returns ErrConcurrencyLimitReached when limit exceeded
//
// Usage:
// How to use:
//
//	// Create aspect with maximum 100 concurrent executions
//	Create an aspect that permits up to 100 concurrent executions
//	limiter := NewConcurrencyLimiterAspect(100)
//	config := types.NewConfig().WithAspects(limiter)
//	engine := rulego.NewRuleEngine(config)
type ConcurrencyLimiterAspect struct {
	Max          int64 // Maximum number of concurrent executions
	currentCount int64 // Current number of concurrent executions
}

var _ types.StartAspect = (*ConcurrencyLimiterAspect)(nil)
var _ types.CompletedAspect = (*ConcurrencyLimiterAspect)(nil)

// NewConcurrencyLimiterAspect creates a new concurrency limiter aspect with the specified
// maximum number of concurrent executions. This factory function initializes the aspect
// with proper configuration.
//
// NewConcurrencyLimiterAspect creates a new concurrency limiting aspect with a specified maximum number of concurrent executions.
// This factory function uses the appropriate configuration to initialize the face.
//
// Parameters:
// Parameters:
//   - max: Maximum number of concurrent rule executions allowed
//     max: The maximum number of concurrency rules allowed to be executed
//
// Returns:
// Returns:
//   - *ConcurrencyLimiterAspect: Configured concurrency limiter aspect
//     *ConcurrencyLimiterAspect: The concurrency limiting aspect configured
func NewConcurrencyLimiterAspect(max int) *ConcurrencyLimiterAspect {
	return &ConcurrencyLimiterAspect{
		Max: int64(max),
	}
}

// Order returns the execution priority of this aspect. Lower values execute earlier.
// This aspect has order 10, making it one of the first aspects to execute.
//
// Order returns the execution priority of this aspect. The lower the value, the earlier it is executed.
// This section is in the order of 10, making it one of the earliest to be executed.
func (a *ConcurrencyLimiterAspect) Order() int {
	return 10
}

// New creates a new instance of the aspect for each rule engine instance.
// Each instance maintains its own concurrency counter starting from zero.
//
// New: Create a new instance of the facet for each instance of the rule engine.
// Each instance maintains its own concurrency counter, starting from zero.
func (a *ConcurrencyLimiterAspect) New() types.Aspect {
	return &ConcurrencyLimiterAspect{
		Max:          a.Max,
		currentCount: 0,
	}
}

// PointCut determines which nodes this aspect applies to.
// Returns true for all nodes, applying concurrency limiting globally.
//
// PointCut determines which nodes this section is applied to.
// Returns true for all nodes, applying global concurrency limits.
func (a *ConcurrencyLimiterAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// Start is called at the beginning of rule processing. It implements a thread-safe
// concurrency check using atomic operations and compare-and-swap to ensure the
// current execution count doesn't exceed the maximum limit.
//
// Start is called when rule processing begins. It uses atomic operations and comparisons to exchange concurrency checks that achieve thread safety,
// Ensure the current execution count does not exceed the maximum limit.
//
// Algorithm:
// Algorithm:
//  1. Load current count atomically
//  2. Check if limit would be exceeded
//  3. Use CAS to increment if within limit
//  4. Retry if CAS fails due to concurrent modification
//
// Returns:
// Returns:
//   - types.RuleMsg: The original message unchanged
//   - error: ErrConcurrencyLimitReached if limit exceeded, nil otherwise
//     error: Returns ErrConcurrencyLimitReached if the limit is exceeded; otherwise, nil is returned
func (a *ConcurrencyLimiterAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	// Using atomic operations ensures inspection and increases the atomicity of the operation
	for {
		current := atomic.LoadInt64(&a.currentCount)
		if current >= a.Max {
			return msg, types.ErrConcurrencyLimitReached
		}
		// Try to atomically increase the counter, and if successful, exit the loop
		if atomic.CompareAndSwapInt64(&a.currentCount, current, current+1) {
			break
		}
		// If CAS fails, another goroutine changed the counter; retry
	}
	return msg, nil
}

// Completed is called when rule processing is finished. It atomically decrements
// the current execution count, allowing new executions to proceed.
//
// Completed is called when the rule is finished. It atomically reduces the current execution count, allowing new executions to continue.
//
// This method ensures proper cleanup and maintains accurate concurrency tracking.
// This method ensures proper cleanup and maintains accurate concurrency tracking.
func (a *ConcurrencyLimiterAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	atomic.AddInt64(&a.currentCount, -1)
	return msg
}

// incrementCurrent atomically increments the current execution count.
// This is an internal helper method for testing purposes.
//
// incrementCurrent atomically increases the current execution count.
// This is an internal auxiliary method for testing purposes.
func (a *ConcurrencyLimiterAspect) incrementCurrent() {
	atomic.AddInt64(&a.currentCount, 1)
}

// decrementCurrent atomically decrements the current execution count.
// This is an internal helper method for testing purposes.
//
// decrementCurrent atomically reduces the current execution count.
// This is an internal auxiliary method for testing purposes.
func (a *ConcurrencyLimiterAspect) decrementCurrent() {
	atomic.AddInt64(&a.currentCount, -1)
}
