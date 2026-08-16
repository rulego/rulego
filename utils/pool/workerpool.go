/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package pool provides high-performance worker pool implementations for concurrent task execution.
// It includes optimized worker pool that manages goroutines efficiently to reduce allocation overhead.
//
// Package pool 提供用于并发任务执行的高性能工作池实现。
// 它包括优化的工作池，有效管理协程以减少分配开销。
//
// Note: This file is inspired by:
// Valyala, A. (2023) workerpool.go (Version 1.48.0)
// [Source code]. https://github.com/valyala/fasthttp/blob/master/workerpool.go
// 1.Change the Serve(c net.Conn) method to Submit(fn func()) error method
// 2.Shard the ready list per CPU to remove the single global lock contention
package pool

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// maxShardCount limits the number of shards to bound per-shard cleanup cost.
const maxShardCount = 32

// WorkerPool serves incoming functions using a pool of workers in FILO order.
// The most recently stopped worker will serve the next incoming function.
// This scheme keeps CPU caches hot for better performance.
//
// WorkerPool 使用工作池以 FILO 顺序处理传入函数。
// 最近停止的工作者将处理下一个传入函数。
// 这种方案保持 CPU 缓存热度以获得更好的性能。
//
// Internally the pool is split into per-CPU shards, each with its own lock
// and ready list, so concurrent Submit/release calls on multi-core machines
// do not contend on a single mutex.
//
// 内部按 CPU 数分片，每片独立持锁与维护就绪列表，
// 避免多核下并发 Submit/release 争抢同一把锁。
type WorkerPool struct {
	// MaxWorkersCount is the maximum number of workers that can be created.
	// If set to 0, the pool will create workers without limit (not recommended).
	// MaxWorkersCount 是可以创建的最大工作者数量。
	// 如果设置为 0，池将无限制地创建工作者（不推荐）。
	MaxWorkersCount int

	// MaxIdleWorkerDuration is the maximum duration a worker can remain idle
	// before being cleaned up. Workers idle longer than this duration will be terminated.
	// Default is 10 seconds if not specified.
	// MaxIdleWorkerDuration 是工作者在被清理之前可以保持空闲的最大持续时间。
	// 空闲时间超过此持续时间的工作者将被终止。
	// 如果未指定，默认为 10 秒。
	MaxIdleWorkerDuration time.Duration

	// stopCh is used to signal the cleanup goroutine to stop
	// stopCh 用于向清理协程发送停止信号
	stopCh chan struct{}

	// workerChanPool pools worker channel objects to reduce allocations
	// workerChanPool 池化工作者通道对象以减少分配
	workerChanPool sync.Pool

	// shards are per-CPU sub-pools, each holding its own lock and ready list
	// shards 是按 CPU 划分的子池，各自持有锁与就绪列表
	shards []*workerShard

	// next is the round-robin counter for shard selection
	// next 是分片轮询计数器
	next uint64

	// startOnce ensures the pool is started only once
	// startOnce 确保池只启动一次
	startOnce sync.Once
}

// workerShard is a sub-pool holding its own lock, ready list and worker quota.
// workerShard 是持有独立锁、就绪列表与工作者配额的子池。
type workerShard struct {
	lock sync.Mutex
	// ready maintains a list of available workers in FILO order
	// ready 以 FILO 顺序维护可用工作者列表
	ready []*workerChan
	// workersCount tracks the current number of active workers in this shard
	// workersCount 跟踪本分片当前活动工作者数量
	workersCount int
	// quota is the maximum number of workers this shard may create
	// quota 是本分片可创建的最大工作者数量
	quota int
	// mustStop indicates whether the shard should stop accepting new tasks
	// mustStop 指示分片是否停止接受新任务
	mustStop bool
}

// workerChan represents a worker with its communication channel and metadata.
// workerChan 表示具有通信通道和元数据的工作者。
type workerChan struct {
	// lastUseTime records when the worker was last used for cleanup purposes
	// lastUseTime 记录工作者最后使用时间用于清理
	lastUseTime time.Time

	// ch is the communication channel for sending functions to the worker
	// ch 是向工作者发送函数的通道
	ch chan func()

	// shard is the shard this worker currently belongs to
	// shard 是工作者当前所属的分片
	shard *workerShard
}

// Start initializes and starts the worker pool.
// It creates the cleanup goroutine and sets up the worker channel pool.
// This method is thread-safe and can be called multiple times safely.
//
// Start 初始化并启动工作池。
// 它创建清理协程并设置工作者通道池。
// 此方法是线程安全的，可以被多次安全调用。
func (wp *WorkerPool) Start() {
	if wp.stopCh != nil {
		return
	}
	wp.startOnce.Do(func() {
		// Create stop channel for cleanup coordination
		// 创建用于清理协调的停止通道
		wp.stopCh = make(chan struct{})
		stopCh := wp.stopCh

		// Initialize worker channel pool with factory function
		// 使用工厂函数初始化工作者通道池
		wp.workerChanPool.New = func() interface{} {
			return &workerChan{
				ch: make(chan func(), workerChanCap),
			}
		}

		// Build per-CPU shards and split the worker quota evenly.
		// The remainder is given to the first shards so the total equals MaxWorkersCount.
		// 按 CPU 数构建分片并均分工作者配额，余数分给前几个分片，总数与 MaxWorkersCount 一致。
		n := runtime.GOMAXPROCS(0)
		if n < 1 {
			n = 1
		}
		if n > maxShardCount {
			n = maxShardCount
		}
		base := wp.MaxWorkersCount / n
		extra := wp.MaxWorkersCount % n
		wp.shards = make([]*workerShard, n)
		for i := range wp.shards {
			quota := base
			if i < extra {
				quota++
			}
			wp.shards[i] = &workerShard{quota: quota}
		}

		// Start background cleanup goroutine
		// 启动后台清理协程
		go func() {
			var scratch []*workerChan
			for {
				// Clean up idle workers
				// 清理空闲工作者
				wp.clean(&scratch)
				select {
				case <-stopCh:
					// Pool has been stopped, exit cleanup loop
					// 池已停止，退出清理循环
					return
				default:
					// Wait for next cleanup cycle
					// 等待下一个清理周期
					time.Sleep(wp.getMaxIdleWorkerDuration())
				}
			}
		}()
	})
}

// Stop gracefully shuts down the worker pool.
// It stops accepting new tasks and signals all idle workers to terminate.
// Busy workers will complete their current tasks before terminating.
//
// Stop 优雅地关闭工作池。
// 它停止接受新任务并向所有空闲工作者发送终止信号。
// 忙碌的工作者将完成当前任务后终止。
//
// Note: This method does not wait for busy workers to complete.
// 注意：此方法不会等待忙碌工作者完成。
func (wp *WorkerPool) Stop() {
	if wp.stopCh == nil {
		return
	}

	// Signal cleanup goroutine to stop
	// 通知清理协程停止
	close(wp.stopCh)
	wp.stopCh = nil

	// Stop all the workers waiting for incoming connections.
	// Do not wait for busy workers - they will stop after
	// serving the connection and noticing shard mustStop = true.
	// 停止所有等待任务的工作者，忙碌工作者完成当前任务后退出。
	for _, sh := range wp.shards {
		sh.lock.Lock()
		ready := sh.ready
		for i := range ready {
			// Send termination signal to each idle worker
			// 向每个空闲工作者发送终止信号
			ready[i].ch <- nil
			ready[i] = nil
		}
		sh.ready = ready[:0]
		sh.mustStop = true
		sh.lock.Unlock()
	}
}

// Release is an alias for Stop() provided for compatibility.
// Release 是为兼容性提供的 Stop() 的别名。
func (wp *WorkerPool) Release() {
	wp.Stop()
}

// getMaxIdleWorkerDuration returns the configured idle duration or a default.
// getMaxIdleWorkerDuration 返回配置的空闲时长或默认值 10 秒。
func (wp *WorkerPool) getMaxIdleWorkerDuration() time.Duration {
	if wp.MaxIdleWorkerDuration <= 0 {
		return 10 * time.Second
	}
	return wp.MaxIdleWorkerDuration
}

// clean removes idle workers that have exceeded the maximum idle duration.
// It scans every shard and uses binary search within each shard's ready list.
//
// clean 清理超过最大空闲时长的空闲工作者。
// 它遍历每个分片，并在分片就绪列表内使用二分查找。
func (wp *WorkerPool) clean(scratch *[]*workerChan) {
	maxIdleWorkerDuration := wp.getMaxIdleWorkerDuration()

	// Clean least recently used workers if they didn't serve connections
	// for more than maxIdleWorkerDuration.
	// 清理最近最少使用且超过空闲时长的工作者。
	criticalTime := time.Now().Add(-maxIdleWorkerDuration)

	for _, sh := range wp.shards {
		sh.lock.Lock()
		ready := sh.ready
		n := len(ready)

		// Use binary-search algorithm to find out the index of the least recently worker which can be cleaned up.
		// 使用二分搜索算法找出可以清理的最近最少使用工作者的索引。
		l, r, mid := 0, n-1, 0
		for l <= r {
			mid = (l + r) / 2
			if criticalTime.After(sh.ready[mid].lastUseTime) {
				l = mid + 1
			} else {
				r = mid - 1
			}
		}
		i := r
		if i == -1 {
			// No workers to clean up in this shard
			// 该分片没有需要清理的工作者
			sh.lock.Unlock()
			continue
		}

		// Move workers to be cleaned to scratch slice
		// 将要清理的工作者移到临时切片
		*scratch = append((*scratch)[:0], ready[:i+1]...)
		m := copy(ready, ready[i+1:])
		for i = m; i < n; i++ {
			ready[i] = nil
		}
		sh.ready = ready[:m]
		sh.lock.Unlock()

		// Notify obsolete workers to stop.
		// This notification must be outside the shard lock, since ch.ch
		// may be blocking and may consume a lot of time if many workers
		// are located on non-local CPUs.
		// 在锁外通知过时工作者停止，避免通道阻塞持锁过久。
		tmp := *scratch
		for i := range tmp {
			tmp[i].ch <- nil
			tmp[i] = nil
		}
	}
}

// Submit submits a function for execution by the worker pool.
// It returns an error if no idle workers are available and the maximum
// worker count has been reached.
//
// Submit 提交函数供工作池执行。
// 如果没有空闲工作者可用且已达到最大工作者数量，它返回错误。
func (wp *WorkerPool) Submit(fn func()) error {
	shards := wp.shards
	n := len(shards)
	start := int((atomic.AddUint64(&wp.next, 1) - 1) % uint64(n))
	// 从轮询选中的分片开始依次尝试，避免命中无配额/无空闲的分片时误报失败
	for i := 0; i < n; i++ {
		if ch := wp.getCh(shards[(start+i)%n]); ch != nil {
			ch.ch <- fn
			return nil
		}
	}
	return errors.New("no idle workers")
}

// workerChanCap determines the capacity of worker channels based on GOMAXPROCS.
// workerChanCap 基于 GOMAXPROCS 确定工作者通道容量。
var workerChanCap = func() int {
	// Use blocking workerChan if GOMAXPROCS=1.
	// This immediately switches Serve to WorkerFunc, which results
	// in higher performance (under go1.5 at least).
	// 如果 GOMAXPROCS=1 使用阻塞 workerChan。
	if runtime.GOMAXPROCS(0) == 1 {
		return 0
	}

	// Use non-blocking workerChan if GOMAXPROCS>1,
	// since otherwise the Serve caller (Acceptor) may lag accepting
	// new connections if WorkerFunc is CPU-bound.
	// 如果 GOMAXPROCS>1 使用非阻塞 workerChan。
	return 1
}()

// getCh attempts to acquire a worker channel from the given shard.
// It either reuses an idle worker or creates a new one if possible.
//
// getCh 尝试从指定分片获取工作者通道。
// 它要么重用空闲工作者，要么在可能的情况下创建新工作者。
func (wp *WorkerPool) getCh(sh *workerShard) *workerChan {
	var ch *workerChan
	createWorker := false

	sh.lock.Lock()
	ready := sh.ready
	n := len(ready) - 1
	if n < 0 {
		// No idle workers available, check if we can create a new one
		// 没有空闲工作者可用，检查是否可以创建新工作者
		if sh.workersCount < sh.quota {
			createWorker = true
			sh.workersCount++
		}
	} else {
		// Reuse the most recently used worker (FILO order)
		// 重用最近使用的工作者（FILO 顺序）
		ch = ready[n]
		ready[n] = nil
		sh.ready = ready[:n]
	}
	sh.lock.Unlock()

	if ch == nil {
		if !createWorker {
			// Cannot create new worker and no idle workers available
			// 无法创建新工作者且没有空闲工作者可用
			return nil
		}
		// Create new worker channel from pool
		// 从池中创建新的工作者通道
		vch := wp.workerChanPool.Get()
		ch = vch.(*workerChan)
		ch.shard = sh
		go func() {
			// Start worker goroutine
			// 启动工作者协程
			wp.workerFunc(ch)
			// Return worker channel to pool when done
			// 完成时将工作者通道返回到池
			wp.workerChanPool.Put(vch)
		}()
	}
	return ch
}

// release returns a worker channel to its shard's ready list.
// release 将工作者通道归还到其所属分片的就绪列表。
func (wp *WorkerPool) release(ch *workerChan) bool {
	// Update last use time for cleanup purposes
	// 更新最后使用时间用于清理
	ch.lastUseTime = time.Now()

	sh := ch.shard
	sh.lock.Lock()
	if sh.mustStop {
		// Shard is stopping, don't reuse this worker
		// 分片正在停止，不要重用此工作者
		sh.lock.Unlock()
		return false
	}
	// Add worker back to ready list (will be used in FILO order)
	// 将工作者添加回就绪列表（将以 FILO 顺序使用）
	sh.ready = append(sh.ready, ch)
	sh.lock.Unlock()
	return true
}

// workerFunc is the main worker goroutine function.
// It continuously processes functions sent through the worker channel
// until a termination signal is received.
//
// workerFunc 是主要的工作者协程函数。
// 它持续处理通过工作者通道发送的函数，直到收到终止信号。
func (wp *WorkerPool) workerFunc(ch *workerChan) {
	var fn func()
	for fn = range ch.ch {
		if fn == nil {
			// Termination signal received
			// 收到终止信号
			break
		}

		// Execute the user function
		// 执行用户函数
		fn()
		fn = nil

		// Try to release the worker back to the pool
		// 尝试将工作者释放回池
		if !wp.release(ch) {
			// Pool is stopping, exit worker goroutine
			// 池正在停止，退出工作者协程
			break
		}
	}

	// Decrement worker count when exiting
	// 退出时减少工作者计数
	sh := ch.shard
	sh.lock.Lock()
	sh.workersCount--
	sh.lock.Unlock()
}
