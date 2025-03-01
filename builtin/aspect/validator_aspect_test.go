/*
 * Copyright 2025 The RuleGo Authors.
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

package aspect

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestCheckCycles(t *testing.T) {
	// 创建一个不存在环的规则链
	metadataWithoutCycle := types.RuleMetadata{
		Nodes: []*types.RuleNode{
			{Id: "s1_1"},
			{Id: "s1_2"},
			{Id: "s2"},
			{Id: "s3"},
		},
		Connections: []types.NodeConnection{
			{FromId: "s1_1", ToId: "s2"},
			{FromId: "s1_2", ToId: "s2"},
			{FromId: "s2", ToId: "s3"},
		},
	}

	// 测试无环情况
	err := CheckCycles(metadataWithoutCycle)
	assert.Nil(t, err)
	//assert.NoError(t, err, "Cycle detection failed for a valid rule chain")

	// 创建一个存在环的规则链
	metadataWithCycle := types.RuleMetadata{
		Nodes: []*types.RuleNode{
			{Id: "s1"},
			{Id: "s2"},
			{Id: "s3"},
		},
		Connections: []types.NodeConnection{
			{FromId: "s1", ToId: "s2"},
			{FromId: "s2", ToId: "s3"},
			{FromId: "s3", ToId: "s1"}, // 形成环
		},
	}

	// 测试有环情况
	err = CheckCycles(metadataWithCycle)
	assert.NotNil(t, err)
	//assert.EqualError(t, err, ErrCycleDetected.Error(), "Cycle detection failed for a rule chain with cycles")
}
