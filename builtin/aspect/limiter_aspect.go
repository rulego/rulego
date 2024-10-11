package aspect

import (
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
)

// ConcurrencyLimiterAspect limits the concurrent execution of the rule engine
type ConcurrencyLimiterAspect struct {
	Max          int64 // Maximum number of concurrent executions
	currentCount int64 // Current number of concurrent executions
}

var _ types.StartAspect = (*ConcurrencyLimiterAspect)(nil)
var _ types.CompletedAspect = (*ConcurrencyLimiterAspect)(nil)

func NewConcurrencyLimiterAspect(max int) *ConcurrencyLimiterAspect {
	return &ConcurrencyLimiterAspect{
		Max: int64(max),
	}
}

func (a *ConcurrencyLimiterAspect) Order() int {
	return 10
}

func (a *ConcurrencyLimiterAspect) New() types.Aspect {
	return &ConcurrencyLimiterAspect{
		Max:          a.Max,
		currentCount: 0,
	}
}

func (a *ConcurrencyLimiterAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (a *ConcurrencyLimiterAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	if a.currentCount >= a.Max {
		return msg, types.ErrConcurrencyLimitReached
	}
	a.incrementCurrent()
	return msg, nil
}

func (a *ConcurrencyLimiterAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	a.decrementCurrent()
	return msg
}

func (a *ConcurrencyLimiterAspect) incrementCurrent() {
	atomic.AddInt64(&a.currentCount, 1)
}

func (a *ConcurrencyLimiterAspect) decrementCurrent() {
	newVale := atomic.AddInt64(&a.currentCount, -1)
	if newVale < 0 {
		atomic.StoreInt64(&a.currentCount, newVale)
	}
}
