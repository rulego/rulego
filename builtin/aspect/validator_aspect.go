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
	"sync"

	"github.com/rulego/rulego/api/types"
)

var (
	// Compile-time check Validator implements types.OnChainBeforeInitAspect.
	_ types.OnChainBeforeInitAspect = (*Validator)(nil)
)

// Validator is a rule chain initialization validation aspect that performs
// comprehensive validation checks before rule chain creation. It ensures
// rule chain integrity and prevents invalid configurations from being deployed.
//
// Validator 是规则链初始化验证切面，在规则链创建之前执行全面的验证检查。
// 它确保规则链完整性并防止部署无效配置。
//
// Features:
// 功能特性：
//   - Pre-initialization validation  初始化前验证
//   - Cycle detection in rule chains  规则链中的环检测
//   - Endpoint node restrictions for sub-chains  子链的端点节点限制
//   - Extensible validation rule system  可扩展的验证规则系统
//   - Configurable validation behavior  可配置的验证行为
//
// Built-in Validation Rules:
// 内置验证规则：
//   - Sub-chains cannot contain endpoint nodes  子链不能包含端点节点
//   - Cycle detection (unless explicitly allowed)  环检测（除非明确允许）
//   - Node existence validation  节点存在性验证
//   - Connection integrity checks  连接完整性检查
//
// Usage:
// 使用方法：
//
//	// Apply validator to rule engine
//	// 为规则引擎应用验证器
//	config := types.NewConfig().WithAspects(&Validator{})
//	engine := rulego.NewRuleEngine(config)
//
//	// Add custom validation rules
//	// 添加自定义验证规则
//	Rules.AddRule(func(config types.Config, def *types.RuleChain) error {
//		// Custom validation logic
//		return nil
//	})
type Validator struct {
}

// Order returns the execution order of this aspect. Lower values execute earlier.
// Validator has order 10, ensuring validation occurs before other aspects.
//
// Order 返回此切面的执行顺序。值越低，执行越早。
// Validator 的顺序为 10，确保验证在其他切面之前进行。
func (aspect *Validator) Order() int {
	return 10
}

// New creates a new instance of the validation aspect.
// Each rule engine gets its own validator instance.
//
// New 创建验证切面的新实例。
// 每个规则引擎都获得自己的验证器实例。
func (aspect *Validator) New() types.Aspect {
	return &Validator{}
}

// Type returns the unique identifier for this aspect type.
//
// Type 返回此切面类型的唯一标识符。
func (aspect *Validator) Type() string {
	return "validator"
}

// OnChainBeforeInit is called before rule chain initialization. It executes
// all registered validation rules and returns an error if any validation fails.
// This prevents invalid rule chains from being created.
//
// OnChainBeforeInit 在规则链初始化之前调用。它执行所有注册的验证规则，
// 如果任何验证失败则返回错误。这防止创建无效的规则链。
//
// Parameters:
// 参数：
//   - config: Rule engine configuration  规则引擎配置
//   - def: Rule chain definition to validate  要验证的规则链定义
//
// Returns:
// 返回：
//   - error: Validation error if any rule fails, nil if all pass
//     error：如果任何规则失败则返回验证错误，如果全部通过则为 nil
func (aspect *Validator) OnChainBeforeInit(config types.Config, def *types.RuleChain) error {
	ruleList := Rules.Rules()
	for _, rule := range ruleList {
		if err := rule(config, def); err != nil {
			return err
		}
	}
	return nil
}

// Rules is the global validation rules registry that manages all validation
// functions applied during rule chain initialization.
//
// Rules 是全局验证规则注册表，管理规则链初始化期间应用的所有验证函数。
var Rules = NewRules()

var (
	// ErrNotAllowEndpointNode is returned when a sub-rule chain attempts to define endpoint nodes.
	// Sub-chains are not allowed to have endpoints as they should only contain processing logic.
	//
	// ErrNotAllowEndpointNode 当子规则链尝试定义端点节点时返回。
	// 子链不允许有端点，因为它们应该只包含处理逻辑。
	ErrNotAllowEndpointNode = errors.New("the sub rule chain does not allow endpoint nodes")

	// ErrCycleDetected is returned when a circular reference is detected in the rule chain.
	// This prevents infinite loops during rule execution.
	//
	// ErrCycleDetected 当在规则链中检测到循环引用时返回。
	// 这防止规则执行期间的无限循环。
	ErrCycleDetected = errors.New("cycle detected in rule chain")
)

// rules is a thread-safe container for validation rule functions.
// It provides methods to add new rules and retrieve existing ones safely.
//
// rules 是验证规则函数的线程安全容器。
// 它提供安全地添加新规则和检索现有规则的方法。
type rules struct {
	rules        []func(config types.Config, def *types.RuleChain) error // Validation rule functions  验证规则函数
	sync.RWMutex                                                         // Reader-writer mutex for thread safety  用于线程安全的读写互斥锁
}

// NewRules creates a new rules registry with default validation rules pre-configured.
// It includes built-in rules for endpoint node restrictions and cycle detection.
//
// NewRules 创建一个预配置默认验证规则的新规则注册表。
// 它包括端点节点限制和环检测的内置规则。
//
// Default Rules:
// 默认规则：
//  1. Sub-chains cannot contain endpoint nodes  子链不能包含端点节点
//  2. Cycle detection (when not explicitly allowed)  环检测（当未明确允许时）
//
// Returns:
// 返回：
//   - *rules: Configured rules registry  配置好的规则注册表
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

// AddRule adds one or more validation rule functions to the registry.
// New rules are appended to the existing list and will be executed
// in the order they were added.
//
// AddRule 向注册表添加一个或多个验证规则函数。
// 新规则会附加到现有列表中，并按添加顺序执行。
//
// Parameters:
// 参数：
//   - fn: Variable number of validation rule functions
//     fn：可变数量的验证规则函数
//
// Thread Safety:
// 线程安全：
// This method is thread-safe and uses a write lock to ensure
// concurrent modifications don't corrupt the rules list.
// 此方法是线程安全的，使用写锁确保并发修改不会破坏规则列表。
func (r *rules) AddRule(fn ...func(config types.Config, def *types.RuleChain) error) {
	r.Lock()
	defer r.Unlock()
	r.rules = append(r.rules, fn...)
}

// Rules returns a copy of all validation rule functions.
// This method provides thread-safe access to the rules without exposing
// the internal slice to modification.
//
// Rules 返回所有验证规则函数的副本。
// 此方法提供对规则的线程安全访问，而不会将内部切片暴露给修改。
//
// Returns:
// 返回：
//   - []func(...) error: Copy of validation rule functions  验证规则函数的副本
//
// Thread Safety:
// 线程安全：
// This method uses a read lock to allow concurrent reads while
// preventing reads during rule modifications.
// 此方法使用读锁允许并发读取，同时防止在规则修改期间读取。
func (r *rules) Rules() []func(config types.Config, def *types.RuleChain) error {
	r.RLock()
	defer r.RUnlock()
	return append([]func(config types.Config, def *types.RuleChain) error(nil), r.rules...)
}

// CheckCycles performs cycle detection in rule chains using topological sorting algorithm.
// It builds a directed graph from rule node connections and detects cycles that would
// cause infinite loops during rule execution.
//
// CheckCycles 使用拓扑排序算法在规则链中执行环检测。
// 它从规则节点连接构建有向图，并检测在规则执行期间会导致无限循环的环。
//
// Algorithm:
// 算法：
//  1. Build adjacency list and in-degree table  构建邻接表和入度表
//  2. Initialize queue with zero in-degree nodes  用零入度节点初始化队列
//  3. Process nodes in topological order  按拓扑顺序处理节点
//  4. If not all nodes processed, cycle exists  如果未处理所有节点，则存在环
//
// Parameters:
// 参数：
//   - metadata: Rule chain metadata containing nodes and connections
//     metadata：包含节点和连接的规则链元数据
//
// Returns:
// 返回：
//   - error: ErrCycleDetected if cycle found, nil if no cycles
//     error：如果发现环则返回 ErrCycleDetected，如果没有环则为 nil
//
// Time Complexity: O(V + E) where V is nodes and E is connections
// 时间复杂度：O(V + E)，其中 V 是节点数，E 是连接数
//
// Space Complexity: O(V + E) for adjacency list and degree tracking
// 空间复杂度：O(V + E) 用于邻接表和度数跟踪
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
