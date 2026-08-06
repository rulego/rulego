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

package types

// NodeStatus represents a connection status.
type NodeStatus int

const (
	// StatusNone: no connection (no concept or no activity yet).
	StatusNone NodeStatus = iota
	// StatusConnected: connection established.
	StatusConnected
	// StatusReconnecting: connecting or reconnecting in progress; Message holds the last error.
	StatusReconnecting
	// StatusDisconnected: disconnected (closed or not started).
	StatusDisconnected
)

func (s NodeStatus) String() string {
	switch s {
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusDisconnected:
		return "disconnected"
	default:
		return "none"
	}
}

func (s NodeStatus) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// StatusInfo holds connection status details.
type StatusInfo struct {
	// Status is the connection status.
	Status NodeStatus `json:"status"`
	// Message holds extra info, usually the last connection error.
	Message string `json:"message,omitempty"`
}

// ConnectionStatusGetter is an optional status interface for connection-oriented components,
// orthogonal to SharedNode.
type ConnectionStatusGetter interface {
	// ConnectionStatus returns the current status; must be side-effect free (no connection IO).
	ConnectionStatus() StatusInfo
}
