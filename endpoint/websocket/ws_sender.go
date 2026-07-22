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

package websocket

import (
	"errors"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types/endpoint"
)

// wsSender 封装 *websocket.Conn，实现 endpoint.Sender。互斥锁保护 WriteMessage。
type wsSender struct {
	conn        *websocket.Conn
	messageType int // 0 时默认 TextMessage
	mu          sync.Mutex
}

func (s *wsSender) Send(data []byte) error {
	return s.SendWithType(data, s.messageType)
}

// SendWithType 加锁写一帧，指定消息类型。
func (s *wsSender) SendWithType(data []byte, messageType int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return errors.New("wsSender: conn is nil")
	}
	mt := messageType
	if mt == 0 {
		mt = websocket.TextMessage
	}
	return s.conn.WriteMessage(mt, data)
}

// Close 关闭连接，满足 io.Closer（TTL 扫描用）。
func (s *wsSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

var _ endpoint.Sender = (*wsSender)(nil)
var _ io.Closer = (*wsSender)(nil)
