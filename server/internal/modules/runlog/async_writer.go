package runlog

import (
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

// 固定容量，未做成配置项
const asyncQueueSize = 10000

type writeTask struct {
	username string
	event    model.Event
}

// asyncRunLogWriter 在 store 之上套一层异步队列：Save 仅入队，单 worker 串行落盘；
// 读操作直接穿透到底层 store。运行记录是可观测数据，吞吐优先于可靠交付。
type asyncRunLogWriter struct {
	store   store.RunLogStore
	queue   chan writeTask
	stopCh  chan struct{}
	done    chan struct{}
	stopped int64 // 原子置位后 Save 直接丢弃，避免 Stop 后再往已关闭通道发送
	dropped int64
	logger  types.Logger
}

func newAsyncRunLogWriter(s store.RunLogStore, logger types.Logger) *asyncRunLogWriter {
	return &asyncRunLogWriter{
		store:  s,
		queue:  make(chan writeTask, asyncQueueSize),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
		logger: logger,
	}
}

func (w *asyncRunLogWriter) Start() {
	go w.run()
}

// Save 非阻塞入队。队列满或已停止则丢弃并计数——绝不阻塞调用方（规则链回调路径）。
func (w *asyncRunLogWriter) Save(username string, event model.Event) error {
	if atomic.LoadInt64(&w.stopped) == 1 {
		return nil
	}
	select {
	case w.queue <- writeTask{username: username, event: event}:
	default:
		n := atomic.AddInt64(&w.dropped, 1)
		// 采样告警，避免持续打满日志
		if n%100 == 1 {
			w.logger.Warnf("run log queue full, dropped %d events", n)
		}
	}
	return nil
}

func (w *asyncRunLogWriter) run() {
	defer close(w.done)
	for {
		select {
		case t := <-w.queue:
			if err := w.store.Save(t.username, t.event); err != nil {
				w.logger.Errorf("async save run log error: %v", err)
			}
		case <-w.stopCh:
			return
		}
	}
}

// Stop 标记停止并等待 worker 退出。队列中未落盘的任务直接丢弃，
// 运行记录是可观测数据，关停时丢一部分可接受，换取干净的退出语义。
func (w *asyncRunLogWriter) Stop() {
	atomic.StoreInt64(&w.stopped, 1)
	close(w.stopCh)
	<-w.done
}

// Dropped 累计丢弃数，供监控暴露
func (w *asyncRunLogWriter) Dropped() int64 {
	return atomic.LoadInt64(&w.dropped)
}

// 读操作直接穿透底层 store

func (w *asyncRunLogWriter) List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	return w.store.List(username, chainId, startTime, endTime, size, page)
}
func (w *asyncRunLogWriter) Get(username, logId string) (model.Event, error) {
	return w.store.Get(username, logId)
}
func (w *asyncRunLogWriter) Delete(username, logId string) error {
	return w.store.Delete(username, logId)
}
func (w *asyncRunLogWriter) DeleteByChainId(username, chainId string) error {
	return w.store.DeleteByChainId(username, chainId)
}
