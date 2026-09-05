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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Locker is a distributed key lock abstraction shared by endpoints, nodes and hosts.
// Implementations must be safe for concurrent use by multiple goroutines.
//
// Locker 是端点、组件与宿主共用的分布式键锁抽象。实现必须支持多协程并发调用。
type Locker interface {
	// Lock acquires the lock, blocking until it is obtained or ctx is done,
	// and returns the token used to release it.
	// Lock 阻塞获取锁，返回持有凭证，用于释放时校验。
	Lock(ctx context.Context, key string, expiration time.Duration) (string, error)
	// Unlock releases the lock. It returns an error when the token does not
	// match the current holder, so a stale holder never releases someone
	// else's lock.
	// Unlock 释放锁。token 与当前持有凭证不匹配时返回错误，防止误释放他人的锁。
	Unlock(ctx context.Context, key, token string) error
	// TryLock acquires the lock without blocking. acquired=false means the
	// lock is held elsewhere and is not an error.
	// TryLock 非阻塞获取锁。acquired=false 表示锁被其他持有者占用，不视为错误。
	TryLock(ctx context.Context, key string, expiration time.Duration) (string, bool, error)
	// LockWithRetry acquires the lock, retrying every retryInterval up to
	// maxRetries times before returning an error.
	// LockWithRetry 按固定间隔重试获取锁，超过 maxRetries 次仍未获得则返回错误。
	LockWithRetry(ctx context.Context, key string, expiration time.Duration, retryInterval time.Duration, maxRetries int) (string, error)
}

// ErrLockNotHeld is returned by LocalLocker.Unlock when the token does not
// match the current holder.
// ErrLockNotHeld 在 token 与当前持有凭证不匹配时由 LocalLocker.Unlock 返回。
var ErrLockNotHeld = errors.New("rulego: lock token mismatch")

// localLockRetryInterval is the polling interval used by LocalLocker.Lock.
// localLockRetryInterval 是 LocalLocker.Lock 的轮询间隔。
const localLockRetryInterval = 50 * time.Millisecond

// LocalLocker is an in-process Locker implementation with expiration
// bookkeeping. It coordinates goroutines within one process only; cross
// replica mutual exclusion requires an external implementation such as Redis.
//
// LocalLocker 是带过期记账的进程内锁实现，仅协调同进程内的协程；
// 跨副本互斥需注入外部实现（如 Redis）。
type LocalLocker struct {
	mu        sync.Mutex
	locks     map[string]localLockEntry
	lastSweep time.Time
}

type localLockEntry struct {
	token    string
	expireAt time.Time
}

// localLockSweepInterval 过期键清理周期。到期的键在释放或覆盖前会留在 map 中，
// 周期性清理避免唯一键持续增长（如定时槽位键）造成内存泄漏。
// var 而非 const：测试需要缩短周期。
var localLockSweepInterval = time.Minute

// NewLocalLocker creates a LocalLocker.
// NewLocalLocker 创建进程内键锁。
func NewLocalLocker() *LocalLocker {
	return &LocalLocker{locks: make(map[string]localLockEntry)}
}

func (l *LocalLocker) Lock(ctx context.Context, key string, expiration time.Duration) (string, error) {
	for {
		if token, ok := l.tryLock(key, expiration); ok {
			return token, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(localLockRetryInterval):
		}
	}
}

func (l *LocalLocker) Unlock(_ context.Context, key, token string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.locks[key]
	if !ok || entry.token != token {
		return ErrLockNotHeld
	}
	delete(l.locks, key)
	return nil
}

func (l *LocalLocker) TryLock(_ context.Context, key string, expiration time.Duration) (string, bool, error) {
	token, ok := l.tryLock(key, expiration)
	return token, ok, nil
}

// LockWithRetry 先立即尝试一次，之后每隔 retryInterval 重试，最多重试 maxRetries 次。
func (l *LocalLocker) LockWithRetry(ctx context.Context, key string, expiration time.Duration, retryInterval time.Duration, maxRetries int) (string, error) {
	if retryInterval <= 0 {
		retryInterval = localLockRetryInterval
	}
	for i := 0; i <= maxRetries; i++ {
		if token, ok := l.tryLock(key, expiration); ok {
			return token, nil
		}
		if i == maxRetries {
			break
		}
		if err := sleepWithContext(ctx, retryInterval); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("rulego: lock %s not acquired after %d retries", key, maxRetries)
}

func (l *LocalLocker) tryLock(key string, expiration time.Duration) (string, bool) {
	token := newLockToken()
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) >= localLockSweepInterval {
		for k, entry := range l.locks {
			if now.After(entry.expireAt) {
				delete(l.locks, k)
			}
		}
		l.lastSweep = now
	}
	if entry, ok := l.locks[key]; ok && now.Before(entry.expireAt) {
		return "", false
	}
	l.locks[key] = localLockEntry{token: token, expireAt: now.Add(expiration)}
	return token, true
}

func newLockToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// onceGuardDefaultTTL is the default lock retention for OnceGuard. It must
// cover the longest expected action so a slow execution never lets another
// replica re-run the same key.
// onceGuardDefaultTTL 是 OnceGuard 的默认锁保留时长，须覆盖最长的单次动作执行时间，
// 避免慢执行期间锁过期导致其他副本重复执行同一 key。
const onceGuardDefaultTTL = time.Hour

// onceLockKeyPrefix is the lock key namespace of OnceGuard.
// onceLockKeyPrefix 是 OnceGuard 锁键的统一命名空间前缀。
const onceLockKeyPrefix = "rulego:once:"

// OnceGuard makes an action run at most once per key across all replicas
// sharing the same Locker.
//
// When Config.Locker is nil, Allow always returns true, so callers can keep
// the guard in place unconditionally: single process deployments are
// unaffected, replicated deployments deduplicate as soon as a Locker is
// injected.
//
// The key must be derived deterministically from the action itself (for
// example a cron schedule slot, a message id or a business primary key). It
// must not include values that differ between replicas, such as a local
// clock reading taken at execution time, otherwise each replica produces a
// different lock key and the deduplication silently stops working.
//
// OnceGuard 让同一动作在共享同一 Locker 的全部副本间至多执行一次。
//
// Config.Locker 为 nil 时 Allow 恒返回 true，调用方可无条件常驻守卫：
// 单机部署不受影响，多副本部署注入 Locker 后自动去重。
//
// key 必须由动作语义确定性推导（如 cron 计划槽位、消息 ID、业务主键），
// 不能包含副本间存在差异的值（如执行时刻的本地时钟读数），
// 否则各副本生成不同锁键，去重将静默失效。
type OnceGuard struct {
	locker   Locker
	logger   Logger
	scope    string
	ttl      time.Duration
	failOpen bool
}

// OnceGuardOption customizes a OnceGuard.
// OnceGuardOption 自定义守卫行为。
type OnceGuardOption func(*OnceGuard)

// WithGuardTTL sets how long the lock key is retained. It must exceed the
// longest expected execution of the guarded action.
// WithGuardTTL 设置锁键保留时长，须大于被守护动作的最长执行时间。
func WithGuardTTL(ttl time.Duration) OnceGuardOption {
	return func(g *OnceGuard) {
		if ttl > 0 {
			g.ttl = ttl
		}
	}
}

// WithGuardFailOpen lets the action run when the Locker backend fails. The
// default policy is fail-closed: skip this action and log a warning, which
// suits periodic actions because a missed run self heals on the next period
// while a duplicated one does not.
// WithGuardFailOpen 在锁服务故障时放行执行。默认策略为 fail-closed：
// 跳过本次并告警，适合周期性动作——漏一拍下个周期自愈，重复执行不可自愈。
func WithGuardFailOpen() OnceGuardOption {
	return func(g *OnceGuard) {
		g.failOpen = true
	}
}

// NewOnceGuard creates a guard bound to ruleConfig.Locker. scope isolates
// unrelated actions from each other and typically names the component and its
// owner, for example "schedule:{owner}:{endpointId}:{routerId}".
// NewOnceGuard 创建绑定 ruleConfig.Locker 的守卫。scope 用于隔离互不相关的动作，
// 通常包含组件与所属者标识，如 "schedule:{owner}:{endpointId}:{routerId}"。
func NewOnceGuard(ruleConfig Config, scope string, opts ...OnceGuardOption) *OnceGuard {
	g := &OnceGuard{
		scope: scope,
		ttl:   onceGuardDefaultTTL,
	}
	if ruleConfig.Locker != nil {
		g.locker = ruleConfig.Locker
	}
	if ruleConfig.Logger != nil {
		g.logger = ruleConfig.Logger
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

// Allow reports whether this process should run the action identified by key.
// The final lock key is "rulego:once:{scope}:{key}".
//
// Allow 判定是否由本进程执行 key 标识的动作。最终锁键为
// "rulego:once:{scope}:{key}"。
func (g *OnceGuard) Allow(ctx context.Context, key string) bool {
	if g == nil || g.locker == nil {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, ok, err := g.locker.TryLock(ctx, onceLockKeyPrefix+g.scope+":"+key, g.ttl)
	if err != nil {
		if g.failOpen {
			g.logf("warn", "once guard %s key=%s backend error, fail-open proceed: %v", g.scope, key, err)
			return true
		}
		g.logf("warn", "once guard %s key=%s backend error, skip action: %v", g.scope, key, err)
		return false
	}
	if !ok {
		g.logf("info", "once guard %s key=%s already executed by another holder, skip", g.scope, key)
		return false
	}
	g.logf("debug", "once guard %s key=%s acquired", g.scope, key)
	return true
}

func (g *OnceGuard) logf(level string, format string, v ...interface{}) {
	if g.logger == nil {
		return
	}
	switch level {
	case "debug":
		g.logger.Debugf(format, v...)
	case "info":
		g.logger.Infof(format, v...)
	default:
		g.logger.Warnf(format, v...)
	}
}
