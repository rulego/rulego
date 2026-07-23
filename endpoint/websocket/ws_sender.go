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

// wsSender wrapper *websocket.Conn, implementing the endpoint.Sender. Mutex locks protect WriteMessage.
type wsSender struct {
	conn        *websocket.Conn
	messageType int // 0, the default TextMessage is used
	mu          sync.Mutex
}

func (s *wsSender) Send(data []byte) error {
	return s.SendWithType(data, s.messageType)
}

// SendWithType writes a lock to one frame and specifies the message type.
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

// close to close the connection and satisfy io.Closer (for TTL scanning).
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
