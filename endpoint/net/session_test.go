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

package net

import (
	"net"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
)

// TestEndpointNetSessionExtraction end-to-end verification of endpoint/net session maintenance:
// Device TCP connection → first frame extracts deviceId rewriting Key → second frame holds → disconnect and log out.
// No rule chain is started (routers are empty), only session extraction/addressing is verified (completed before DoProcess).
func TestEndpointNetSessionExtraction(t *testing.T) {
	ep := &Net{}
	cfg := types.Configuration{
		"protocol":   "tcp",
		"server":     ":0", // Random port
		"sessionKey": "${msg.deviceId}",
	}
	if err := ep.Init(engine.NewConfig(), cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ep.Destroy()

	// Take the actual listening address (random port)
	ep.mu.RLock()
	addr := ep.listener.Addr().String()
	ep.mu.RUnlock()

	// Simulating TCP connection to the device
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	// First frame {"deviceId":"DEV_001"} (line mode, ends one frame with \n)
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// Verification: session is registered, Key rewritten to DEV_001, resolved
	sessions := ep.Lookup("DEV_001")
	if len(sessions) != 1 {
		t.Fatalf("after frame1: Lookup(DEV_001) = %d, want 1", len(sessions))
	}
	if !sessions[0].IsResolved() {
		t.Fatal("session should be resolved after first frame")
	}

	// Second frame {"temp":26} (no deviceId): keyResolved=true, Key should remain DEV_001
	if _, err := conn.Write([]byte(`{"temp":26}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if sessions[0].Key() != "DEV_001" {
		t.Fatalf("after frame2: Key = %q, want DEV_001 (should not change)", sessions[0].Key())
	}

	// Addressing by deviceId: Lookup should hit, Sender can send (verifying Sender channel connectivity)
	if got := ep.Lookup("DEV_001"); len(got) != 1 {
		t.Fatalf("addressing lookup = %d, want 1", len(got))
	}

	// Device disconnected→ session logs out
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if got := ep.Lookup("DEV_001"); len(got) != 0 {
		t.Fatalf("after disconnect: Lookup(DEV_001) = %d, want 0 (session should be removed)", len(got))
	}
}

// TestEndpointNetSessionDefaultKey When verification does not match a sessionKey, the default Key is RemoteAddr (IP addressing)
func TestEndpointNetSessionDefaultKey(t *testing.T) {
	ep := &Net{}
	cfg := types.Configuration{
		"protocol": "tcp",
		"server":   ":0",
		// Does not include sessionKey → default is RemoteAddr
	}
	if err := ep.Init(engine.NewConfig(), cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ep.Destroy()

	ep.mu.RLock()
	addr := ep.listener.Addr().String()
	ep.mu.RUnlock()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// Default Key = RemoteAddr(full host:port); Accurate lookup accuracy
	fullAddr := conn.LocalAddr().String()
	if got := ep.Lookup(fullAddr); len(got) != 1 {
		t.Fatalf("Lookup by full RemoteAddr %q = %d, want 1", fullAddr, len(got))
	}
	// No more hits by IP (host segment): IP segment matching has been removed; addressing requires sessionKey to extract stable identifiers
	host, _, _ := net.SplitHostPort(fullAddr)
	if got := ep.Lookup(host); len(got) != 0 {
		t.Fatalf("Lookup by IP %q = %d, want 0 (IP-segment match removed; configure sessionKey for addressing)", host, len(got))
	}
}
