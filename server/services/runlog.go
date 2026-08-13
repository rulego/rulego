package services

import (
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/internal/runlogutil"
	"github.com/rulego/rulego/server/model"
)

// RunLogService 运行日志服务。SaveRunLog 由引擎 OnRuleChainCompleted 回调调用，
// 其余为同步查询/删除，供 REST API 直接使用。
type RunLogService interface {
	SaveRunLog(username string, ctx types.RuleContext, snapshot types.RuleChainRunSnapshot, level runlogutil.Level, triggerSource string) error
	List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error)
	Get(username, logId string) (model.Event, error)
	Delete(username, logId string) error
	DeleteByChainId(username, chainId string) error
}
