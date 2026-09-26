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

// TestRuleChainGetNode 按节点 ID 查找
func TestRuleChainGetNode(t *testing.T) {
	chain := RuleChain{
		Metadata: RuleMetadata{
			Nodes: []*RuleNode{
				{Id: "n1", Type: "jsFilter"},
				{Id: "n2", Type: "log"},
			},
		},
	}
	node, ok := chain.GetNode("n2")
	assert.True(t, ok)
	assert.Equal(t, "log", node.Type)

	_, ok = chain.GetNode("missing")
	assert.False(t, ok)
}

// TestRuleChainBaseInfoAdditionalInfo 附加信息的读取与写入；
// PutAdditionalInfo 为值接收者，只在 map 已初始化时可观察生效
func TestRuleChainBaseInfoAdditionalInfo(t *testing.T) {
	base := RuleChainBaseInfo{AdditionalInfo: map[string]interface{}{"author": "admin"}}
	v, ok := base.GetAdditionalInfo("author")
	assert.True(t, ok)
	assert.Equal(t, "admin", v)

	base.PutAdditionalInfo("version", "1.0.0")
	v, ok = base.GetAdditionalInfo("version")
	assert.True(t, ok)
	assert.Equal(t, "1.0.0", v)

	_, ok = base.GetAdditionalInfo("missing")
	assert.False(t, ok)

	empty := RuleChainBaseInfo{}
	v, ok = empty.GetAdditionalInfo("any")
	assert.False(t, ok)
	assert.Equal(t, "", v)
	// 值接收者在 nil map 时只初始化副本字段，调用方不 panic 但看不到写入
	empty.PutAdditionalInfo("k", "v")
	_, ok = empty.GetAdditionalInfo("k")
	assert.False(t, ok)
}

// TestRuleNodeGetAdditionalInfo 节点附加信息读取
func TestRuleNodeGetAdditionalInfo(t *testing.T) {
	node := RuleNode{AdditionalInfo: map[string]interface{}{"layoutX": 100}}
	v, ok := node.GetAdditionalInfo("layoutX")
	assert.True(t, ok)
	assert.Equal(t, 100, v)

	_, ok = node.GetAdditionalInfo("missing")
	assert.False(t, ok)

	_, ok = RuleNode{}.GetAdditionalInfo("any")
	assert.False(t, ok)
}
