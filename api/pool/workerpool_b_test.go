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
)

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

//	func BenchmarkAntsPoolTimeLifeSetTimes(b *testing.B) {
//		wg.Add(runTimes)
//		for i := 0; i < runTimes; i++ {
//			ants.Submit(func() {
//				demoTask2()
//			})
//		}
//
//		wg.Wait()
//	}
func BenchmarkWorkPoolTimeLifeSetTimes(b *testing.B) {
	wp := &WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	//defer wp.Stop()
	b.ResetTimer()
	wg.Add(runTimes)
	for i := 0; i < runTimes; i++ {
		wp.Submit(func() {
			demoTask2()
		})
	}

	wg.Wait()
}
