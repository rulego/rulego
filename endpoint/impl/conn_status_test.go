/*
 * Copyright 2024 The RuleGo Authors.
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

package impl

import (
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// Status and message must always be updated together and read back consistently.
func TestConnStatusTransitions(t *testing.T) {
	var c ConnStatus
	assert.Equal(t, types.StatusNone, c.ConnectionStatus().Status)

	transitions := []struct {
		status  types.NodeStatus
		message string
	}{
		{types.StatusConnected, ""},
		{types.StatusReconnecting, "connection lost"},
		{types.StatusDisconnected, ""},
	}
	for _, tr := range transitions {
		c.SetConnStatus(tr.status, tr.message)
		got := c.ConnectionStatus()
		assert.Equal(t, tr.status, got.Status)
		assert.Equal(t, tr.message, got.Message)
	}
}

// SetConnStatus must be callable from concurrent goroutines (read loops,
// reconnect loops, Start/Destroy) without data races on the message.
func TestConnStatusConcurrent(t *testing.T) {
	var c ConnStatus
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.SetConnStatus(types.StatusReconnecting, "retry")
			_ = c.ConnectionStatus()
		}(i)
	}
	wg.Wait()
}
