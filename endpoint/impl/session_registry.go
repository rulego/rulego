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
	"io"
	"sync"
	"time"

	"github.com/rulego/rulego/api/types/endpoint"
)

// DefaultSessionRegistry is based on sync.Map's universal SessionRegistry implementation, reusable by server-based endpoints for embedding.
type DefaultSessionRegistry struct {
	sessions sync.Map // key: Session.Key() -> *endpoint.Session

	sweepMu   sync.Mutex
	sweepStop chan struct{} // nil = Not scanned
	sweepDone chan struct{} // goroutine exit signal
}

// Add registers a session, ignoring nil entries.
func (r *DefaultSessionRegistry) Add(s *endpoint.Session) {
	if s == nil {
		return
	}
	r.sessions.Store(s.Key(), s)
}

// Remove to log out the session with Key.
func (r *DefaultSessionRegistry) Remove(key string) {
	r.sessions.Delete(key)
}

// Clear clears all sessions (endpoints are destroyed as a safety net, so concurrent writes should not occur at this time).
func (r *DefaultSessionRegistry) Clear() {
	r.sessions.Range(func(k, _ any) bool {
		r.sessions.Delete(k)
		return true
	})
}

// Rekey rewrites session Keys: delete the old Key, SetKey, and register the new Key. Three steps of encapsulation to prevent the caller from missing Remove.
// Concurrency constraint: There is a brief window between Delete(oldKey) and Store(newKey), during which the concurrent Lookup may not find the session.
// The caller should ensure that the same session is a Rekey serial call (currently, each endpoint handler satisfies this constraint per single goroutine).
func (r *DefaultSessionRegistry) Rekey(s *endpoint.Session, newKey string) {
	if s == nil || newKey == "" {
		return
	}
	oldKey := s.Key()
	s.SetKey(newKey)
	if oldKey != newKey {
		r.sessions.Delete(oldKey)
	}
	r.sessions.Store(newKey, s)
}

// Lookup addresses by target: empty or "*" broadcast; Other exact match keys (the target should be the sessionKey value, such as deviceId).
func (r *DefaultSessionRegistry) Lookup(target string) []*endpoint.Session {
	if target == "" || target == "*" {
		var all []*endpoint.Session
		r.sessions.Range(func(_, v any) bool {
			all = append(all, v.(*endpoint.Session))
			return true
		})
		return all
	}
	var out []*endpoint.Session
	r.sessions.Range(func(_, v any) bool {
		if v.(*endpoint.Session).Key() == target {
			out = append(out, v.(*endpoint.Session))
		}
		return true
	})
	return out
}

// StartSweeping starts background scanning, eliminating sessions with idle exceeding TTL by interval (close the connection to encourage reconnection).
// ttl/interval<=0 or repeated calls as no-op.
func (r *DefaultSessionRegistry) StartSweeping(ttl, interval time.Duration) {
	if ttl <= 0 || interval <= 0 {
		return
	}
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	if r.sweepStop != nil {
		return // Scanning is already underway
	}
	r.sweepStop = make(chan struct{})
	r.sweepDone = make(chan struct{})
	stop, done := r.sweepStop, r.sweepDone
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				r.sweep(time.Now(), ttl)
			case <-stop:
				return
			}
		}
	}()
}

// StopSweeping: Stop scanning and wait for exit, power, etc.
func (r *DefaultSessionRegistry) StopSweeping() {
	r.sweepMu.Lock()
	if r.sweepStop == nil {
		r.sweepMu.Unlock()
		return
	}
	close(r.sweepStop)
	done := r.sweepDone
	r.sweepStop, r.sweepDone = nil, nil
	r.sweepMu.Unlock()
	<-done // Wait for the goroutine to exit and prevent leaks
}

// sweep scans out idle timed sessions, returns the number of eliminations. Now for test injection.
func (r *DefaultSessionRegistry) sweep(now time.Time, ttl time.Duration) int {
	deadline := now.UnixNano() - int64(ttl)
	var evicted int
	r.sessions.Range(func(k, v any) bool {
		s := v.(*endpoint.Session)
		if s.LastSeen() > deadline {
			return true
		}
		// Atomic claims, avoiding double deletion with disconnect defers
		actual, loaded := r.sessions.LoadAndDelete(k)
		if !loaded {
			return true
		}
		ss := actual.(*endpoint.Session)
		// If you have just received a frame in the claim window, fill it in
		if ss.LastSeen() > deadline {
			r.sessions.Store(k, ss)
			return true
		}
		// Rekey to a new key without backfilling the old key
		if ss.Key() != k.(string) {
			return true
		}
		if c, ok := ss.Sender.(io.Closer); ok {
			_ = c.Close()
		}
		evicted++
		return true
	})
	return evicted
}
