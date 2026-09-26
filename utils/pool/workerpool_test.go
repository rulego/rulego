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

package pool

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	wp := &WorkerPool{MaxWorkersCount: 200000}
	wp.Start()
	defer func() {
		wp.Stop()
	}()
	var n int32
	fn := func() {
		atomic.AddInt32(&n, 1)
	}

	for i := 0; i < 10000; i++ {
		if wp.Submit(fn) != nil {
			t.Fatalf("cannot submit function #%d", i)
		}
	}

	time.Sleep(time.Second)

	if atomic.LoadInt32(&n) != 10000 {
		t.Fatalf("unexpected number of served functions: %d. Expecting %d", atomic.LoadInt32(&n), 10000)
	}
	wp.Release()
	if wp.Submit(fn) != nil {
		t.Fatalf("cannot submit")
	}
}

func TestWorkerPoolWithMaxIdleWorkerD(t *testing.T) {
	wp := &WorkerPool{MaxWorkersCount: 200000, MaxIdleWorkerDuration: time.Second * 10}
	wp.Start()
	defer func() {
		wp.Stop()
	}()
	var n int32
	fn := func() {
		atomic.AddInt32(&n, 1)
	}

	for i := 0; i < 10000; i++ {
		if wp.Submit(fn) != nil {
			t.Fatalf("cannot submit function #%d", i)
		}
	}

	time.Sleep(time.Second)

	if atomic.LoadInt32(&n) != 10000 {
		t.Fatalf("unexpected number of served functions: %d. Expecting %d", atomic.LoadInt32(&n), 10000)
	}
	wp.Release()
	if wp.Submit(fn) != nil {
		t.Fatalf("cannot submit")
	}
}

func TestWorkerPoolWithDoubleStart(*testing.T) {
	wp := &WorkerPool{MaxWorkersCount: 200000, MaxIdleWorkerDuration: time.Second * 10}
	wp.Start()
	wp.Start()
	defer func() {
		wp.Stop()
	}()
}

// Benchmarks below were moved from the former workerpool_b_test.go so the
// workerpool.go source has a single test file.

var sum int64
var runTimes = 10000

var wg = sync.WaitGroup{}

func demoTask2(v ...interface{}) {
	defer wg.Done()
	for i := 0; i < 100; i++ {
		atomic.AddInt64(&sum, 1)
	}
}

func BenchmarkGoroutine(b *testing.B) {
	wg.Add(runTimes)
	for i := 0; i < runTimes; i++ {
		go func() {
			demoTask2()
		}()
	}
	wg.Wait()
}

func BenchmarkWorkPoolTimeLifeSetTimes(b *testing.B) {
	wp := &WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	b.ResetTimer()
	wg.Add(runTimes)
	for i := 0; i < runTimes; i++ {
		wp.Submit(func() {
			demoTask2()
		})
	}

	wg.Wait()
}

func TestWorkerPoolSubmitWithoutQuota(t *testing.T) {
	// zero quota on every shard: Submit must report the failure instead of blocking
	wp := &WorkerPool{MaxWorkersCount: 0}
	wp.Start()
	defer wp.Stop()
	if err := wp.Submit(func() {}); err == nil {
		t.Fatal("expected error when no worker can be created")
	}
}

// clean() reaps workers idle for longer than MaxIdleWorkerDuration.
func TestWorkerPoolCleanIdleWorkers(t *testing.T) {
	wp := &WorkerPool{MaxWorkersCount: 100, MaxIdleWorkerDuration: time.Millisecond}
	wp.Start()
	defer wp.Stop()

	var done sync.WaitGroup
	done.Add(1)
	if err := wp.Submit(done.Done); err != nil {
		t.Fatalf("cannot submit: %v", err)
	}
	done.Wait()

	// wait for the worker to be released and reaped by the cleanup loop
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		total := 0
		for _, sh := range wp.shards {
			sh.lock.Lock()
			total += len(sh.ready)
			sh.lock.Unlock()
		}
		if total == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("idle workers were not cleaned up")
}
