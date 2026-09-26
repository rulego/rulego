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
	"errors"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
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

func TestCheckCyclesEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		metadata types.RuleMetadata
		wantErr  error
	}{
		{"empty metadata", types.RuleMetadata{}, nil},
		{
			// nil nodes are skipped but still counted in len(Nodes), so a nil
			// entry always yields processed < len(Nodes) and reads as a cycle
			"nil node entry reads as a cycle",
			types.RuleMetadata{
				Nodes:       []*types.RuleNode{nil, {Id: "s1"}},
				Connections: []types.NodeConnection{{FromId: "missing", ToId: "s1"}},
			},
			ErrCycleDetected,
		},
		{
			"connection to a missing target is ignored",
			types.RuleMetadata{
				Nodes:       []*types.RuleNode{{Id: "s1"}},
				Connections: []types.NodeConnection{{FromId: "s1", ToId: "missing"}},
			},
			nil,
		},
		{
			"self loop is a cycle",
			types.RuleMetadata{
				Nodes:       []*types.RuleNode{{Id: "s1"}},
				Connections: []types.NodeConnection{{FromId: "s1", ToId: "s1"}},
			},
			ErrCycleDetected,
		},
		{
			"cycle behind a valid branch",
			types.RuleMetadata{
				Nodes: []*types.RuleNode{{Id: "s1"}, {Id: "s2"}, {Id: "s3"}},
				Connections: []types.NodeConnection{
					{FromId: "s1", ToId: "s2"},
					{FromId: "s2", ToId: "s1"},
					{FromId: "s2", ToId: "s3"},
				},
			},
			ErrCycleDetected,
		},
	}
	for _, c := range cases {
		err := CheckCycles(c.metadata)
		assert.Equal(t, c.wantErr, err, c.name)
	}
}

func TestValidatorBasics(t *testing.T) {
	v := &Validator{}
	assert.Equal(t, 10, v.Order())
	assert.Equal(t, "validator", v.Type())
	assert.NotNil(t, v.New())
}

func TestValidatorOnChainBeforeInit(t *testing.T) {
	rootChain := &types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{ID: "root", Root: true},
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{
				{Id: "s1", Type: "jsFilter"},
				{Id: "s2", Type: "jsFilter"},
			},
			Connections: []types.NodeConnection{{FromId: "s1", ToId: "s2"}},
		},
	}
	cyclicChain := &types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{ID: "cycle", Root: true},
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{{Id: "s1"}, {Id: "s2"}},
			Connections: []types.NodeConnection{
				{FromId: "s1", ToId: "s2"},
				{FromId: "s2", ToId: "s1"},
			},
		},
	}
	subChainWithEndpoint := &types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{ID: "sub"},
		Metadata: types.RuleMetadata{
			Nodes:     []*types.RuleNode{{Id: "s1", Type: "mqtt"}},
			Endpoints: []*types.EndpointDsl{{RuleNode: types.RuleNode{Id: "ep1"}}},
		},
	}

	v := &Validator{}
	assert.Nil(t, v.OnChainBeforeInit(types.NewConfig(), nil))
	assert.Nil(t, v.OnChainBeforeInit(types.NewConfig(), rootChain))
	assert.Equal(t, ErrNotAllowEndpointNode, v.OnChainBeforeInit(types.NewConfig(), subChainWithEndpoint)) // sub chains cannot own endpoints
	assert.Nil(t, v.OnChainBeforeInit(types.NewConfig(), withRoot(subChainWithEndpoint, true)))
	assert.Equal(t, ErrCycleDetected, v.OnChainBeforeInit(types.NewConfig(), cyclicChain))

	allowCycle := types.NewConfig()
	allowCycle.AllowCycle = true
	assert.Nil(t, v.OnChainBeforeInit(allowCycle, cyclicChain))
}

func withRoot(def *types.RuleChain, root bool) *types.RuleChain {
	clone := *def
	clone.RuleChain.Root = root
	return &clone
}

func TestValidatorRulesRegistry(t *testing.T) {
	r := NewRules()
	builtin := len(r.Rules())
	assert.True(t, builtin >= 2, "built-in rules should be registered")

	r.AddRule(func(config types.Config, def *types.RuleChain) error {
		return errors.New("custom rule failed")
	})
	rules := r.Rules()
	assert.Equal(t, builtin+1, len(rules))
	assert.NotNil(t, rules[len(rules)-1])

	// the returned slice is a copy: appending does not affect the registry
	rules = append(rules, func(config types.Config, def *types.RuleChain) error { return nil })
	assert.Equal(t, builtin+1, len(r.Rules()))

	// a failing custom rule aborts validation with its error
	sentinel := &types.RuleChain{RuleChain: types.RuleChainBaseInfo{ID: "sentinel"}}
	err := rules[builtin](types.NewConfig(), sentinel)
	assert.Equal(t, "custom rule failed", err.Error())
}
