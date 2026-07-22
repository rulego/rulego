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

// DefaultSessionRegistry 基于 sync.Map 的通用 SessionRegistry 实现，供服务端型 endpoint 嵌入复用。
type DefaultSessionRegistry struct {
	sessions sync.Map // key: Session.Key() -> *endpoint.Session

	sweepMu   sync.Mutex
	sweepStop chan struct{} // nil = 未扫描
	sweepDone chan struct{} // goroutine 退出信号
}

// Add 注册一个 session，nil 入参忽略。
func (r *DefaultSessionRegistry) Add(s *endpoint.Session) {
	if s == nil {
		return
	}
	r.sessions.Store(s.Key(), s)
}

// Remove 按 Key 注销 session。
func (r *DefaultSessionRegistry) Remove(key string) {
	r.sessions.Delete(key)
}

// Clear 清空所有 session（endpoint 销毁时兜底，此时不应有并发写入）。
func (r *DefaultSessionRegistry) Clear() {
	r.sessions.Range(func(k, _ any) bool {
		r.sessions.Delete(k)
		return true
	})
}

// Rekey 改写 session 的 Key：注销旧 Key、SetKey、注册新 Key。封装三步避免调用方遗漏 Remove。
// 并发约束：Delete(oldKey) 与 Store(newKey) 之间存在短暂窗口，期间并发 Lookup 可能找不到该 session。
// 调用方应保证同一 session 的 Rekey 串行调用（当前各 endpoint handler 每连接单 goroutine，满足此约束）。
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

// Lookup 按 target 寻址：空或 "*" 广播；其他精确匹配 Key（target 应为 sessionKey 值，如 deviceId）。
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

// StartSweeping 启动后台扫描，按 interval 淘汰 idle 超过 ttl 的 session（关连接促其重连）。
// ttl/interval<=0 或重复调用为 no-op。
func (r *DefaultSessionRegistry) StartSweeping(ttl, interval time.Duration) {
	if ttl <= 0 || interval <= 0 {
		return
	}
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	if r.sweepStop != nil {
		return // 已在扫描
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

// StopSweeping 停止扫描并等待退出，幂等。
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
	<-done // 等 goroutine 退出，防泄漏
}

// sweep 扫描淘汰 idle 超时的 session，返回淘汰数。now 供测试注入。
func (r *DefaultSessionRegistry) sweep(now time.Time, ttl time.Duration) int {
	deadline := now.UnixNano() - int64(ttl)
	var evicted int
	r.sessions.Range(func(k, v any) bool {
		s := v.(*endpoint.Session)
		if s.LastSeen() > deadline {
			return true
		}
		// 原子认领，避免与 disconnect defer 双删
		actual, loaded := r.sessions.LoadAndDelete(k)
		if !loaded {
			return true
		}
		ss := actual.(*endpoint.Session)
		// 认领窗口内若刚收到帧，回填
		if ss.LastSeen() > deadline {
			r.sessions.Store(k, ss)
			return true
		}
		// 已 Rekey 到新键，不回填旧键
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
