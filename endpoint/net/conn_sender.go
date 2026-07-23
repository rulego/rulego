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

// connSender encapsulation net.Conn, implementing the endpoint.Sender. Mutex locks protect write and prevent concurrent write interlacing.
type connSender struct {
	conn net.Conn
	mu   sync.Mutex
}

// Send: Lock and write one frame.
func (s *connSender) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return errors.New("connSender: conn is nil")
	}
	_, err := s.conn.Write(data)
	return err
}

// close to close the connection and satisfy io.Closer (for TTL scanning).
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
