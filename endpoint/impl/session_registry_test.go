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
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

func TestDefaultSessionRegistry_AddRemoveLookup(t *testing.T) {
	r := &DefaultSessionRegistry{}
	s1 := endpoint.NewSession("192.168.1.10:1000", nil)
	s2 := endpoint.NewSession("192.168.1.10:1001", nil)
	s3 := endpoint.NewSession("DEV_001", nil)
	r.Add(s1)
	r.Add(s2)
	r.Add(s3)
	r.Add(nil) // nil tolerated

	// Precisely match keys
	if got := r.Lookup("DEV_001"); len(got) != 1 || got[0] != s3 {
		t.Fatalf("exact lookup DEV_001 = %v, want [s3]", got)
	}
	// Exact matching of IP:port (no longer matching the host segment)
	if got := r.Lookup("192.168.1.10:1000"); len(got) != 1 || got[0] != s1 {
		t.Fatalf("exact lookup 192.168.1.10:1000 = %v, want [s1]", got)
	}
	// Bare IPs no longer hit host:port connections (implicit matching of ipOf removed)
	if got := r.Lookup("192.168.1.10"); len(got) != 0 {
		t.Fatalf("bare ip should not match host:port keys, got %d", len(got))
	}
	// Broadcast *
	if got := r.Lookup("*"); len(got) != 3 {
		t.Fatalf("broadcast * = %d, want 3", len(got))
	}
	// Broadcast empty
	if got := r.Lookup(""); len(got) != 3 {
		t.Fatalf("broadcast empty = %d, want 3", len(got))
	}
	// No matching
	if got := r.Lookup("UNKNOWN"); len(got) != 0 {
		t.Fatalf("no-match = %d, want 0", len(got))
	}
	// After Removal, it cannot be found
	r.Remove("DEV_001")
	if got := r.Lookup("DEV_001"); len(got) != 0 {
		t.Fatalf("after remove = %d, want 0", len(got))
	}
	// Remove a nonexistent key and don't panic
	r.Remove("NOT_EXIST")
}

func TestSessionSetKey(t *testing.T) {
	s := endpoint.NewSession("OLD", nil)
	if s.Key() != "OLD" || s.IsResolved() {
		t.Fatal("initial state wrong")
	}
	s.SetKey("NEW")
	if s.Key() != "NEW" || !s.IsResolved() {
		t.Fatalf("after SetKey: Key=%q resolved=%v", s.Key(), s.IsResolved())
	}
	// Empty keys ignored (no clearing, no incorrect marking)
	s.SetKey("")
	if s.Key() != "NEW" || !s.IsResolved() {
		t.Fatalf("empty key ignored: Key=%q resolved=%v", s.Key(), s.IsResolved())
	}
}

func TestRekey(t *testing.T) {
	r := &DefaultSessionRegistry{}
	s := endpoint.NewSession("OLD", nil)
	r.Add(s)

	r.Rekey(s, "NEW")
	if s.Key() != "NEW" || !s.IsResolved() {
		t.Fatalf("after Rekey: Key=%q resolved=%v", s.Key(), s.IsResolved())
	}
	if got := r.Lookup("NEW"); len(got) != 1 || got[0] != s {
		t.Fatalf("lookup NEW = %v, want [s]", got)
	}
	if got := r.Lookup("OLD"); len(got) != 0 {
		t.Fatalf("old key should be deregistered, got %d", len(got))
	}
	// Rekey empty keys are ignored
	r.Rekey(s, "")
	if s.Key() != "NEW" {
		t.Fatalf("Rekey empty should be ignored, got %q", s.Key())
	}
	// Rekey nil sessions don't panic
	r.Rekey(nil, "X")
}

// closerSender tests with a Sender and records the number of Close calls.
type closerSender struct {
	closed int
}

func (s *closerSender) Send(data []byte) error { return nil }
func (s *closerSender) Close() error           { s.closed++; return nil }

// nonCloserSender only implements Send (not io.Closer), verifying the graceful downgrade of TTL sweep.
type nonCloserSender struct{}

func (s *nonCloserSender) Send(data []byte) error { return nil }

func TestSweep_EvictsExpired(t *testing.T) {
	r := &DefaultSessionRegistry{}
	cs := &closerSender{}
	r.Add(endpoint.NewSession("k1", cs))
	evicted := r.sweep(time.Now().Add(time.Hour), time.Second)
	if evicted != 1 {
		t.Fatalf("evicted=%d, want 1", evicted)
	}
	if len(r.Lookup("*")) != 0 {
		t.Fatal("registry should be empty after sweep")
	}
	if cs.closed != 1 {
		t.Fatalf("Close not called once, got %d", cs.closed)
	}
}

func TestSweep_KeepsActive(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.Add(endpoint.NewSession("k1", &closerSender{}))
	evicted := r.sweep(time.Now(), time.Hour)
	if evicted != 0 {
		t.Fatalf("evicted=%d, want 0", evicted)
	}
	if len(r.Lookup("*")) != 1 {
		t.Fatal("active session should remain")
	}
}

func TestSweep_MixedAges(t *testing.T) {
	r := &DefaultSessionRegistry{}
	old := endpoint.NewSession("old", &closerSender{})
	old.TouchAt(time.Now().Add(-time.Hour))
	r.Add(old)
	r.Add(endpoint.NewSession("fresh", &closerSender{}))
	evicted := r.sweep(time.Now(), 30*time.Second)
	if evicted != 1 {
		t.Fatalf("evicted=%d, want 1 (only old)", evicted)
	}
	all := r.Lookup("*")
	if len(all) != 1 || all[0].Key() != "fresh" {
		t.Fatalf("should keep only fresh, got %v", all)
	}
}

func TestSweep_NonCloserSender(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.Add(endpoint.NewSession("k1", &nonCloserSender{}))
	evicted := r.sweep(time.Now().Add(time.Hour), time.Second)
	if evicted != 1 {
		t.Fatalf("evicted=%d, want 1 (Remove without Close)", evicted)
	}
	if len(r.Lookup("*")) != 0 {
		t.Fatal("registry should be empty")
	}
}

func TestStartSweeping_StopsCleanly(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.StartSweeping(10*time.Millisecond, 5*time.Millisecond)
	r.StartSweeping(10*time.Millisecond, 5*time.Millisecond) // Repeat the no-op
	r.StopSweeping()
	r.StopSweeping() // Power equal
}

func TestStartSweeping_Disabled(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.StartSweeping(0, 0) // ttl<=0 no-op
	r.StopSweeping()
}

func TestSession_TouchUpdatesLastSeen(t *testing.T) {
	s := endpoint.NewSession("k1", &closerSender{})
	old := s.LastSeen()
	time.Sleep(2 * time.Millisecond)
	s.Touch()
	if s.LastSeen() <= old {
		t.Fatal("Touch should advance lastSeen")
	}
}

// fakeSender records received data frames to verify addressed pushes
type fakeSender struct {
	received [][]byte
}

func (f *fakeSender) Send(data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	f.received = append(f.received, cp)
	return nil
}

// TestSessionKeyExtractionFlow simulates end-to-end sessionKey extraction and addressing push processes,
// Do not start real TCP; verify the collaboration between DefaultSessionRegistry + SessionKeyResolver + Session.
//
// Process:
//  1. Connection established: session initial Key=RemoteAddr, not resolved
//  2. First frame {"deviceId":"DEV_001"}: extract key → delete the old key → SetKey → register the new key
//  3. Second frame {"temp":26} (no deviceId): IsResolved=true skips extraction, key retains
//  4. Business pushes addressed by deviceId: Lookup("DEV_001") → Send
//  5. Broadcast: Lookup("*") → Send
func TestSessionKeyExtractionFlow(t *testing.T) {
	registry := &DefaultSessionRegistry{}
	resolver := NewSessionKeyResolver("${msg.deviceId}")

	// (1) Connection establishment
	sender := &fakeSender{}
	session := endpoint.NewSession("192.168.1.10:5000", sender)
	registry.Add(session)
	if session.IsResolved() {
		t.Fatal("new session should not be resolved")
	}

	// (2) First frame extraction
	firstFrame := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"deviceId":"DEV_001"}`)
	if !session.IsResolved() {
		key := resolver.Resolve(firstFrame, nil)
		if key != "DEV_001" {
			t.Fatalf("resolve got %q, want DEV_001", key)
		}
		registry.Rekey(session, key) // Atomic key change: Deregister the old Key + SetKey + register the new Key
	}

	if !session.IsResolved() {
		t.Fatal("should be resolved after first frame")
	}
	if session.Key() != "DEV_001" {
		t.Fatalf("session.Key = %q, want DEV_001", session.Key())
	}
	if got := registry.Lookup("DEV_001"); len(got) != 1 || got[0] != session {
		t.Fatalf("lookup DEV_001 = %v, want [session]", got)
	}
	if got := registry.Lookup("192.168.1.10:5000"); len(got) != 0 {
		t.Fatalf("old key should be deregistered, got %d", len(got))
	}

	// (3) Second frame: IsResolved=true, handler should skip extraction (Resolve is not called here)
	if !session.IsResolved() {
		t.Fatal("expected to skip extraction when resolved")
	}

	// (4) Push by deviceId addressing
	targets := registry.Lookup("DEV_001")
	if len(targets) != 1 {
		t.Fatalf("lookup for push = %d, want 1", len(targets))
	}
	if err := targets[0].Sender.Send([]byte("HELLO_DEVICE")); err != nil {
		t.Fatalf("send err: %v", err)
	}
	if len(sender.received) != 1 || string(sender.received[0]) != "HELLO_DEVICE" {
		t.Fatalf("sender received %v, want [HELLO_DEVICE]", sender.received)
	}

	// (5) Broadcasting
	for _, s := range registry.Lookup("*") {
		if err := s.Sender.Send([]byte("BROADCAST")); err != nil {
			t.Fatalf("broadcast send err: %v", err)
		}
	}
	if len(sender.received) != 2 || string(sender.received[1]) != "BROADCAST" {
		t.Fatalf("after broadcast received %v, want 2 frames", sender.received)
	}

	// (6) Device disconnection: Cancel the session
	registry.Remove(session.Key())
	if got := registry.Lookup("DEV_001"); len(got) != 0 {
		t.Fatalf("after disconnect should be removed, got %d", len(got))
	}
}

// TestSessionKeyExtractionMultiCandidateFlow verifies the extraction process for multiple candidate scenarios
// (Private hex protocol device: extracting from bytes when JSON has no fields)
func TestSessionKeyExtractionMultiCandidateFlow(t *testing.T) {
	registry := &DefaultSessionRegistry{}
	resolver := NewSessionKeyResolver([]string{"${msg.deviceId}", "${data[0:6]}"})

	session := endpoint.NewSession("10.0.0.1:9", &fakeSender{})
	registry.Add(session)

	// Private protocol frames: JSON parsing failed / no deviceId → revert offset to take the first 6 bytes
	data := []byte("SN0001") // offset:0,len:6
	key := resolver.Resolve(jsonMsg("{}"), data)
	if key != "SN0001" {
		t.Fatalf("got %q, want SN0001", key)
	}
	registry.Rekey(session, key)

	if got := registry.Lookup("SN0001"); len(got) != 1 {
		t.Fatalf("lookup SN0001 = %d, want 1", len(got))
	}
}
