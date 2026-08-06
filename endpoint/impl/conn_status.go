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
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
)

// ConnStatus is a mixin for endpoints that manage their own connections (i.e.
// do not embed base.SharedNode) to expose connection status. Embed it and call
// SetConnStatus at every connection state transition; ConnectionStatus then
// satisfies types.ConnectionStatusGetter, so the status is picked up by the
// chain-level aggregation (types.ChainStatusesGetter) and the server status API.
//
// The status field is atomic and the message is guarded by an independent lock,
// so SetConnStatus is safe to call from any goroutine (read loops, reconnect
// loops, Start/Destroy) without risking a deadlock with the endpoint's own lock.
type ConnStatus struct {
	status int32 // types.NodeStatus
	msgMu  sync.RWMutex
	msg    string
}

// SetConnStatus updates the connection status and last message.
func (c *ConnStatus) SetConnStatus(s types.NodeStatus, msg string) {
	atomic.StoreInt32(&c.status, int32(s))
	c.msgMu.Lock()
	c.msg = msg
	c.msgMu.Unlock()
}

// ConnectionStatus returns the current connection status. Implements
// types.ConnectionStatusGetter.
func (c *ConnStatus) ConnectionStatus() types.StatusInfo {
	c.msgMu.RLock()
	msg := c.msg
	c.msgMu.RUnlock()
	return types.StatusInfo{
		Status:  types.NodeStatus(atomic.LoadInt32(&c.status)),
		Message: msg,
	}
}
