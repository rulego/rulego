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

func (a *ConcurrencyLimiterAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	atomic.AddInt64(&a.currentCount, -1)
	return msg
}

func (a *ConcurrencyLimiterAspect) incrementCurrent() {
	atomic.AddInt64(&a.currentCount, 1)
}

func (a *ConcurrencyLimiterAspect) decrementCurrent() {
	atomic.AddInt64(&a.currentCount, -1)
}
