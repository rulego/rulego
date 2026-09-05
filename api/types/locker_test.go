/*
 * Copyright 2023 The RuleGo Authors.
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

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalLocker(t *testing.T) {
	locker := NewLocalLocker()
	ctx := context.Background()

	token1, ok, err := locker.TryLock(ctx, "k1", time.Minute)
	if err != nil || !ok || token1 == "" {
		t.Fatalf("first TryLock should succeed, token=%s ok=%v err=%v", token1, ok, err)
	}
	if _, ok, _ := locker.TryLock(ctx, "k1", time.Minute); ok {
		t.Fatal("second TryLock on the same key should fail while held")
	}
	if _, ok, _ := locker.TryLock(ctx, "k2", time.Minute); !ok {
		t.Fatal("TryLock on another key should succeed")
	}

	if err := locker.Unlock(ctx, "k1", "wrong-token"); !errors.Is(err, ErrLockNotHeld) {
		t.Fatalf("Unlock with wrong token should return ErrLockNotHeld, got %v", err)
	}
	if err := locker.Unlock(ctx, "k1", token1); err != nil {
		t.Fatalf("Unlock with matching token should succeed, got %v", err)
	}
	if token2, ok, _ := locker.TryLock(ctx, "k1", time.Minute); !ok || token2 == token1 {
		t.Fatal("key should be re-acquirable after unlock with a new token")
	}
}

func TestLocalLockerExpiration(t *testing.T) {
	locker := NewLocalLocker()
	ctx := context.Background()

	_, ok, _ := locker.TryLock(ctx, "k1", 30*time.Millisecond)
	if !ok {
		t.Fatal("initial TryLock should succeed")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok, _ := locker.TryLock(ctx, "k1", time.Minute); !ok {
		t.Fatal("expired lock should be acquirable by the next holder")
	}
}

func TestLocalLockerLockWithRetry(t *testing.T) {
	locker := NewLocalLocker()
	ctx := context.Background()

	token, err := locker.LockWithRetry(ctx, "k1", time.Minute, 5*time.Millisecond, 3)
	if err != nil || token == "" {
		t.Fatalf("LockWithRetry on a free key should succeed, got %v", err)
	}

	_, err = locker.LockWithRetry(ctx, "k1", time.Minute, 5*time.Millisecond, 2)
	if err == nil {
		t.Fatal("LockWithRetry on a held key should fail after retries are exhausted")
	}
}

// stubLocker records every TryLock key and makes the backend fail on demand.
type stubLocker struct {
	held  map[string]string
	order []string
	ttl   time.Duration
	fail  bool
}

func (s *stubLocker) Lock(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return "", errors.New("not implemented")
}

func (s *stubLocker) Unlock(ctx context.Context, key, token string) error {
	delete(s.held, key)
	return nil
}

func (s *stubLocker) TryLock(ctx context.Context, key string, expiration time.Duration) (string, bool, error) {
	if s.fail {
		return "", false, errors.New("backend unavailable")
	}
	s.order = append(s.order, key)
	s.ttl = expiration
	if _, held := s.held[key]; held {
		return "", false, nil
	}
	if s.held == nil {
		s.held = make(map[string]string)
	}
	s.held[key] = "token"
	return "token", true, nil
}

func (s *stubLocker) LockWithRetry(ctx context.Context, key string, expiration time.Duration, retryInterval time.Duration, maxRetries int) (string, error) {
	return "", errors.New("not implemented")
}

func TestOnceGuardWithoutLocker(t *testing.T) {
	guard := NewOnceGuard(NewConfig(), "schedule:o1:ep1:r1")
	if !guard.Allow(context.Background(), "1000") || !guard.Allow(context.Background(), "1000") {
		t.Fatal("guard without a Locker should always allow")
	}
}

func TestOnceGuardDedup(t *testing.T) {
	locker := &stubLocker{}
	config := NewConfig(WithLocker(locker))
	guard := NewOnceGuard(config, "schedule:o1:ep1:r1")

	if !guard.Allow(context.Background(), "1000") {
		t.Fatal("first Allow on a fresh key should pass")
	}
	if guard.Allow(context.Background(), "1000") {
		t.Fatal("second Allow on the same key should be rejected")
	}
	if !guard.Allow(context.Background(), "1001") {
		t.Fatal("a different key should not be affected")
	}

	want := onceLockKeyPrefix + "schedule:o1:ep1:r1:1000"
	if len(locker.order) == 0 || locker.order[0] != want {
		t.Fatalf("lock key should be %s, got %v", want, locker.order)
	}
}

func TestOnceGuardScopeIsolation(t *testing.T) {
	locker := &stubLocker{}
	config := NewConfig(WithLocker(locker))
	guardA := NewOnceGuard(config, "schedule:tenantA:ep1:r1")
	guardB := NewOnceGuard(config, "schedule:tenantB:ep1:r1")

	if !guardA.Allow(context.Background(), "1000") {
		t.Fatal("first Allow should pass")
	}
	if !guardB.Allow(context.Background(), "1000") {
		t.Fatal("a different scope must not be blocked by another scope's key")
	}
}

func TestOnceGuardBackendFailure(t *testing.T) {
	locker := &stubLocker{fail: true}
	config := NewConfig(WithLocker(locker))

	closed := NewOnceGuard(config, "schedule:o1:ep1:r1")
	if closed.Allow(context.Background(), "1000") {
		t.Fatal("default policy should skip the action on backend failure")
	}

	open := NewOnceGuard(config, "schedule:o1:ep1:r1", WithGuardFailOpen())
	if !open.Allow(context.Background(), "1000") {
		t.Fatal("WithGuardFailOpen should let the action run on backend failure")
	}
}

func TestLocalLockerConcurrent(t *testing.T) {
	locker := NewLocalLocker()
	const goroutines = 50
	var success int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, ok, _ := locker.TryLock(context.Background(), "same-key", time.Minute); ok {
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if success != 1 {
		t.Fatalf("exactly one goroutine should acquire the lock, got %d", success)
	}
}

func TestLocalLockerLockBlocksUntilReleased(t *testing.T) {
	locker := NewLocalLocker()
	ctx := context.Background()

	token, ok, _ := locker.TryLock(ctx, "k1", time.Minute)
	if !ok {
		t.Fatal("initial TryLock should succeed")
	}
	acquired := make(chan string, 1)
	go func() {
		token, err := locker.Lock(ctx, "k1", time.Minute)
		if err != nil {
			t.Errorf("Lock should succeed after release: %v", err)
			return
		}
		acquired <- token
	}()
	select {
	case <-acquired:
		t.Fatal("Lock should block while the key is held")
	case <-time.After(150 * time.Millisecond):
	}
	if err := locker.Unlock(ctx, "k1", token); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("Lock should return after the key is released")
	}
}

func TestLocalLockerSweep(t *testing.T) {
	old := localLockSweepInterval
	localLockSweepInterval = 20 * time.Millisecond
	defer func() { localLockSweepInterval = old }()

	locker := NewLocalLocker()
	for i := 0; i < 100; i++ {
		_, _, _ = locker.TryLock(context.Background(), fmt.Sprintf("k%d", i), 30*time.Millisecond)
	}
	if len(locker.locks) != 100 {
		t.Fatalf("expected 100 entries before sweep, got %d", len(locker.locks))
	}
	time.Sleep(60 * time.Millisecond)
	// 任意一次加锁触发周期性清扫
	if _, ok, _ := locker.TryLock(context.Background(), "trigger", time.Minute); !ok {
		t.Fatal("trigger TryLock should succeed")
	}
	if len(locker.locks) != 1 {
		t.Fatalf("expired entries should be swept, expected 1 entry, got %d", len(locker.locks))
	}
}

func TestLocalLockerLockWithRetryAttempts(t *testing.T) {
	locker := NewLocalLocker()
	ctx := context.Background()

	// maxRetries=0 表示只尝试一次：空闲键成功
	if _, err := locker.LockWithRetry(ctx, "free", time.Minute, time.Millisecond, 0); err != nil {
		t.Fatalf("single attempt on a free key should succeed: %v", err)
	}
	// 被持有的键一次尝试必然失败，且不做多余等待
	start := time.Now()
	if _, err := locker.LockWithRetry(ctx, "free", time.Minute, 50*time.Millisecond, 0); err == nil {
		t.Fatal("single attempt on a held key should fail")
	}
	if elapsed := time.Since(start); elapsed > 30*time.Millisecond {
		t.Fatalf("maxRetries=0 should not sleep between attempts, elapsed %v", elapsed)
	}
}

func TestOnceGuardTTLOption(t *testing.T) {
	locker := &stubLocker{}
	config := NewConfig(WithLocker(locker))
	guard := NewOnceGuard(config, "schedule:o1:ep1:r1", WithGuardTTL(5*time.Minute))

	if !guard.Allow(context.Background(), "1000") {
		t.Fatal("first Allow should pass")
	}
	if locker.ttl != 5*time.Minute {
		t.Fatalf("custom TTL should reach the Locker, want 5m got %v", locker.ttl)
	}
}
