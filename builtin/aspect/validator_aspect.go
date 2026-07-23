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
// Validator is the validation face of the rule chain initialization, performing comprehensive validation checks before the rule chain is created.
// It ensures the integrity of the rule chain and prevents the deployment of invalid configurations.
//
// Features:
// Features:
//   - Pre-initialization validation
//   - Cycle detection in rule chains
//   - Endpoint node restrictions for sub-chains
//   - Extensible validation rule system
//   - Configurable validation behavior
//
// Built-in Validation Rules:
// Built-in Verification Rules:
//   - Sub-chains cannot contain endpoint nodes
//   - Cycle detection (unless explicitly allowed)
//   - Node existence validation
//   - Connection integrity checks
//
// Usage:
// How to use:
//
//	// Apply validator to rule engine
//	Apply validators to rule engines
//	config := types.NewConfig().WithAspects(&Validator{})
//	engine := rulego.NewRuleEngine(config)
//
//	// Add custom validation rules
//	Add custom authentication rules
//	Rules.AddRule(func(config types.Config, def *types.RuleChain) error {
//		// Custom validation logic
//		return nil
//	})
type Validator struct {
}

// Order returns the execution order of this aspect. Lower values execute earlier.
// Validator has order 10, ensuring validation occurs before other aspects.
//
// Order returns the execution order of this aspect. The lower the value, the earlier it is executed.
// ValidatorAspect has order 10, ensuring validation occurs before other aspects.
func (aspect *Validator) Order() int {
	return 10
}

// New creates a new instance of the validation aspect.
// Each rule engine gets its own validator instance.
//
// New: Create a new instance of the verification facet.
// Each rule engine receives its own validator instance.
func (aspect *Validator) New() types.Aspect {
	return &Validator{}
}

// Type returns the unique identifier for this aspect type.
//
// Type returns a unique identifier for this facet type.
func (aspect *Validator) Type() string {
	return "validator"
}

// OnChainBeforeInit is called before rule chain initialization. It executes
// all registered validation rules and returns an error if any validation fails.
// This prevents invalid rule chains from being created.
//
// OnChainBeforeInit is called before the rule chain is initialized. It enforces all registered verification rules,
// If any authentication fails, an error is returned. This prevents the creation of invalid chain of rules.
//
// Parameters:
// Parameters:
//   - config: Rule engine configuration
//   - def: Rule chain definition to validate
//
// Returns:
// Returns:
//   - error: Validation error if any rule fails, nil if all pass
//     error: If any rule fails, it returns a validation error; if all rules pass, it is nil
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
// Rules is a global validation rule registry that manages all validation functions applied during the rule chain initialization.
var Rules = NewRules()

var (
	// ErrNotAllowEndpointNode is returned when a sub-rule chain attempts to define endpoint nodes.
	// Sub-chains are not allowed to have endpoints as they should only contain processing logic.
	//
	// ErrNotAllowEndpointNode Returned when the sub-rule chain attempts to define an endpoint node.
	// Subchains are not allowed to have endpoints, as they should only contain processing logic.
	ErrNotAllowEndpointNode = errors.New("the sub rule chain does not allow endpoint nodes")

	// ErrCycleDetected is returned when a circular reference is detected in the rule chain.
	// This prevents infinite loops during rule execution.
	//
	// ErrCycleDetected Returns when a loop reference is detected in the rule chain.
	// This prevents an endless loop during rule enforcement.
	ErrCycleDetected = errors.New("cycle detected in rule chain")
)

// rules is a thread-safe container for validation rule functions.
// It provides methods to add new rules and retrieve existing ones safely.
//
// Rules are thread-safe containers that validate rule functions.
// It provides a way to securely add new rules and retrieve existing ones.
type rules struct {
	rules        []func(config types.Config, def *types.RuleChain) error // Validation rule functions
	sync.RWMutex                                                         // Reader-writer mutex for thread safety
}

// NewRules creates a new rules registry with default validation rules pre-configured.
// It includes built-in rules for endpoint node restrictions and cycle detection.
//
// NewRules creates a new rule registry with pre-configured default validation rules.
// It includes built-in rules for endpoint node limiting and ring detection.
//
// Default Rules:
// Default rules:
//  1. Sub-chains cannot contain endpoint nodes
//  2. Cycle detection (when not explicitly allowed)
//
// Returns:
// Returns:
//   - *rules: Configured rules registry
func NewRules() *rules {
	r := &rules{}
	//Sub-rule chains do not allow the creation of endpoint components
	r.AddRule(func(config types.Config, def *types.RuleChain) error {
		if def != nil {
			if !def.RuleChain.Root && len(def.Metadata.Endpoints) > 0 {
				return ErrNotAllowEndpointNode
			}
		}
		return nil
	})
	//Construction Environment Testing
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
// AddRule adds one or more validation rule functions to the registry.
// New rules are appended to existing lists and executed in the order they are added.
//
// Parameters:
// Parameters:
//   - fn: Variable number of validation rule functions
//     fn: Variable quantity validation rule function
//
// Thread Safety:
// Thread safety:
// This method is thread-safe and uses a write lock to ensure
// concurrent modifications don't corrupt the rules list.
// This method is thread-safe and uses write locks to ensure concurrent modifications do not break the rule list.
func (r *rules) AddRule(fn ...func(config types.Config, def *types.RuleChain) error) {
	r.Lock()
	defer r.Unlock()
	r.rules = append(r.rules, fn...)
}

// Rules returns a copy of all validation rule functions.
// This method provides thread-safe access to the rules without exposing
// the internal slice to modification.
//
// Rules returns copies of all validation rule functions.
// This method provides thread-safe access to the rules without exposing the internal slices to modifications.
//
// Returns:
// Returns:
//   - []func(...) error: Copy of validation rule functions
//
// Thread Safety:
// Thread safety:
// This method uses a read lock to allow concurrent reads while
// preventing reads during rule modifications.
// This method uses a read lock to allow concurrent reads while preventing readings during rule modifications.
func (r *rules) Rules() []func(config types.Config, def *types.RuleChain) error {
	r.RLock()
	defer r.RUnlock()
	return append([]func(config types.Config, def *types.RuleChain) error(nil), r.rules...)
}

// CheckCycles performs cycle detection in rule chains using topological sorting algorithm.
// It builds a directed graph from rule node connections and detects cycles that would
// cause infinite loops during rule execution.
//
// CheckCycles uses topological sorting algorithms to perform loop detection in the rule chain.
// It connects from rule nodes to construct directed graphs and detects loops that cause infinite loops during rule execution.
//
// Algorithm:
// Algorithm:
//  1. Build adjacency list and in-degree table
//  2. Initialize queue with zero in-degree nodes
//  3. Process nodes in topological order
//  4. If not all nodes processed, cycle exists
//
// Parameters:
// Parameters:
//   - metadata: Rule chain metadata containing nodes and connections
//     metadata: Contains the rule chain metadata of nodes and connections
//
// Returns:
// Returns:
//   - error: ErrCycleDetected if cycle found, nil if no cycles
//     error: If a loop is found, returns ErrCycleDetected; if there is no loop, it returns nil
//
// Time Complexity: O(V + E) where V is nodes and E is connections
// Time complexity: O(V + E), where V is the number of nodes and E is the number of connections
//
// Space Complexity: O(V + E) for adjacency list and degree tracking
// Spatial complexity: O(V + E) is used for adjacency lists and degree tracking
func CheckCycles(metadata types.RuleMetadata) error {
	// Create adjacency lists and incoming lists
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
		if adj[from] != nil { // Ensure nodes exist
			if adj[to] == nil {
				continue // If the target node does not exist, skip it
			}
			adj[from] = append(adj[from], to)
			inDegree[to] += 1
		}
	}

	// Initialize the queue and collect nodes with entry degree 0
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	// Records the number of nodes processed
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

	// If the number of nodes processed is less than the total number of nodes, it indicates the presence of a loop
	if processed < len(metadata.Nodes) {
		return ErrCycleDetected
	}

	return nil
}
