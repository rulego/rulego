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
func (s *EventService) SaveRunLog(username string, ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) error {
	return s.EventDao.SaveRunLog(username, ctx, snapshot)
}

func (s *EventService) Delete(username, chainId, id string) error {
	return s.EventDao.Delete(username, chainId, id)
}
func (s *EventService) DeleteByChainId(username, chainId string) error {
	return s.EventDao.DeleteByChainId(username, chainId)
}

func (s *EventService) List(username, chainId string, current, size int) ([]types.RuleChainRunSnapshot, int, error) {
	return s.EventDao.List(username, chainId, current, size)
}

func (s *EventService) Get(username, chainId, snapshotId string) (types.RuleChainRunSnapshot, error) {
	return s.EventDao.Get(username, chainId, snapshotId)
}
