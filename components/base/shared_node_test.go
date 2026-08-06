/*
 * Copyright 2026 The RuleGo Authors.
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

package base

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
)

// TestSharedNodeInitFailFast verifies fast-fail within the cooldown, retry after the window, and clearing on success.
func TestSharedNodeInitFailFast(t *testing.T) {
	var calls int32
	dialErr := errors.New("dial: connection refused")
	x := &SharedNode[string]{InitFailRetryInterval: 100 * time.Millisecond}
	_ = x.InitWithClose(types.Config{}, "testNode", "resource1", false, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", dialErr
	}, nil)

	// first call triggers init and fails
	if _, err := x.GetSafely(); err != dialErr {
		t.Fatalf("first call: expected %v, got %v", dialErr, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls after first failure: got %d, want 1", got)
	}

	// subsequent calls within the window fast-fail without retrying init
	for i := 0; i < 10; i++ {
		if _, err := x.GetSafely(); err != dialErr {
			t.Fatalf("fast-fail call: expected %v, got %v", dialErr, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("init retried within cooldown: calls=%d", got)
	}

	// retry is allowed after the window
	time.Sleep(120 * time.Millisecond)
	if _, err := x.GetSafely(); err != dialErr {
		t.Fatalf("cooldown retry: expected %v, got %v", dialErr, err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls after cooldown retry: got %d, want 2", got)
	}

	// a successful retry clears the failure; later calls return the cached instance
	x.Locker.Lock()
	x.InitInstanceFunc = func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "client-ok", nil
	}
	x.Locker.Unlock()
	time.Sleep(120 * time.Millisecond)
	if v, err := x.GetSafely(); err != nil || v != "client-ok" {
		t.Fatalf("after success: v=%q err=%v", v, err)
	}
	if v, err := x.GetSafely(); err != nil || v != "client-ok" {
		t.Fatalf("cached call: v=%q err=%v", v, err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls after success: got %d, want 3", got)
	}
}

// TestSharedNodeInitFailFastConcurrent verifies only one init retry within the cooldown under concurrent callers.
func TestSharedNodeInitFailFastConcurrent(t *testing.T) {
	var calls int32
	x := &SharedNode[string]{InitFailRetryInterval: time.Second}
	_ = x.InitWithClose(types.Config{}, "testNode", "resource2", false, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return "", errors.New("endpoint down")
	}, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = x.GetSafely()
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("init called %d times within cooldown, want 1", got)
	}
}

// TestSharedNodeInitFailClearedByClose verifies Close clears the failure record and allows immediate reinit.
func TestSharedNodeInitFailClearedByClose(t *testing.T) {
	var calls int32
	x := &SharedNode[int]{InitFailRetryInterval: time.Hour}
	_ = x.InitWithClose(types.Config{}, "testNode", "resource3", false, func() (int, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return 0, errors.New("first fail")
		}
		return 42, nil
	}, nil)

	if _, err := x.GetSafely(); err == nil {
		t.Fatal("expected first init error")
	}
	// fast-fail expected within the window; Close clears the record and allows an immediate retry
	_ = x.Close()
	v, err := x.GetSafely()
	if err != nil || v != 42 {
		t.Fatalf("after Close: v=%d err=%v", v, err)
	}
}

// TestSharedNodeSetStatusFromInitFunc is a regression guard: GetSafely calls
// InitInstanceFunc while holding x.Locker; if that callback also calls SetStatus
// (as NetNode.initConnect does via setDisconnected), a SetStatus that takes
// x.Locker would self-deadlock. This test catches that with a short timeout.
func TestSharedNodeSetStatusFromInitFunc(t *testing.T) {
	x := &SharedNode[string]{}
	_ = x.InitWithClose(types.Config{}, "testNode", "local-resource", false, func() (string, error) {
		// Sync status from within the init callback, mirroring NetNode.initConnect.
		x.SetStatus(types.StatusConnected, "dial ok")
		return "client", nil
	}, nil)

	done := make(chan struct{})
	go func() {
		// GetSafely holds x.Locker while invoking the InitInstanceFunc callback above.
		_, _ = x.GetSafely()
		close(done)
	}()
	select {
	case <-done:
		// succeeded: no deadlock
	case <-time.After(3 * time.Second):
		t.Fatal("GetSafely deadlocked: SetStatus inside InitInstanceFunc blocked for over 3s")
	}
	if info := x.ConnectionStatus(); info.Status != types.StatusConnected {
		t.Fatalf("status=%s, want connected", info.Status)
	}
}
