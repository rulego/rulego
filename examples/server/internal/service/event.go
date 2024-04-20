package service

import (
	"examples/server/config"
	"examples/server/internal/dao"
	"github.com/rulego/rulego/api/types"
)

var EventServiceImpl *EventService

type EventService struct {
	EventDao *dao.EventDao
	config   config.Config
}

func NewEventService(config config.Config) (*EventService, error) {
	if eventDao, err := dao.NewEventDao(config); err != nil {
		return nil, err
	} else {
		return &EventService{
			EventDao: eventDao,
			config:   config,
		}, nil
	}
}

// SaveRunLog 保存工作流运行日志快照
func (s *EventService) SaveRunLog(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) error {
	return s.EventDao.SaveRunLog(ctx, snapshot)
}

func (s *EventService) Delete(chainId string) error {
	return s.EventDao.Delete(chainId)
}
