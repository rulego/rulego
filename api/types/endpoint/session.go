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

// Session addressing abstraction for server-based endpoints. The connected connection was originally only within the handler's local scope and could not be addressed across requests;
// Here, a Session registry is provided, allowing the business side to proactively push to a specified client by pressing a Key.
//
// Concurrency model: Key is protected by RWMutex within Key()/SetKey(); Sender remains unchanged throughout the connection lifecycle, with concurrency security guaranteed by the implementer;
// Rewriting the key of a registered session should be done via SessionRegistry.Rekey (while updating the index).
package endpoint

import (
	"sync"
	"sync/atomic"
	"time"
)

// Sender protocol-independent send channels, implemented by each endpoint.
type Sender interface {
	// Send a frame of data, and the implementer must ensure concurrency safety.
	Send(data []byte) error
}

// Session represents a client-side session that is secure concurrently.
type Session struct {
	// Sender send channel, which remains unchanged throughout the connection lifecycle.
	Sender Sender

	mu          sync.RWMutex
	key         string
	keyResolved bool // Is the sessionKey confirmed (no changes after extracting the first frame)

	lastSeen int64 // Recent Active Time (UnixNano), atomic protection, Touch refresh per frame, TTL scans are eliminated accordingly
}

// NewSession creates a session, with the initial key usually being RemoteAddr.
func NewSession(key string, sender Sender) *Session {
	s := &Session{Sender: sender, key: key}
	s.Touch() // Construct lastSeen
	return s
}

// Touch refreshes lastSeen to the current time. Each frame call (including heartbeat frames) is used for TTL keep-alive.
func (s *Session) Touch() { atomic.StoreInt64(&s.lastSeen, time.Now().UnixNano()) }

// TouchAt refreshes lastSeen in explicit time. Testing injects clocks to construct mixed-age scenarios.
func (s *Session) TouchAt(now time.Time) { atomic.StoreInt64(&s.lastSeen, now.UnixNano()) }

// LastSeen returns the most recent active time (UnixNano). TTL scanning is used.
func (s *Session) LastSeen() int64 { return atomic.LoadInt64(&s.lastSeen) }

// Key returns the current address key.
func (s *Session) Key() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.key
}

// IsResolved returns whether the sessionKey is determined.
func (s *Session) IsResolved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keyResolved
}

// SetKey updates the key and marks it as confirmed, while the empty key is ignored.
// Only change fields, do not update the registry index; Change the registered session app to Rekey.
func (s *Session) SetKey(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	s.keyResolved = true
}

// SessionRegistry is a session registry implemented by a server-based endpoint.
//
// Lookup addresses by target:
//   - "*" or empty: Broadcast, returns all sessions
//   - Other: Exact match key (target should be the value extracted from sessionKey, e.g., deviceId)
type SessionRegistry interface {
	Add(*Session)
	Remove(key string)
	// Rekey rewrites session Keys: delete the old Key, SetKey, and register the new Key.
	Rekey(s *Session, newKey string)
	Lookup(target string) []*Session
}
