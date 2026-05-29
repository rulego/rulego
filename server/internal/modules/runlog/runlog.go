package runlog

import (
	"context"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/json"
)

const (
	ModuleName = "runlog"
	Priority   = 45
)

// Module runlog 业务模块，负责运行日志的收集和查询。
type Module struct {
	cfg    *config.Config
	logger types.Logger
}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger

	// 使用配置的 max_node_log_size 初始化调试数据存储
	maxSize := m.cfg.MaxNodeLogSize
	if maxSize <= 0 {
		maxSize = 60
	}
	DefaultDebugDataStore = NewDebugDataStore(maxSize)

	storeProvider, err := app.GetAs[store.StoreProvider](ctx.Container, "store.provider")
	if err != nil {
		return err
	}
	m.initDefaultService(storeProvider)
	if err := ctx.Container.Register(services.KeyRunLogService, services.RunLogService(defaultRunLogService)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyDebugService, services.DebugService(m)); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

func (m *Module) initDefaultService(storeProvider store.StoreProvider) {
	s, err := storeProvider.GetRunLogStore()
	if err != nil {
		m.logger.Errorf("init run log store error: %s", err.Error())
	} else {
		defaultRunLogService = &runLogServiceImpl{
			cfg:    m.cfg,
			logger: m.logger,
			store:  s,
		}
	}
}

var defaultRunLogService services.RunLogService

// DefaultDebugDataStore 全局调试数据内存存储
var DefaultDebugDataStore = NewDebugDataStore(0)

type runLogServiceImpl struct {
	cfg    *config.Config
	logger types.Logger
	store  store.RunLogStore
}

func (s *runLogServiceImpl) SaveRunLog(username string, ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) error {
	snapshot.Id = time.Now().Format("20060102150405000") + "_" + snapshot.Id

	success := true
	var errorMsg string
	for _, l := range snapshot.Logs {
		if l.Err != "" {
			success = false
			errorMsg = l.Err
			break
		}
	}

	logsJSON, err := json.Marshal(snapshot.Logs)
	if err != nil {
		s.logger.Errorf("SaveRunLog marshal logs error: %v", err)
		return err
	}

	event := model.Event{
		Id:        snapshot.Id,
		ChainId:   snapshot.RuleChain.RuleChain.ID,
		ChainName: snapshot.RuleChain.RuleChain.Name,
		StartTs:   snapshot.StartTs,
		EndTs:     snapshot.EndTs,
		Success:   success,
		ErrorMsg:  errorMsg,
		Logs:      logsJSON,
	}
	return s.store.Save(username, event)
}

func (s *runLogServiceImpl) List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	return s.store.List(username, chainId, startTime, endTime, size, page)
}

func (s *runLogServiceImpl) Get(username, logId string) (model.Event, error) {
	return s.store.Get(username, logId)
}

func (s *runLogServiceImpl) Delete(username, logId string) error {
	return s.store.Delete(username, logId)
}

func (s *runLogServiceImpl) DeleteByChainId(username, chainId string) error {
	return s.store.DeleteByChainId(username, chainId)
}
