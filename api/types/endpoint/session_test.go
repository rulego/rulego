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

package endpoint

import (
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

type recordingSender struct {
	frames [][]byte
}

func (s *recordingSender) Send(data []byte) error {
	s.frames = append(s.frames, data)
	return nil
}

// TestSession Key 解析状态与空 key 忽略语义
func TestSession(t *testing.T) {
	sender := &recordingSender{}
	s := NewSession("192.168.1.1:9000", sender)

	assert.Equal(t, "192.168.1.1:9000", s.Key())
	assert.Equal(t, sender, s.Sender)
	assert.False(t, s.IsResolved())
	assert.True(t, s.LastSeen() > 0)

	// 空 key 被忽略，不标记 resolved
	s.SetKey("")
	assert.Equal(t, "192.168.1.1:9000", s.Key())
	assert.False(t, s.IsResolved())

	// 首帧提取出业务 key 后固定
	s.SetKey("device-001")
	assert.Equal(t, "device-001", s.Key())
	assert.True(t, s.IsResolved())
}

// TestSessionLastSeen Touch/TouchAt/LastSeen 的时间记账
func TestSessionLastSeen(t *testing.T) {
	s := NewSession("k", &recordingSender{})

	past := time.Now().Add(-time.Hour)
	s.TouchAt(past)
	assert.Equal(t, past.UnixNano(), s.LastSeen())

	s.Touch()
	assert.True(t, s.LastSeen() > past.UnixNano())
}
