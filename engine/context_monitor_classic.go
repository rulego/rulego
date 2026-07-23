//go:build !go1.21

package engine

// startMonitoring starts monitoring parent contexts using a goroutine for Go < 1.21.
// This provides backward compatibility for older Go versions.
// startMonitoring: Starts monitoring the parent context, and for Go < 1.21, use a coroutine.
// This provides backward compatibility for older versions of Go.
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

		// Use goroutine for backward compatibility with Go < 1.21
		// Initiate a single coroutine to monitor two contexts
		go func() {
			// If userCtx is non-revocable (such as context.Background() or context.TODO()),
			// We don't need to monitor it, only shutdownCtx and internal ctx
			// If userCtx is not cancellable (Done() returns nil), we don't need to select on it
			if c.userCtx.Done() == nil {
				select {
				case <-c.shutdownCtx.Done():
					c.setErr(c.shutdownCtx.Err())
					c.cancel()
				case <-c.ctx.Done():
					// Internal context cancelled, exit goroutine
					// Internal context has been removed, and the coroutine has been removed
				}
			} else {
				select {
				case <-c.userCtx.Done():
					c.setErr(c.userCtx.Err())
					c.cancel()
				case <-c.shutdownCtx.Done():
					c.setErr(c.shutdownCtx.Err())
					c.cancel()
				case <-c.ctx.Done():
					// Internal context cancelled, exit goroutine
					// Internal context has been removed, and the coroutine has been removed
				}
			}
		}()
	})
}
