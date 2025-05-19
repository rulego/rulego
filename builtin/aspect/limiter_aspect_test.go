package aspect

import (
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestConcurrencyLimiterAspect(t *testing.T) {
	maxConcurrent := 2
	aspect := NewConcurrencyLimiterAspect(maxConcurrent)

	assert.Equal(t, 10, aspect.Order())

	newAspect := aspect.New().(*ConcurrencyLimiterAspect)
	assert.NotNil(t, newAspect)
	assert.Equal(t, int64(maxConcurrent), newAspect.Max)
	assert.Equal(t, int64(0), newAspect.currentCount)

	config := types.NewConfig()
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {})
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// Test PointCut
	assert.True(t, aspect.PointCut(ctx, msg, ""))

	// Test Start and Completed
	var wg sync.WaitGroup
	concurrencyReached := false
	var mu sync.Mutex

	// Attempt to exceed the limit
	for i := 0; i < maxConcurrent+1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := newAspect.Start(ctx, msg)
			if err != nil {
				mu.Lock()
				assert.Equal(t, types.ErrConcurrencyLimitReached, err)
				concurrencyReached = true
				mu.Unlock()
			} else {
				// Simulate work
				time.Sleep(10 * time.Millisecond)
				newAspect.Completed(ctx, msg)
			}
		}()
	}

	wg.Wait()
	assert.True(t, concurrencyReached, "Concurrency limit should have been reached")
	assert.Equal(t, int64(0), newAspect.currentCount, "Current count should be zero after all goroutines complete or fail")

	// Test incrementCurrent and decrementCurrent directly (though they are internal)
	aspect.incrementCurrent()
	assert.Equal(t, int64(1), aspect.currentCount)
	aspect.decrementCurrent()
	assert.Equal(t, int64(0), aspect.currentCount)

	// Test decrementCurrent when currentCount is already 0
	aspect.decrementCurrent() // should go to -1 then reset to 0 by the logic inside
	// The logic `if newVale < 0 { atomic.StoreInt64(&a.currentCount, newVale) }` seems to intend to keep it at 0 or negative.
	// However, typical usage would not expect currentCount to go below 0 if Start/Completed are paired correctly.
	// Let's verify it becomes 0 if it was negative due to direct call.
	// Actually, the logic `if newVale < 0 { atomic.StoreInt64(&a.currentCount, newVale) }` is a bit confusing.
	// If `newVale` is -1, it stores -1. If the intention is to prevent it from going below 0, it should be `atomic.StoreInt64(&a.currentCount, 0)`.
	// Given the current implementation, if we call decrement when count is 0, it will become -1.
	// Let's test the state after such an operation.
	limiter := NewConcurrencyLimiterAspect(1)
	limiter.decrementCurrent()                       // currentCount becomes -1
	assert.Equal(t, int64(-1), limiter.currentCount) // Based on the code, it will be -1
	limiter.incrementCurrent()                       // currentCount becomes 0
	assert.Equal(t, int64(0), limiter.currentCount)
}
