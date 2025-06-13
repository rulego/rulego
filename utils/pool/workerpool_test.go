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
