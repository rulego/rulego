/*
 * Copyright 2023 The RuleGo Authors.
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
	"github.com/rulego/rulego/api/types"
	"sync"
)

var (
	// Compile-time check Validator implements types.OnChainBeforeInitAspect.
	_ types.OnChainBeforeInitAspect = (*Validator)(nil)
)

// Validator 规则链初始化校验切面
type Validator struct {
}

func (aspect *Validator) Order() int {
	return 10
}

func (aspect *Validator) New() types.Aspect {
	return &Validator{}
}

func (aspect *Validator) Type() string {
	return "validator"
}

// OnChainBeforeInit 规则引擎初始化之前的增强点，如果返回错误，则创建失败
func (aspect *Validator) OnChainBeforeInit(config types.Config, def *types.RuleChain) error {
	ruleList := Rules.Rules()
	for _, rule := range ruleList {
		if err := rule(config, def); err != nil {
			return err
		}
	}
	return nil
}

// Rules 规则校验规则
var Rules = NewRules()
var (
	// ErrNotAllowEndpointNode 子规则链不允许创建endpoint组件
	ErrNotAllowEndpointNode = errors.New("the sub rule chain does not allow endpoint nodes")
	// ErrCycleDetected 检测到环引用
	ErrCycleDetected = errors.New("cycle detected in rule chain")
)

type rules struct {
	rules []func(config types.Config, def *types.RuleChain) error
	sync.RWMutex
}

func NewRules() *rules {
	r := &rules{}
	//子规则链不允许创建endpoint组件
	r.AddRule(func(config types.Config, def *types.RuleChain) error {
		if def != nil {
			if !def.RuleChain.Root && len(def.Metadata.Endpoints) > 0 {
				return ErrNotAllowEndpointNode
			}
		}
		return nil
	})
	//建环检测
	r.AddRule(func(config types.Config, def *types.RuleChain) error {
		if def != nil {
			if !config.AllowCycle {
				return CheckCycles(def.Metadata)
			}
		}
		return nil
	})
	return r
}
func (r *rules) AddRule(fn ...func(config types.Config, def *types.RuleChain) error) {
	r.Lock()
	defer r.Unlock()
	r.rules = append(r.rules, fn...)
}

// Rules 返回副本
func (r *rules) Rules() []func(config types.Config, def *types.RuleChain) error {
	r.RLock()
	defer r.RUnlock()
	return append([]func(config types.Config, def *types.RuleChain) error(nil), r.rules...)
}

// CheckCycles 拓扑排序检测环
func CheckCycles(metadata types.RuleMetadata) error {
	// 创建邻接表和入度表
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, node := range metadata.Nodes {
		if node == nil {
			continue
		}
		adj[node.Id] = []string{}
		inDegree[node.Id] = 0
	}

	for _, connection := range metadata.Connections {
		from := connection.FromId
		to := connection.ToId
		if adj[from] != nil { // 确保节点存在
			if adj[to] == nil {
				continue // 如果目标节点不存在，跳过
			}
			adj[from] = append(adj[from], to)
			inDegree[to] += 1
		}
	}

	// 初始化队列，收集入度为0的节点
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	// 记录处理过的节点数量
	processed := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		processed++

		for _, neighbor := range adj[node] {
			inDegree[neighbor] -= 1
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// 如果处理过的节点数少于总节点数，说明存在环
	if processed < len(metadata.Nodes) {
		return ErrCycleDetected
	}

	return nil
}
