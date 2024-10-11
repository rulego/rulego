package aspect

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/metrics"
)

// MetricsAspect 实现了统计规则引擎指标的功能
type MetricsAspect struct {
	metrics *metrics.EngineMetrics
}

var _ types.StartAspect = (*MetricsAspect)(nil)
var _ types.EndAspect = (*MetricsAspect)(nil)
var _ types.CompletedAspect = (*MetricsAspect)(nil)

func NewMetricsAspect(m *metrics.EngineMetrics) *MetricsAspect {
	if m == nil {
		m = metrics.NewEngineMetrics()
	}
	return &MetricsAspect{
		metrics: m,
	}
}

func (a *MetricsAspect) Order() int {
	return 20
}

func (a *MetricsAspect) New() types.Aspect {
	if a.metrics == nil {
		a.metrics = metrics.NewEngineMetrics()
	}
	a.metrics.Reset()
	return &MetricsAspect{
		metrics: a.metrics,
	}
}

func (a *MetricsAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (a *MetricsAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	a.metrics.IncrementCurrent()
	a.metrics.IncrementTotal()
	return msg, nil
}

func (a *MetricsAspect) End(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	if err != nil {
		a.metrics.IncrementFailed()
	} else {
		a.metrics.IncrementSuccess()
	}
	return msg
}

func (a *MetricsAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	a.metrics.DecrementCurrent()
	return msg
}

// GetMetrics 返回当前的指标
func (a *MetricsAspect) GetMetrics() *metrics.EngineMetrics {
	return a.metrics
}
