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

// 服务端型 endpoint 的会话寻址抽象。接入的连接原本只在 handler 局部作用域，无法跨请求寻址；
// 这里提供 Session 注册表，让业务侧能按 Key 向指定客户端主动推送。
//
// 并发模型：Key 经 Key()/SetKey() 内部 RWMutex 保护；Sender 连接生命周期内不变，并发安全由实现方保证；
// 改写已注册 session 的 Key 应通过 SessionRegistry.Rekey（同时更新索引）。
package endpoint

import (
	"sync"
	"sync/atomic"
	"time"
)

// Sender 协议无关的发送通道，由各 endpoint 实现。
type Sender interface {
	// Send 发送一帧数据，实现方需保证并发安全。
	Send(data []byte) error
}

// Session 表示一个客户端会话，并发安全。
type Session struct {
	// Sender 发送通道，连接生命周期内不变。
	Sender Sender

	mu          sync.RWMutex
	key         string
	keyResolved bool // sessionKey 是否已确定（首帧提取后不再变更）

	lastSeen int64 // 最近活跃时间（UnixNano），atomic 保护，每帧 Touch 刷新，TTL 扫描据此淘汰
}

// NewSession 创建一个 session，初始 Key 通常为 RemoteAddr。
func NewSession(key string, sender Sender) *Session {
	s := &Session{Sender: sender, key: key}
	s.Touch() // 构造即置 lastSeen
	return s
}

// Touch 刷新 lastSeen 为当前时间。每收到一帧调用（含心跳帧），用于 TTL 保活。
func (s *Session) Touch() { atomic.StoreInt64(&s.lastSeen, time.Now().UnixNano()) }

// TouchAt 用显式时间刷新 lastSeen。供测试注入时钟构造混合年龄场景。
func (s *Session) TouchAt(now time.Time) { atomic.StoreInt64(&s.lastSeen, now.UnixNano()) }

// LastSeen 返回最近活跃时间（UnixNano）。TTL 扫描用。
func (s *Session) LastSeen() int64 { return atomic.LoadInt64(&s.lastSeen) }

// Key 返回当前寻址键。
func (s *Session) Key() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.key
}

// IsResolved 返回 sessionKey 是否已确定。
func (s *Session) IsResolved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keyResolved
}

// SetKey 更新 Key 并标记为已确定，空 key 被忽略。
// 只改字段，不更新 registry 索引；改已注册的 session 应用 Rekey。
func (s *Session) SetKey(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	s.keyResolved = true
}

// SessionRegistry 会话注册表，由服务端型 endpoint 实现。
//
// Lookup 按 target 寻址：
//   - "*" 或空：广播，返回全部 session
//   - 其他：精确匹配 Key（target 应为 sessionKey 提取出的值，如 deviceId）
type SessionRegistry interface {
	Add(*Session)
	Remove(key string)
	// Rekey 改写 session 的 Key：注销旧 Key、SetKey、注册新 Key。
	Rekey(s *Session, newKey string)
	Lookup(target string) []*Session
}
