// Package audit 管理面操作审计模块：注册审计服务。
// Record 仅非阻塞入队，单 worker 串行落盘；审计不可静默丢，
// 队列满逐条 Error 并计数，Stop 时同步刷完剩余队列。
package audit

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

const (
	ModuleName = "audit"
	Priority   = 46
	queueSize  = 256
)

// 字段长度上限。Actor/IP 来自请求输入（登录失败会记尝试用户名与 XFF 头），
// 超长会破坏审计文件的体积上限；Detail 与运行日志消息摘要同限。
const (
	maxActorLen  = 128
	maxUALen     = 128
	maxIPLen     = 64
	maxActionLen = 64
	maxOpLen     = 32
	maxTargetLen = 256
	maxDetailLen = 512
)

// truncateRune 按字符截断，避免把多字节字符腰斩。
func truncateRune(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func sanitizeEvent(e model.AuditEvent) model.AuditEvent {
	e.Actor = truncateRune(e.Actor, maxActorLen)
	e.ActorType = truncateRune(e.ActorType, 16)
	e.IP = truncateRune(e.IP, maxIPLen)
	e.UA = truncateRune(e.UA, maxUALen)
	e.Action = truncateRune(e.Action, maxActionLen)
	e.Op = truncateRune(e.Op, maxOpLen)
	e.Target = truncateRune(e.Target, maxTargetLen)
	e.Result = truncateRune(e.Result, 16)
	e.Detail = truncateRune(e.Detail, maxDetailLen)
	return e
}

// Module audit 模块：注册管理面操作审计服务。
// 存储经 StoreProvider 的可选扩展接口 AuditLogStoreProvider 获取，
// 宿主未实现或 audit_enable=false 时服务降级为空操作（Record 静默跳过）。
type Module struct {
	cfg    *config.Config
	logger types.Logger
	svc    *auditService
}

func New() *Module { return &Module{} }

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger
	m.svc = &auditService{}
	if m.cfg.AuditEnable {
		if provider, err := app.GetAs[store.StoreProvider](ctx.Container, "store.provider"); err == nil {
			if sp, ok := provider.(store.AuditLogStoreProvider); ok {
				if s, err := sp.GetAuditLogStore(); err == nil {
					m.svc.writer = newAsyncAuditWriter(s, ctx.Logger)
				} else {
					ctx.Logger.Warnf("audit log store unavailable, audit disabled: %s", err.Error())
				}
			}
		}
	}
	if err := ctx.Container.Register(services.KeyAuditService, services.AuditService(m.svc)); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error {
	if m.svc.writer != nil {
		m.svc.writer.Start()
	}
	return nil
}

func (m *Module) Stop(_ context.Context) error {
	if m.svc.writer != nil {
		m.svc.writer.Stop()
	}
	return nil
}

// auditService Record 非阻塞，失败只影响审计不影响业务；List 穿透到存储。
type auditService struct {
	writer *asyncAuditWriter
}

func (s *auditService) Record(event model.AuditEvent) {
	if s.writer == nil {
		return
	}
	if event.Ts <= 0 {
		event.Ts = time.Now().UnixMilli()
	}
	s.writer.enqueue(sanitizeEvent(event))
}

func (s *auditService) List(filter model.AuditFilter) ([]model.AuditEvent, int64, error) {
	if s.writer == nil {
		return []model.AuditEvent{}, 0, nil
	}
	return s.writer.store.List(filter)
}

// asyncAuditWriter 单 worker 串行落盘。队列满丢弃并逐条 Error——
// 审计序列的空洞必须显式暴露，不能采样静默。
type asyncAuditWriter struct {
	store   store.AuditLogStore
	queue   chan model.AuditEvent
	stopCh  chan struct{}
	done    chan struct{}
	started int64
	stopped int64
	dropped int64
	logger  types.Logger
}

func newAsyncAuditWriter(s store.AuditLogStore, logger types.Logger) *asyncAuditWriter {
	if logger == nil {
		logger = types.DefaultLogger()
	}
	return &asyncAuditWriter{
		store:  s,
		queue:  make(chan model.AuditEvent, queueSize),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
		logger: logger,
	}
}

func (w *asyncAuditWriter) Start() {
	if !atomic.CompareAndSwapInt64(&w.started, 0, 1) {
		return
	}
	go w.run()
}

func (w *asyncAuditWriter) enqueue(event model.AuditEvent) {
	if atomic.LoadInt64(&w.stopped) == 1 {
		atomic.AddInt64(&w.dropped, 1)
		return
	}
	select {
	case w.queue <- event:
	default:
		n := atomic.AddInt64(&w.dropped, 1)
		w.logger.Errorf("audit queue full, event dropped (total %d): actor=%s action=%s target=%s",
			n, event.Actor, event.Action, event.Target)
	}
}

func (w *asyncAuditWriter) run() {
	defer close(w.done)
	for {
		select {
		case e := <-w.queue:
			w.save(e)
		case <-w.stopCh:
			return
		}
	}
}

func (w *asyncAuditWriter) save(e model.AuditEvent) {
	if err := w.store.Save(e); err != nil {
		w.logger.Errorf("audit save error: %s", err.Error())
	}
}

// Stop 停止接收并把队列剩余事件同步落盘，审计不随停机丢事件。
func (w *asyncAuditWriter) Stop() {
	atomic.StoreInt64(&w.stopped, 1)
	if atomic.LoadInt64(&w.started) == 1 {
		close(w.stopCh)
		<-w.done
	}
	for {
		select {
		case e := <-w.queue:
			w.save(e)
		default:
			return
		}
	}
}

// Dropped 累计丢弃数，供监控暴露
func (w *asyncAuditWriter) Dropped() int64 {
	return atomic.LoadInt64(&w.dropped)
}
