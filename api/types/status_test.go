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

package types

import (
	"testing"

	"github.com/rulego/rulego/test/assert"
)

// TestNodeStatus 各状态的字符串表示与 JSON 序列化，未知值回落 none
func TestNodeStatus(t *testing.T) {
	cases := []struct {
		status NodeStatus
		text   string
	}{
		{StatusNone, "none"},
		{StatusConnected, "connected"},
		{StatusReconnecting, "reconnecting"},
		{StatusDisconnected, "disconnected"},
		{NodeStatus(99), "none"},
	}
	for _, c := range cases {
		assert.Equal(t, c.text, c.status.String())
		data, err := c.status.MarshalJSON()
		assert.Nil(t, err)
		assert.Equal(t, `"`+c.text+`"`, string(data))
	}
}
