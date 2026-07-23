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
// Package pool provides a high-performance work pool implementation for executing concurrent tasks.
// It includes an optimized work pool that effectively manages coroutines to reduce allocation overhead.
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
// WorkerPool uses the working pool to process incoming functions in FILO order.
// The recently stopped worker will handle the next input function.
// This approach maintains CPU cache heat for better performance.
//
// Key Features:
// Key features:
//   - FILO (First In, Last Out) worker management for CPU cache efficiency
//     FILO (First In, First Out) worker management to improve CPU cache efficiency
//   - Configurable maximum worker count and idle duration
//     Maximum configurable number of workers and duration of idleness
//   - Automatic worker cleanup for memory efficiency
//     Automatic worker cleanup to improve memory efficiency
//   - Non-blocking task submission with error handling
//     Non-blocking task submission and error handling
//   - Graceful start/stop lifecycle management
//     Elegant start/stop lifecycle management
//
// Performance Benefits:
// Performance Advantages:
//   - Reduces goroutine allocation overhead
//     Reduces coroutine allocation overhead
//   - Maintains hot CPU caches through worker reuse
//     Maintain the hot CPU cache through worker reuse
//   - Efficient memory usage with automatic cleanup
//     Efficient memory usage achieved through automatic cleaning
//   - Scalable worker management based on load
//     Load-based scalable worker management
//
// Usage Example:
// Example:
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
	// MaxWorkersCount is the maximum number of workers that can be created.
	// If set to 0, the pool will create unlimited workers (not recommended).
	MaxWorkersCount int

	// MaxIdleWorkerDuration is the maximum duration a worker can remain idle
	// before being cleaned up. Workers idle longer than this duration will be terminated.
	// Default is 10 seconds if not specified.
	// MaxIdleWorkerDuration is the maximum duration a worker can remain free before being cleaned up.
	// Workers whose idle hours exceed this duration will be terminated.
	// If not specified, the default is 10 seconds.
	MaxIdleWorkerDuration time.Duration

	// lock provides thread-safe access to internal state
	// lock provides thread-safe access to internal states
	lock sync.Mutex

	// workersCount tracks the current number of active workers
	// workersCount tracks the number of active workers currently active
	workersCount int

	// mustStop indicates whether the pool should stop accepting new tasks
	// mustStop indicates whether the pool should stop accepting new tasks
	mustStop bool

	// ready maintains a list of available workers in FILO order
	// ready Maintains the list of available workers in FILO order
	ready []*workerChan

	// stopCh is used to signal the cleanup goroutine to stop
	// stopCh is used to send a stop signal to the cleaning coroutine
	stopCh chan struct{}

	// workerChanPool pools worker channel objects to reduce allocations
	// workerChanPool pools worker channel objects to reduce allocation
	workerChanPool sync.Pool

	// startOnce ensures the pool is started only once
	// startOnce ensures the pool only starts once
	startOnce sync.Once
}

// workerChan represents a worker with its communication channel and metadata.
// It encapsulates the worker's state and provides the communication mechanism
// between the pool and the worker goroutine.
//
// workerChan refers to workers with communication channels and metadata.
// It encapsulates the state of workers and provides a communication mechanism between the pool and worker coroutines.
type workerChan struct {
	// lastUseTime records when the worker was last used for cleanup purposes
	// lastUseTime records the last time a worker used for cleaning purposes
	lastUseTime time.Time

	// ch is the communication channel for sending functions to the worker
	// ch is the communication channel for sending functions to the worker
	ch chan func()
}

// Start initializes and starts the worker pool.
// It creates the cleanup goroutine and sets up the worker channel pool.
// This method is thread-safe and can be called multiple times safely.
//
// Start initializes and starts the working pool.
// It creates cleanup coroutines and sets up the worker channel pool.
// This method is thread-safe and can be safely called multiple times.
//
// Initialization Process:
// Initialization process:
//  1. Create stop channel for cleanup coordination
//  2. Initialize worker channel pool with factory function
//  3. Start background cleanup goroutine
//  4. Set up periodic worker cleanup based on idle duration
//
// The cleanup goroutine runs continuously until Stop() is called and:
// The cleanup coroutine continues running until Stop() is called, and:
//   - Removes workers that have been idle longer than MaxIdleWorkerDuration
//     Removes workers whose idle time exceeds MaxIdleWorkerDuration
//   - Maintains optimal pool size based on workload
//   - Prevents memory leaks from unused workers
func (wp *WorkerPool) Start() {
	if wp.stopCh != nil {
		return
	}
	wp.startOnce.Do(func() {
		// Create stop channel for cleanup coordination
		// Create stop lanes for cleaning coordination
		wp.stopCh = make(chan struct{})
		stopCh := wp.stopCh

		// Initialize worker channel pool with factory function
		// Use factory functions to initialize the worker channel pool
		wp.workerChanPool.New = func() interface{} {
			return &workerChan{
				ch: make(chan func(), workerChanCap),
			}
		}

		// Start background cleanup goroutine
		// Start the background cleanup coroutine
		go func() {
			var scratch []*workerChan
			for {
				// Clean up idle workers
				// Clearing out idle workers
				wp.clean(&scratch)
				select {
				case <-stopCh:
					// Pool has been stopped, exit cleanup loop
					// The pool has stopped, and the cleanup cycle is exited
					return
				default:
					// Wait for next cleanup cycle
					// Waiting for the next cleanup cycle
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
// Stop gracefully closing the work pool.
// It stops accepting new tasks and sends termination signals to all idle workers.
// Busy workers will terminate after completing their current tasks.
//
// Shutdown Process:
// Closing process:
//  1. Close the stop channel to signal cleanup goroutine
//  2. Set mustStop flag to prevent new task acceptance
//  3. Send termination signals to all idle workers
//  4. Clear the ready worker list
//
// Note: This method does not wait for busy workers to complete.
// For applications requiring graceful shutdown, consider implementing
// a separate mechanism to wait for task completion.
//
// Note: This method does not wait for busy workers to complete.
// For applications that need to close gracefully, consider implementing separate mechanisms to wait for tasks to complete.
func (wp *WorkerPool) Stop() {
	if wp.stopCh == nil {
		return
	}

	// Signal cleanup goroutine to stop
	// Send a stop signal to the cleanup coroutine
	close(wp.stopCh)
	wp.stopCh = nil

	// Stop all the workers waiting for incoming connections.
	// Do not wait for busy workers - they will stop after
	// serving the connection and noticing wp.mustStop = true.
	// Stop all workers waiting for incoming connections.
	// Busy workers are not waiting — they will stop after the service connection and note wp.mustStop = true.
	wp.lock.Lock()
	ready := wp.ready
	for i := range ready {
		// Send termination signal to each idle worker
		// Send a termination signal to each idle worker
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
// Release is another name for Stop(), which provides compatibility support.
// It performs the same close operation as Stop().
func (wp *WorkerPool) Release() {
	wp.Stop()
}

// getMaxIdleWorkerDuration returns the configured idle duration or a default value.
// It provides a sensible default of 10 seconds if no duration is specified.
//
// getMaxIdleWorkerDuration returns the configured idle duration or default value.
// If no duration is specified, it provides a reasonable default value of 10 seconds.
//
// Returns:
// Returns:
//   - time.Duration: The idle duration after which workers are cleaned up
//     The duration of downtime for workers being cleared
func (wp *WorkerPool) getMaxIdleWorkerDuration() time.Duration {
	if wp.MaxIdleWorkerDuration <= 0 {
		return 10 * time.Second
	}
	return wp.MaxIdleWorkerDuration
}

// clean removes idle workers that have exceeded the maximum idle duration.
// It uses binary search for efficient identification of workers to be cleaned.
//
// clean: removes idle workers who have exceeded the maximum idle duration.
// It uses binary search to efficiently identify workers to be cleaned up.
//
// Parameters:
// Parameters:
//   - scratch: Reusable slice to minimize allocations during cleanup
//     Reusable slices minimize distribution during cleanup
//
// Algorithm:
// Algorithm:
//  1. Calculate critical time threshold for cleanup
//  2. Use binary search to find oldest workers to clean
//  3. Move remaining workers to front of ready list
//  4. Send termination signals to cleaned workers
//
// Performance: O(log n + m) where n is ready workers and m is workers to clean
// Performance: O(log n + m), where n is the number of ready workers and m is the number of workers to be cleaned
func (wp *WorkerPool) clean(scratch *[]*workerChan) {
	maxIdleWorkerDuration := wp.getMaxIdleWorkerDuration()

	// Clean least recently used workers if they didn't serve connections
	// for more than maxIdleWorkerDuration.
	// Clean up the least recently used workers if they exceed maxIdleWorkerDuration and have no service connections.
	criticalTime := time.Now().Add(-maxIdleWorkerDuration)

	wp.lock.Lock()
	ready := wp.ready
	n := len(ready)

	// Use binary-search algorithm to find out the index of the least recently worker which can be cleaned up.
	// Using a binary search algorithm, it finds the index of the most recent least used worker that can be cleaned.
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
		// No workers need to clean up
		wp.lock.Unlock()
		return
	}

	// Move workers to be cleaned to scratch slice
	// Move the workers to be cleaned to temporary slices
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
	// Notify outdated workers to stop.
	// This notification must be outside of wp.lock, because ch.ch may be blocked,
	// If many workers are on non-local CPUs, it can consume a lot of time.
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
// Submit a function for the working pool to execute.
// If no idle workers are available and the maximum number of workers has been reached, it returns an error.
//
// Parameters:
// Parameters:
//   - fn: The function to be executed by a worker
//
// Returns:
// Returns:
//   - error: nil if submission successful, error if no workers available
//     If the submission succeeds, it is nil; if no workers are available, it is an error
//
// Submission Process:
// Submission process:
//  1. Attempt to acquire an available worker channel
//  2. If successful, send the function to the worker
//  3. If no workers available, return error
//
// Error Conditions:
// False condition:
//   - All workers are busy and maximum worker count reached
//     All workers are busy and have reached the maximum number of workers
//   - Pool has been stopped (mustStop flag is set)
//     The pool has stopped (mustStop flag is set)
//
// Thread Safety:
// Thread safety:
//
//	This method is thread-safe and can be called concurrently
//	This method is thread-safe and can be called concurrently
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
// workerChanCap determines worker channel capacity based on GOMAXPROCS.
// It optimizes performance by using different channel capacities for different CPU configurations.
//
// Channel Capacity Logic:
// Channel capacity logic:
//   - GOMAXPROCS=1: Use blocking channels (capacity 0) for immediate task switching
//     Use blocked channels (capacity 0) for instant task switching
//   - GOMAXPROCS>1: Use buffered channels (capacity 1) to prevent acceptor lag
//     Use a buffer channel (capacity 1) to prevent receiver delay
var workerChanCap = func() int {
	// Use blocking workerChan if GOMAXPROCS=1.
	// This immediately switches Serve to WorkerFunc, which results
	// in higher performance (under go1.5 at least).
	// If GOMAXPROCS=1 uses blocking workerChan.
	// This immediately switches Serve to WorkerFunc, resulting in higher performance (at least at least on go1.5).
	if runtime.GOMAXPROCS(0) == 1 {
		return 0
	}

	// Use non-blocking workerChan if GOMAXPROCS>1,
	// since otherwise the Serve caller (Acceptor) may lag accepting
	// new connections if WorkerFunc is CPU-bound.
	// If GOMAXPROCS>1 uses non-blocking workerChan,
	// Otherwise, if WorkerFunc is CPU-intensive, the Serve caller (Acceptor) may delay accepting new connections.
	return 1
}()

// getCh attempts to acquire a worker channel for task execution.
// It either reuses an idle worker or creates a new one if possible.
//
// getCh attempts to obtain the worker channel for task execution.
// It either reuses idle workers or, where possible, creates new ones.
//
// Returns:
// Returns:
//   - *workerChan: Available worker channel, nil if none available
//     Available worker channels, if not available, are nil
//
// Acquisition Strategy:
// Acquisition strategy:
//  1. Check for idle workers in ready list (FILO order)
//  2. If no idle workers, create new worker if under limit
//  3. If at limit and no idle workers, return nil
//
// Worker Creation:
// Worker Creation:
//   - New workers are started in separate goroutines
//   - Worker channels are pooled to reduce allocations
//   - Worker count is tracked for limit enforcement
func (wp *WorkerPool) getCh() *workerChan {
	var ch *workerChan
	createWorker := false

	wp.lock.Lock()
	ready := wp.ready
	n := len(ready) - 1
	if n < 0 {
		// No idle workers available, check if we can create a new one
		// There are no available workers; check if new workers can be created
		if wp.workersCount < wp.MaxWorkersCount {
			createWorker = true
			wp.workersCount++
		}
	} else {
		// Reuse the most recently used worker (FILO order)
		// Reuse recently used workers (FILO order)
		ch = ready[n]
		ready[n] = nil
		wp.ready = ready[:n]
	}
	wp.lock.Unlock()

	if ch == nil {
		if !createWorker {
			// Cannot create new worker and no idle workers available
			// Unable to create new workers and with no available workers
			return nil
		}
		// Create new worker channel from pool
		// Create new worker channels from the pool
		vch := wp.workerChanPool.Get()
		ch = vch.(*workerChan)
		go func() {
			// Start worker goroutine
			// Initiate worker coordination
			wp.workerFunc(ch)
			// Return worker channel to pool when done
			// Upon completion, return the worker channel to the pool
			wp.workerChanPool.Put(vch)
		}()
	}
	return ch
}

// release returns a worker channel to the pool of available workers.
// It updates the worker's last use time and adds it to the ready list.
//
// release returns the worker channel to the available worker pool.
// It updates the last usage time of the worker and adds it to the ready list.
//
// Parameters:
// Parameters:
//   - ch: Worker channel to be released
//
// Returns:
// Returns:
//   - bool: true if successfully released, false if pool is stopping
//     If released successfully, it is true; if the pool is stopped, it is false
//
// Release Process:
// Release process:
//  1. Update worker's last use time for cleanup tracking
//  2. Check if pool is stopping
//  3. If not stopping, add worker to ready list
//  4. Return success/failure status
func (wp *WorkerPool) release(ch *workerChan) bool {
	// Update last use time for cleanup purposes
	// Updated last usage times for cleaning purposes
	ch.lastUseTime = time.Now()

	wp.lock.Lock()
	if wp.mustStop {
		// Pool is stopping, don't reuse this worker
		// The pool is stopping, and don't reuse this worker
		wp.lock.Unlock()
		return false
	}
	// Add worker back to ready list (will be used in FILO order)
	// Add workers back to the ready list (will be used in FILO order)
	wp.ready = append(wp.ready, ch)
	wp.lock.Unlock()
	return true
}

// workerFunc is the main worker goroutine function.
// It continuously processes functions sent through the worker channel
// until a termination signal is received.
//
// workerFunc is the main worker coroutine function.
// It continuously processes functions sent through the worker channel until a termination signal is received.
//
// Parameters:
// Parameters:
//   - ch: Worker channel for receiving functions to execute
//
// Worker Lifecycle:
// Worker Lifecycle:
//  1. Wait for function on channel
//  2. Execute received function
//  3. Release worker back to pool
//  4. Repeat until termination signal (nil function)
//  5. Decrement worker count on exit
//
// Termination Conditions:
// Termination Conditions:
//   - Receives nil function (explicit termination signal)
//   - Pool refuses worker release (pool is stopping)
//
// Error Handling:
// Error handling:
//
//	Worker functions are expected to handle their own errors.
//	The worker goroutine itself does not perform error handling for user functions.
//	The worker function should handle its own errors.
//	Worker coroutines themselves do not perform error handling for user functions.
func (wp *WorkerPool) workerFunc(ch *workerChan) {
	var fn func()
	for fn = range ch.ch {
		if fn == nil {
			// Termination signal received
			// A termination signal is received
			break
		}

		// Execute the user function
		// Execute the user function
		fn()
		fn = nil

		// Try to release the worker back to the pool
		// Try to release workers back into the pool
		if !wp.release(ch) {
			// Pool is stopping, exit worker goroutine
			// The pool is stopping and exiting the worker coroutine
			break
		}
	}

	// Decrement worker count when exiting
	// Reduce worker counts upon exit
	wp.lock.Lock()
	wp.workersCount--
	wp.lock.Unlock()
}
