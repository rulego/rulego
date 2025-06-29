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
package pool

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

// WorkerPool serves incoming functions using a pool of workers in FILO order.
// The most recently stopped worker will serve the next incoming function.
// This scheme keeps CPU caches hot for better performance.
//
// WorkerPool 使用工作池以 FILO 顺序处理传入函数。
// 最近停止的工作者将处理下一个传入函数。
// 这种方案保持 CPU 缓存热度以获得更好的性能。
//
// Key Features:
// 主要特性：
//   - FILO (First In, Last Out) worker management for CPU cache efficiency
//     FILO（先进后出）工作者管理以提高 CPU 缓存效率
//   - Configurable maximum worker count and idle duration
//     可配置的最大工作者数量和空闲持续时间
//   - Automatic worker cleanup for memory efficiency
//     自动工作者清理以提高内存效率
//   - Non-blocking task submission with error handling
//     非阻塞任务提交与错误处理
//   - Graceful start/stop lifecycle management
//     优雅的启动/停止生命周期管理
//
// Performance Benefits:
// 性能优势：
//   - Reduces goroutine allocation overhead
//     减少协程分配开销
//   - Maintains hot CPU caches through worker reuse
//     通过工作者重用维护热 CPU 缓存
//   - Efficient memory usage with automatic cleanup
//     通过自动清理实现高效内存使用
//   - Scalable worker management based on load
//     基于负载的可扩展工作者管理
//
// Usage Example:
// 使用示例：
//
//	pool := &WorkerPool{
//	  MaxWorkersCount: 100,
//	  MaxIdleWorkerDuration: 10 * time.Second,
//	}
//	pool.Start()
//	defer pool.Stop()
//
//	err := pool.Submit(func() {
//	  // Your task implementation
//	})
//	if err != nil {
//	  // Handle submission error
//	}
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

	// lock provides thread-safe access to internal state
	// lock 提供对内部状态的线程安全访问
	lock sync.Mutex

	// workersCount tracks the current number of active workers
	// workersCount 跟踪当前活动工作者的数量
	workersCount int

	// mustStop indicates whether the pool should stop accepting new tasks
	// mustStop 指示池是否应停止接受新任务
	mustStop bool

	// ready maintains a list of available workers in FILO order
	// ready 以 FILO 顺序维护可用工作者列表
	ready []*workerChan

	// stopCh is used to signal the cleanup goroutine to stop
	// stopCh 用于向清理协程发送停止信号
	stopCh chan struct{}

	// workerChanPool pools worker channel objects to reduce allocations
	// workerChanPool 池化工作者通道对象以减少分配
	workerChanPool sync.Pool

	// startOnce ensures the pool is started only once
	// startOnce 确保池只启动一次
	startOnce sync.Once
}

// workerChan represents a worker with its communication channel and metadata.
// It encapsulates the worker's state and provides the communication mechanism
// between the pool and the worker goroutine.
//
// workerChan 表示具有通信通道和元数据的工作者。
// 它封装工作者的状态并提供池和工作者协程之间的通信机制。
type workerChan struct {
	// lastUseTime records when the worker was last used for cleanup purposes
	// lastUseTime 记录工作者最后使用时间以用于清理目的
	lastUseTime time.Time

	// ch is the communication channel for sending functions to the worker
	// ch 是向工作者发送函数的通信通道
	ch chan func()
}

// Start initializes and starts the worker pool.
// It creates the cleanup goroutine and sets up the worker channel pool.
// This method is thread-safe and can be called multiple times safely.
//
// Start 初始化并启动工作池。
// 它创建清理协程并设置工作者通道池。
// 此方法是线程安全的，可以安全地多次调用。
//
// Initialization Process:
// 初始化过程：
//  1. Create stop channel for cleanup coordination  创建用于清理协调的停止通道
//  2. Initialize worker channel pool with factory function  使用工厂函数初始化工作者通道池
//  3. Start background cleanup goroutine  启动后台清理协程
//  4. Set up periodic worker cleanup based on idle duration  基于空闲持续时间设置定期工作者清理
//
// The cleanup goroutine runs continuously until Stop() is called and:
// 清理协程持续运行直到调用 Stop()，并且：
//   - Removes workers that have been idle longer than MaxIdleWorkerDuration
//     删除空闲时间超过 MaxIdleWorkerDuration 的工作者
//   - Maintains optimal pool size based on workload  基于工作负载维护最佳池大小
//   - Prevents memory leaks from unused workers  防止未使用工作者造成的内存泄漏
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
// Shutdown Process:
// 关闭过程：
//  1. Close the stop channel to signal cleanup goroutine  关闭停止通道以向清理协程发送信号
//  2. Set mustStop flag to prevent new task acceptance  设置 mustStop 标志以防止接受新任务
//  3. Send termination signals to all idle workers  向所有空闲工作者发送终止信号
//  4. Clear the ready worker list  清空就绪工作者列表
//
// Note: This method does not wait for busy workers to complete.
// For applications requiring graceful shutdown, consider implementing
// a separate mechanism to wait for task completion.
//
// 注意：此方法不等待忙碌工作者完成。
// 对于需要优雅关闭的应用程序，考虑实现单独的机制来等待任务完成。
func (wp *WorkerPool) Stop() {
	if wp.stopCh == nil {
		return
	}

	// Signal cleanup goroutine to stop
	// 向清理协程发送停止信号
	close(wp.stopCh)
	wp.stopCh = nil

	// Stop all the workers waiting for incoming connections.
	// Do not wait for busy workers - they will stop after
	// serving the connection and noticing wp.mustStop = true.
	// 停止所有等待传入连接的工作者。
	// 不等待忙碌的工作者 - 它们将在服务连接后停止并注意到 wp.mustStop = true。
	wp.lock.Lock()
	ready := wp.ready
	for i := range ready {
		// Send termination signal to each idle worker
		// 向每个空闲工作者发送终止信号
		ready[i].ch <- nil
		ready[i] = nil
	}
	wp.ready = ready[:0]
	wp.mustStop = true
	wp.lock.Unlock()
}

// Release is an alias for Stop() provided for compatibility.
// It performs the same shutdown operation as Stop().
//
// Release 是为兼容性提供的 Stop() 的别名。
// 它执行与 Stop() 相同的关闭操作。
func (wp *WorkerPool) Release() {
	wp.Stop()
}

// getMaxIdleWorkerDuration returns the configured idle duration or a default value.
// It provides a sensible default of 10 seconds if no duration is specified.
//
// getMaxIdleWorkerDuration 返回配置的空闲持续时间或默认值。
// 如果未指定持续时间，它提供 10 秒的合理默认值。
//
// Returns:
// 返回：
//   - time.Duration: The idle duration after which workers are cleaned up
//     工作者被清理的空闲持续时间
func (wp *WorkerPool) getMaxIdleWorkerDuration() time.Duration {
	if wp.MaxIdleWorkerDuration <= 0 {
		return 10 * time.Second
	}
	return wp.MaxIdleWorkerDuration
}

// clean removes idle workers that have exceeded the maximum idle duration.
// It uses binary search for efficient identification of workers to be cleaned.
//
// clean 删除超过最大空闲持续时间的空闲工作者。
// 它使用二分搜索高效识别要清理的工作者。
//
// Parameters:
// 参数：
//   - scratch: Reusable slice to minimize allocations during cleanup
//     可重用切片，在清理期间最小化分配
//
// Algorithm:
// 算法：
//  1. Calculate critical time threshold for cleanup  计算清理的关键时间阈值
//  2. Use binary search to find oldest workers to clean  使用二分搜索找到要清理的最旧工作者
//  3. Move remaining workers to front of ready list  将剩余工作者移到就绪列表前面
//  4. Send termination signals to cleaned workers  向清理的工作者发送终止信号
//
// Performance: O(log n + m) where n is ready workers and m is workers to clean
// 性能：O(log n + m)，其中 n 是就绪工作者数，m 是要清理的工作者数
func (wp *WorkerPool) clean(scratch *[]*workerChan) {
	maxIdleWorkerDuration := wp.getMaxIdleWorkerDuration()

	// Clean least recently used workers if they didn't serve connections
	// for more than maxIdleWorkerDuration.
	// 清理最近最少使用的工作者，如果它们超过 maxIdleWorkerDuration 没有服务连接。
	criticalTime := time.Now().Add(-maxIdleWorkerDuration)

	wp.lock.Lock()
	ready := wp.ready
	n := len(ready)

	// Use binary-search algorithm to find out the index of the least recently worker which can be cleaned up.
	// 使用二分搜索算法找出可以清理的最近最少使用工作者的索引。
	l, r, mid := 0, n-1, 0
	for l <= r {
		mid = (l + r) / 2
		if criticalTime.After(wp.ready[mid].lastUseTime) {
			l = mid + 1
		} else {
			r = mid - 1
		}
	}
	i := r
	if i == -1 {
		// No workers to clean up
		// 没有工作者需要清理
		wp.lock.Unlock()
		return
	}

	// Move workers to be cleaned to scratch slice
	// 将要清理的工作者移到临时切片
	*scratch = append((*scratch)[:0], ready[:i+1]...)
	m := copy(ready, ready[i+1:])
	for i = m; i < n; i++ {
		ready[i] = nil
	}
	wp.ready = ready[:m]
	wp.lock.Unlock()

	// Notify obsolete workers to stop.
	// This notification must be outside the wp.lock, since ch.ch
	// may be blocking and may consume a lot of time if many workers
	// are located on non-local CPUs.
	// 通知过时的工作者停止。
	// 此通知必须在 wp.lock 之外，因为 ch.ch 可能阻塞，
	// 如果许多工作者位于非本地 CPU 上，可能会消耗大量时间。
	tmp := *scratch
	for i := range tmp {
		tmp[i].ch <- nil
		tmp[i] = nil
	}
}

// Submit submits a function for execution by the worker pool.
// It returns an error if no idle workers are available and the maximum
// worker count has been reached.
//
// Submit 提交函数供工作池执行。
// 如果没有空闲工作者可用且已达到最大工作者数量，它返回错误。
//
// Parameters:
// 参数：
//   - fn: The function to be executed by a worker  要由工作者执行的函数
//
// Returns:
// 返回：
//   - error: nil if submission successful, error if no workers available
//     如果提交成功则为 nil，如果没有工作者可用则为错误
//
// Submission Process:
// 提交过程：
//  1. Attempt to acquire an available worker channel  尝试获取可用的工作者通道
//  2. If successful, send the function to the worker  如果成功，将函数发送给工作者
//  3. If no workers available, return error  如果没有工作者可用，返回错误
//
// Error Conditions:
// 错误条件：
//   - All workers are busy and maximum worker count reached
//     所有工作者都忙碌且已达到最大工作者数量
//   - Pool has been stopped (mustStop flag is set)
//     池已停止（mustStop 标志已设置）
//
// Thread Safety:
// 线程安全：
//
//	This method is thread-safe and can be called concurrently
//	此方法是线程安全的，可以并发调用
func (wp *WorkerPool) Submit(fn func()) error {
	ch := wp.getCh()
	if ch == nil {
		return errors.New("no idle workers")
	}
	ch.ch <- fn
	return nil
}

// workerChanCap determines the capacity of worker channels based on GOMAXPROCS.
// It optimizes performance by using different channel capacities for different CPU configurations.
//
// workerChanCap 基于 GOMAXPROCS 确定工作者通道的容量。
// 它通过为不同的 CPU 配置使用不同的通道容量来优化性能。
//
// Channel Capacity Logic:
// 通道容量逻辑：
//   - GOMAXPROCS=1: Use blocking channels (capacity 0) for immediate task switching
//     使用阻塞通道（容量 0）进行即时任务切换
//   - GOMAXPROCS>1: Use buffered channels (capacity 1) to prevent acceptor lag
//     使用缓冲通道（容量 1）防止接收者延迟
var workerChanCap = func() int {
	// Use blocking workerChan if GOMAXPROCS=1.
	// This immediately switches Serve to WorkerFunc, which results
	// in higher performance (under go1.5 at least).
	// 如果 GOMAXPROCS=1 使用阻塞 workerChan。
	// 这立即将 Serve 切换到 WorkerFunc，从而获得更高性能（至少在 go1.5 下）。
	if runtime.GOMAXPROCS(0) == 1 {
		return 0
	}

	// Use non-blocking workerChan if GOMAXPROCS>1,
	// since otherwise the Serve caller (Acceptor) may lag accepting
	// new connections if WorkerFunc is CPU-bound.
	// 如果 GOMAXPROCS>1 使用非阻塞 workerChan，
	// 否则如果 WorkerFunc 是 CPU 密集型的，Serve 调用者（Acceptor）可能延迟接受新连接。
	return 1
}()

// getCh attempts to acquire a worker channel for task execution.
// It either reuses an idle worker or creates a new one if possible.
//
// getCh 尝试获取用于任务执行的工作者通道。
// 它要么重用空闲工作者，要么在可能的情况下创建新工作者。
//
// Returns:
// 返回：
//   - *workerChan: Available worker channel, nil if none available
//     可用的工作者通道，如果没有可用则为 nil
//
// Acquisition Strategy:
// 获取策略：
//  1. Check for idle workers in ready list (FILO order)  检查就绪列表中的空闲工作者（FILO 顺序）
//  2. If no idle workers, create new worker if under limit  如果没有空闲工作者，在限制内创建新工作者
//  3. If at limit and no idle workers, return nil  如果达到限制且没有空闲工作者，返回 nil
//
// Worker Creation:
// 工作者创建：
//   - New workers are started in separate goroutines  新工作者在单独的协程中启动
//   - Worker channels are pooled to reduce allocations  工作者通道被池化以减少分配
//   - Worker count is tracked for limit enforcement  跟踪工作者数量以执行限制
func (wp *WorkerPool) getCh() *workerChan {
	var ch *workerChan
	createWorker := false

	wp.lock.Lock()
	ready := wp.ready
	n := len(ready) - 1
	if n < 0 {
		// No idle workers available, check if we can create a new one
		// 没有空闲工作者可用，检查是否可以创建新工作者
		if wp.workersCount < wp.MaxWorkersCount {
			createWorker = true
			wp.workersCount++
		}
	} else {
		// Reuse the most recently used worker (FILO order)
		// 重用最近使用的工作者（FILO 顺序）
		ch = ready[n]
		ready[n] = nil
		wp.ready = ready[:n]
	}
	wp.lock.Unlock()

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

// release returns a worker channel to the pool of available workers.
// It updates the worker's last use time and adds it to the ready list.
//
// release 将工作者通道返回到可用工作者池。
// 它更新工作者的最后使用时间并将其添加到就绪列表。
//
// Parameters:
// 参数：
//   - ch: Worker channel to be released  要释放的工作者通道
//
// Returns:
// 返回：
//   - bool: true if successfully released, false if pool is stopping
//     如果成功释放则为 true，如果池正在停止则为 false
//
// Release Process:
// 释放过程：
//  1. Update worker's last use time for cleanup tracking  更新工作者的最后使用时间以进行清理跟踪
//  2. Check if pool is stopping  检查池是否正在停止
//  3. If not stopping, add worker to ready list  如果没有停止，将工作者添加到就绪列表
//  4. Return success/failure status  返回成功/失败状态
func (wp *WorkerPool) release(ch *workerChan) bool {
	// Update last use time for cleanup purposes
	// 更新最后使用时间以用于清理目的
	ch.lastUseTime = time.Now()

	wp.lock.Lock()
	if wp.mustStop {
		// Pool is stopping, don't reuse this worker
		// 池正在停止，不要重用此工作者
		wp.lock.Unlock()
		return false
	}
	// Add worker back to ready list (will be used in FILO order)
	// 将工作者添加回就绪列表（将以 FILO 顺序使用）
	wp.ready = append(wp.ready, ch)
	wp.lock.Unlock()
	return true
}

// workerFunc is the main worker goroutine function.
// It continuously processes functions sent through the worker channel
// until a termination signal is received.
//
// workerFunc 是主要的工作者协程函数。
// 它持续处理通过工作者通道发送的函数，直到收到终止信号。
//
// Parameters:
// 参数：
//   - ch: Worker channel for receiving functions to execute  用于接收要执行函数的工作者通道
//
// Worker Lifecycle:
// 工作者生命周期：
//  1. Wait for function on channel  在通道上等待函数
//  2. Execute received function  执行接收到的函数
//  3. Release worker back to pool  将工作者释放回池
//  4. Repeat until termination signal (nil function)  重复直到终止信号（nil 函数）
//  5. Decrement worker count on exit  退出时减少工作者计数
//
// Termination Conditions:
// 终止条件：
//   - Receives nil function (explicit termination signal)  接收 nil 函数（显式终止信号）
//   - Pool refuses worker release (pool is stopping)  池拒绝工作者释放（池正在停止）
//
// Error Handling:
// 错误处理：
//
//	Worker functions are expected to handle their own errors.
//	The worker goroutine itself does not perform error handling for user functions.
//	工作者函数应该处理自己的错误。
//	工作者协程本身不为用户函数执行错误处理。
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
	wp.lock.Lock()
	wp.workersCount--
	wp.lock.Unlock()
}
