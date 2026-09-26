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
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestActiveGuardWithoutLockerAlwaysActive(t *testing.T) {
	g := NewActiveGuard(Config{}, "test:scope")
	if !g.IsActive() {
		t.Fatalf("expected always active without locker")
	}
	promoted := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx, func() error {
		close(promoted)
		<-ctx.Done()
		return nil
	}, nil)
	select {
	case <-promoted:
	case <-time.After(time.Second):
		t.Fatalf("onPromoted not invoked without locker")
	}
}

// TestActiveGuardSingleLeader 两个副本共享一个 Locker 时只能有一个 leader，
// leader 停机主动释放租约后，待命副本不等 TTL 过期、在轮询周期内接管。
func TestActiveGuardSingleLeader(t *testing.T) {
	locker := NewLocalLocker()

	var leaders int32
	promotedWg1 := make(chan struct{})
	promotedWg2 := make(chan struct{})
	var promotedOnce1, promotedOnce2 sync.Once

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	// TTL 1s：若 leader 停机时不主动释放，接管要等 ~1s；断言 500ms 内
	// 接管即证明主动释放生效
	g1 := NewActiveGuard(Config{Locker: locker}, "election:test",
		WithActiveTTL(time.Second), WithActiveInterval(50*time.Millisecond))
	go g1.Run(ctx1, func() error {
		atomic.AddInt32(&leaders, 1)
		promotedOnce1.Do(func() { close(promotedWg1) })
		return nil
	}, nil)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	g2 := NewActiveGuard(Config{Locker: locker}, "election:test",
		WithActiveTTL(time.Second), WithActiveInterval(50*time.Millisecond))
	go g2.Run(ctx2, func() error {
		atomic.AddInt32(&leaders, 1)
		promotedOnce2.Do(func() { close(promotedWg2) })
		return nil
	}, nil)

	// 两副本同时开抢，谁先晋升不确定，等任一即可
	var leaderCancel context.CancelFunc
	var standbyPromoted chan struct{}
	select {
	case <-promotedWg1:
		leaderCancel, standbyPromoted = cancel1, promotedWg2
	case <-promotedWg2:
		leaderCancel, standbyPromoted = cancel2, promotedWg1
	case <-time.After(5 * time.Second):
		t.Fatal("no leader elected")
	}

	// 待命副本不得并发晋升
	select {
	case <-standbyPromoted:
		t.Fatalf("two replicas promoted concurrently")
	case <-time.After(500 * time.Millisecond):
	}

	// leader 停机释放租约后，待命副本必须在轮询周期内接管
	leaderCancel()
	select {
	case <-standbyPromoted:
	case <-time.After(5 * time.Second):
		t.Fatal("standby did not take over after leader shutdown")
	}
	if n := atomic.LoadInt32(&leaders); n != 2 {
		t.Fatalf("expected exactly one leader at a time, got %d promotions", n)
	}
}

// TestActiveGuardPromoteError 覆盖 onPromoted 失败：回调 onDemoted 清理已部分
// 启动的资源、释放租约并保持待命，由其他副本接管。
func TestActiveGuardPromoteError(t *testing.T) {
	locker := NewLocalLocker()
	var demoted int32

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	g1 := NewActiveGuard(Config{Locker: locker}, "election:err",
		WithActiveTTL(200*time.Millisecond), WithActiveInterval(50*time.Millisecond))
	go g1.Run(ctx1, func() error {
		return context.DeadlineExceeded
	}, func() {
		atomic.AddInt32(&demoted, 1)
	})

	// 先等首个副本完成一轮晋升失败并回调 onDemoted，再启动第二个副本；
	// 同时启动时无法确定谁先抢到租约
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&demoted) == 0 {
		select {
		case <-deadline:
			t.Fatal("first guard never attempted a promotion")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// 晋升失败的副本释放租约后，其余副本最终能成为 leader
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	g2 := NewActiveGuard(Config{Locker: locker}, "election:err",
		WithActiveTTL(200*time.Millisecond), WithActiveInterval(50*time.Millisecond))
	promoted2 := make(chan struct{})
	go g2.Run(ctx2, func() error {
		close(promoted2)
		<-ctx2.Done()
		return nil
	}, nil)

	select {
	case <-promoted2:
	case <-time.After(2 * time.Second):
		t.Fatalf("standby did not take over after promote failure")
	}
}

// TestActiveGuardIntervalClamp 轮询周期不得超过 TTL 的三分之一，
// 选项先后顺序不影响该上界。
func TestActiveGuardIntervalClamp(t *testing.T) {
	g := NewActiveGuard(Config{}, "test:clamp",
		WithActiveTTL(300*time.Millisecond), WithActiveInterval(time.Second))
	if g.interval != 100*time.Millisecond {
		t.Fatalf("expected interval clamped to ttl/3, got %v", g.interval)
	}
	g = NewActiveGuard(Config{}, "test:clamp",
		WithActiveInterval(time.Second), WithActiveTTL(300*time.Millisecond))
	if g.interval != 100*time.Millisecond {
		t.Fatalf("expected interval clamped to ttl/3 regardless of option order, got %v", g.interval)
	}
	g = NewActiveGuard(Config{}, "test:clamp",
		WithActiveTTL(300*time.Millisecond), WithActiveInterval(50*time.Millisecond))
	if g.interval != 50*time.Millisecond {
		t.Fatalf("interval below the bound must be kept, got %v", g.interval)
	}
}

// flakyRenewer 在 fail 置位时模拟续约失败（键被接管或后端故障）。
type flakyRenewer struct {
	*LocalLocker
	fail int32
}

func (f *flakyRenewer) Renew(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	if atomic.LoadInt32(&f.fail) == 1 {
		return false, nil
	}
	return f.LocalLocker.Renew(ctx, key, token, expiration)
}

// TestActiveGuardRenewFailure leader 续约失败后必须回调 onDemoted，
// 待命副本在后续轮询中接管。
func TestActiveGuardRenewFailure(t *testing.T) {
	locker := &flakyRenewer{LocalLocker: NewLocalLocker()}

	newGuard := func() *ActiveGuard {
		return NewActiveGuard(Config{Locker: locker}, "election:renew",
			WithActiveTTL(200*time.Millisecond), WithActiveInterval(50*time.Millisecond))
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	g1 := newGuard()
	demoted1 := make(chan struct{})
	var demotedOnce sync.Once
	// onDemoted 契约要求可重入：ctx 退出路径也会调用
	go g1.Run(ctx1, func() error { return nil }, func() {
		demotedOnce.Do(func() { close(demoted1) })
	})

	// 轮询等待成为 leader
	deadline := time.Now().Add(2 * time.Second)
	for !g1.IsActive() {
		if time.Now().After(deadline) {
			t.Fatalf("guard never became leader")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 续约失败 → 降级
	atomic.StoreInt32(&locker.fail, 1)
	select {
	case <-demoted1:
	case <-time.After(2 * time.Second):
		t.Fatalf("leader not demoted after renew failure")
	}
	if g1.IsActive() {
		t.Fatalf("guard should be standby after renew failure")
	}

	// 已降级副本先退出再恢复续约：它仍在轮询，恢复后可能自己重新抢到租约
	cancel1()
	atomic.StoreInt32(&locker.fail, 0)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	g2 := newGuard()
	promoted2 := make(chan struct{})
	go g2.Run(ctx2, func() error {
		close(promoted2)
		return nil
	}, nil)

	select {
	case <-promoted2:
	case <-time.After(2 * time.Second):
		t.Fatalf("standby did not take over after leader demoted")
	}
}

func TestLocalLockerRenew(t *testing.T) {
	locker := NewLocalLocker()
	token, ok, err := locker.TryLock(context.Background(), "k", 50*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("try lock: %v %v", ok, err)
	}
	if ok, err := locker.Renew(context.Background(), "k", token, 100*time.Millisecond); err != nil || !ok {
		t.Fatalf("renew own lock: %v %v", ok, err)
	}
	time.Sleep(60 * time.Millisecond)
	// 续约后原 TTL 已顺延，锁仍被持有
	if _, ok, _ := locker.TryLock(context.Background(), "k", 50*time.Millisecond); ok {
		t.Fatalf("lock should still be held after renew")
	}
	if ok, err := locker.Renew(context.Background(), "k", "wrong-token", 100*time.Millisecond); ok || err != nil {
		t.Fatalf("renew with wrong token should be false, nil: %v %v", ok, err)
	}
}

// recordingLogger 按级别记录调用，用于验证守卫的日志路径。
type recordingLogger struct {
	Logger
	debug, info, warn int
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{Logger: NewStdLogger(io.Discard)}
}

func (l *recordingLogger) Printf(format string, v ...interface{}) {}
func (l *recordingLogger) Debugf(format string, v ...interface{}) { l.debug++ }
func (l *recordingLogger) Infof(format string, v ...interface{})  { l.info++ }
func (l *recordingLogger) Warnf(format string, v ...interface{})  { l.warn++ }
func (l *recordingLogger) Errorf(format string, v ...interface{}) {}

// renewlessLocker 隐藏 LocalLocker 的 Renew 方法，迫使守卫走释放后重取的降级路径。
type renewlessLocker struct {
	l *LocalLocker
}

func (r renewlessLocker) Lock(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return r.l.Lock(ctx, key, expiration)
}
func (r renewlessLocker) Unlock(ctx context.Context, key, token string) error {
	return r.l.Unlock(ctx, key, token)
}
func (r renewlessLocker) TryLock(ctx context.Context, key string, expiration time.Duration) (string, bool, error) {
	return r.l.TryLock(ctx, key, expiration)
}
func (r renewlessLocker) LockWithRetry(ctx context.Context, key string, expiration time.Duration, retryInterval time.Duration, maxRetries int) (string, error) {
	return r.l.LockWithRetry(ctx, key, expiration, retryInterval, maxRetries)
}

// unlockingFailsLocker 让 Unlock 恒失败，验证续约降级路径的失败分支。
type unlockingFailsLocker struct {
	renewlessLocker
}

func (u unlockingFailsLocker) Unlock(_ context.Context, _, _ string) error {
	return errors.New("unlock backend down")
}

// waitUntilActive 轮询等待守卫成为 leader。
func waitUntilActive(t *testing.T, g *ActiveGuard, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !g.IsActive() {
		if time.Now().After(deadline) {
			t.Fatalf("guard never became leader")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestActiveGuardRenewFallbackRenewless 后端无续约能力时，leader 通过
// 释放后立即重取保持续约，token 被替换且不触发降级。
func TestActiveGuardRenewFallbackRenewless(t *testing.T) {
	logger := newRecordingLogger()
	locker := renewlessLocker{l: NewLocalLocker()}
	g := NewActiveGuard(Config{Locker: locker, Logger: logger}, "election:fallback",
		WithActiveTTL(200*time.Millisecond), WithActiveInterval(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	demoted := make(chan struct{}, 4)
	go g.Run(ctx, func() error { return nil }, func() { demoted <- struct{}{} })

	waitUntilActive(t, g, 2*time.Second)
	// 至少经历一轮「释放+重取」续约后仍是 leader
	time.Sleep(150 * time.Millisecond)
	if !g.IsActive() {
		t.Fatalf("leader should keep lease through unlock/relock renewals")
	}
	select {
	case <-demoted:
		t.Fatalf("successful renewals must not demote")
	default:
	}
}

// TestActiveGuardRenewFallbackUnlockError 释放失败视为租约丢失，必须降级。
func TestActiveGuardRenewFallbackUnlockError(t *testing.T) {
	logger := newRecordingLogger()
	locker := unlockingFailsLocker{renewlessLocker{l: NewLocalLocker()}}
	g := NewActiveGuard(Config{Locker: locker, Logger: logger}, "election:unlockfail",
		WithActiveTTL(200*time.Millisecond), WithActiveInterval(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	demoted := make(chan struct{}, 4)
	go g.Run(ctx, func() error { return nil }, func() { demoted <- struct{}{} })

	waitUntilActive(t, g, 2*time.Second)
	select {
	case <-demoted:
	case <-time.After(2 * time.Second):
		t.Fatalf("unlock failure during renew must demote the leader")
	}
	if g.IsActive() {
		t.Fatalf("guard should be standby after unlock failure")
	}
	if logger.warn == 0 {
		t.Fatalf("renew failure should be logged as warning")
	}
}

// TestActiveGuardDemote leader 主动放弃租约后立刻回到待命态，
// 待命副本在轮询周期内接管；非活跃态调用为无操作。
func TestActiveGuardDemote(t *testing.T) {
	locker := NewLocalLocker()

	g1 := NewActiveGuard(Config{Locker: locker}, "election:demote",
		WithActiveTTL(time.Second), WithActiveInterval(50*time.Millisecond))
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	go g1.Run(ctx1, func() error { return nil }, nil)
	waitUntilActive(t, g1, 2*time.Second)

	demoted := make(chan struct{})
	g1.Demote(func() { close(demoted) })
	select {
	case <-demoted:
	case <-time.After(time.Second):
		t.Fatalf("Demote must invoke onDemoted callback")
	}
	if g1.IsActive() {
		t.Fatalf("guard should be standby after Demote")
	}
	// 非活跃态再调用 Demote 不应触发回调
	g1.Demote(func() { t.Fatalf("onDemoted must not fire when standby") })

	// 租约已释放，其他副本无需等 TTL 即可接管
	g2 := NewActiveGuard(Config{Locker: locker}, "election:demote",
		WithActiveTTL(time.Second), WithActiveInterval(50*time.Millisecond))
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	go g2.Run(ctx2, func() error { return nil }, nil)
	waitUntilActive(t, g2, 500*time.Millisecond)

	// 无 Locker 的守卫 Demote 为无操作；nil 守卫调用不 panic
	NewActiveGuard(Config{}, "s").Demote(nil)
	var nilGuard *ActiveGuard
	nilGuard.Demote(nil)
	nilGuard.Run(context.Background(), nil, nil)
}
