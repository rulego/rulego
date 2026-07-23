//go:build go1.21

package engine

import "context"

// startMonitoring starts monitoring parent contexts using AfterFunc for Go 1.21+.
// This avoids creating a dedicated goroutine for waiting.
// startMonitoring: Starts monitoring the parent context, and for Go 1.21+, use AfterFunc.
// This avoids creating dedicated wait-waiting coroutines.
func (c *combinedCancelContext) startMonitoring() {
	c.doneOnce.Do(func() {
		// If either is already done, cancel immediately
		if c.userCtx.Err() != nil {
			c.setErr(c.userCtx.Err())
			c.cancel()
			return
		}
		if c.shutdownCtx.Err() != nil {
			c.setErr(c.shutdownCtx.Err())
			c.cancel()
			return
		}

		// Use context.AfterFunc to register cancellation callbacks
		// This is more efficient than a blocking goroutine
		stopUser := context.AfterFunc(c.userCtx, func() {
			c.setErr(c.userCtx.Err())
			c.cancel()
		})

		stopShutdown := context.AfterFunc(c.shutdownCtx, func() {
			c.setErr(c.shutdownCtx.Err())
			c.cancel()
		})

		// Register cleanup when the combined context is done
		// This ensures we stop monitoring parents when we are done
		context.AfterFunc(c.ctx, func() {
			stopUser()
			stopShutdown()
		})
	})
}
