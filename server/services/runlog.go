package services

import (
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/model"
)

// RunLogService runs the log service interface
type RunLogService interface {
	SaveRunLog(username string, ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) error
	List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error)
	Get(username, logId string) (model.Event, error)
	Delete(username, logId string) error
	DeleteByChainId(username, chainId string) error
}
