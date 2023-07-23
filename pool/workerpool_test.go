package pool

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	wp := &WorkerPool{MaxWorkersCount: 200000}
	wp.Start()
	defer wp.Stop()
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

	if n != 10000 {
		t.Fatalf("unexpected number of served functions: %d. Expecting %d", n, 100)
	}
}
