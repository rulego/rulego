package runlog

import (
	"context"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/runlogutil"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/json"
)

const (
	ModuleName = "runlog"
	Priority   = 45
)

// Module runlog 模块：注册运行日志服务与调试数据服务。
// 运行日志的写入路径——引擎 OnRuleChainCompleted 回调经 RunLogService 落库；
// 调试数据（双击节点查看）走内存 + WebSocket 推送，不经本模块的持久化。
type Module struct {
	cfg      *config.Config
	logger   types.Logger
	asyncW   *asyncRunLogWriter
}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger

	// 调试数据每节点缓存上限，配置缺省回退 60
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
	return nil
}

func (m *Module) Start(_ context.Context) error {
	if m.asyncW != nil {
		m.asyncW.Start()
	}
	return nil
}
func (m *Module) Stop(_ context.Context) error {
	if m.asyncW != nil {
		m.asyncW.Stop()
	}
	return nil
}

func (m *Module) initDefaultService(storeProvider store.StoreProvider) {
	s, err := storeProvider.GetRunLogStore()
	if err != nil {
		m.logger.Errorf("init run log store error: %s", err.Error())
	} else {
		// Off 级根本不写日志，无需异步队列，直接走底层 store（实际不会被调用）
		if runlogutil.ParseLevel(m.cfg.RunLogMode) == runlogutil.LevelOff {
			defaultRunLogService = &runLogServiceImpl{
				cfg:    m.cfg,
				logger: m.logger,
				store:  s,
			}
			return
		}
		// 非 Off 级套异步队列，避免回调路径阻塞规则链执行
		m.asyncW = newAsyncRunLogWriter(s, m.logger)
		defaultRunLogService = &runLogServiceImpl{
			cfg:    m.cfg,
			logger: m.logger,
			store:  m.asyncW,
		}
	}
}

var defaultRunLogService services.RunLogService

// DefaultDebugDataStore 调试数据内存存储（节点双击查看 + WebSocket 推送的来源）
var DefaultDebugDataStore = NewDebugDataStore(0)

type runLogServiceImpl struct {
	cfg    *config.Config
	logger types.Logger
	store  store.RunLogStore
}

func (s *runLogServiceImpl) SaveRunLog(username string, ctx types.RuleContext, snapshot types.RuleChainRunSnapshot, level runlogutil.Level, triggerSource string) error {
	snapshot.Id = time.Now().Format("20060102150405000") + "_" + snapshot.Id

	// 成败推导：detail 级 snapshot.Logs 非空，直接扫节点错误；
	// summary 级不收集节点日志，回退到 ctx.GetErr()。
	success := true
	var errorMsg string
	if len(snapshot.Logs) > 0 {
		for _, l := range snapshot.Logs {
			if l.Err != "" {
				success = false
				errorMsg = l.Err
				break
			}
		}
	} else if ctx != nil {
		if err := ctx.GetErr(); err != nil {
			success = false
			errorMsg = err.Error()
		}
	}

	event := model.Event{
		Id:            snapshot.Id,
		ChainId:       snapshot.RuleChain.RuleChain.ID,
		ChainName:     snapshot.RuleChain.RuleChain.Name,
		StartTs:       snapshot.StartTs,
		EndTs:         snapshot.EndTs,
		Success:       success,
		ErrorMsg:      errorMsg,
		TriggerSource: triggerSource,
		Level:         level.String(),
	}
	// 逐节点日志只在 detail 级序列化，summary 级保持零开销
	if level == runlogutil.LevelDetail {
		logsBytes, err := json.Marshal(snapshot.Logs)
		if err != nil {
			s.logger.Errorf("SaveRunLog marshal logs error: %v", err)
			return err
		}
		event.Logs = logsBytes
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
