package dao

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/constants"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"os"
	"path"
	"path/filepath"
	"time"
)

type EventDao struct {
	*FileStorage
	config config.Config
}

func NewEventDao(config config.Config) (*EventDao, error) {
	return &EventDao{
		config: config,
	}, nil
}

// SaveRunLog 保存工作流运行日志快照
func (s *EventDao) SaveRunLog(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) error {
	var paths = []string{s.config.DataDir, constants.WorkflowsDir}
	chainId := ctx.RuleChain().GetNodeId().Id
	paths = append(paths, constants.WorkflowsRunDir, chainId)
	pathStr := path.Join(paths...)
	//创建文件夹
	_ = fs.CreateDirs(pathStr)
	//保存到文件
	if byteV, err := json.Marshal(snapshot); err != nil {
		logger.Logger.Printf("dao/EventDao:SaveRunLog marshal error", err)
		return err
	} else {
		v, _ := json.Format(byteV)
		//保存规则链到文件
		if err = fs.SaveFile(filepath.Join(pathStr, time.Now().Format("20060102150405000")+"_"+snapshot.Id), v); err != nil {
			logger.Logger.Printf("dao/EventDao:SaveRunLog save file error", err)
			return err
		}
	}
	return nil
}

func (s *EventDao) Delete(chainId string) error {
	var paths = []string{s.config.DataDir, constants.WorkflowsDir}
	paths = append(paths, constants.WorkflowsRunDir, chainId)
	pathStr := path.Join(paths...)
	return os.RemoveAll(pathStr)
}
