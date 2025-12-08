//go:build !go1.21

package engine

// startMonitoring starts monitoring parent contexts using a goroutine for Go < 1.21.
// This provides backward compatibility for older Go versions.
// startMonitoring 启动监控父上下文，对于 Go < 1.21 使用协程。
// 这为旧版本的 Go 提供了向后兼容性。
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
		// 启动单个协程来监控两个上下文
		go func() {
			// 如果 userCtx 是不可取消的（如 context.Background() 或 context.TODO()），
			// 我们不需要监听它，只需要监听 shutdownCtx 和 internal ctx
			// If userCtx is not cancellable (Done() returns nil), we don't need to select on it
			if c.userCtx.Done() == nil {
				select {
				case <-c.shutdownCtx.Done():
					c.setErr(c.shutdownCtx.Err())
					c.cancel()
				case <-c.ctx.Done():
					// Internal context cancelled, exit goroutine
					// 内部上下文已取消，退出协程
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
					// 内部上下文已取消，退出协程
				}
			}
		}()
	})
}
