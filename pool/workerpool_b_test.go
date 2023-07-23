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

func demoTask(v ...interface{}) {
	for i := 0; i < 100; i++ {
		atomic.AddInt64(&sum, 1)
	}
}

func demoTask2(v ...interface{}) {
	defer wg.Done()
	for i := 0; i < 100; i++ {
		atomic.AddInt64(&sum, 1)
	}
}

func BenchmarkGoroutine(b *testing.B) {
	for i := 0; i < runTimes; i++ {
		go func() {
			wg.Add(1)
			demoTask2()
		}()
	}
	wg.Wait()
}

//func BenchmarkAntsPoolTimeLifeSetTimes(b *testing.B) {
//	for i := 0; i < 100000; i++ {
//		wg.Add(1)
//		ants.Submit(func() {
//			demoTask2()
//		})
//	}
//
//	wg.Wait()
//}
func BenchmarkWorkPoolTimeLifeSetTimes(b *testing.B) {
	wp := &WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	//defer wp.Stop()
	b.ResetTimer()
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		wp.Submit(func() {
			demoTask2()
		})
	}

	wg.Wait()
}
