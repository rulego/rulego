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
	"errors"
	"io"
	"net"
	"sync"

	"github.com/rulego/rulego/api/types/endpoint"
)

// connSender 封装 net.Conn，实现 endpoint.Sender。互斥锁保护 Write，避免并发写交错。
type connSender struct {
	conn net.Conn
	mu   sync.Mutex
}

// Send 加锁写入一帧。
func (s *connSender) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return errors.New("connSender: conn is nil")
	}
	_, err := s.conn.Write(data)
	return err
}

// Close 关闭连接，满足 io.Closer（TTL 扫描用）。
func (s *connSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

var _ endpoint.Sender = (*connSender)(nil)
var _ io.Closer = (*connSender)(nil)
