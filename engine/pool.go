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

package engine

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/str"
)

var _ types.RuleEnginePool = (*Pool)(nil)

// DefaultPool is the default global instance of the rule engine pool.
// It provides a singleton pool for managing rule engine instances across the application.
// DefaultPool is the default global instance of the rule engine pool.
// It provides a singleton pool to manage rule engine instances across the entire application.
var DefaultPool = &Pool{}

// Pool is a pool of rule engine instances.
// It provides centralized management of multiple rule engines, enabling efficient
// resource sharing, batch operations, and coordinated lifecycle management.
//
// A pool is the pool of rule engine instances.
// It provides centralized management of multiple rule engines, supporting efficient resource sharing, batch operations, and coordinated lifecycle management.
//
// Key Features:
// Key features:
//   - Concurrent-safe rule engine storage using sync.Map
//   - Automatic rule chain loading from filesystem
//   - Callback-based lifecycle events
//   - Batch operations across multiple engines
//   - Dynamic engine creation and management
//
// Use Cases:
// Usage scenarios:
//   - Multi-tenant rule engine management
//   - Rule chain hot reloading and deployment
//   - Distributed rule processing coordination
//   - Resource sharing between related rule chains
type Pool struct {
	// entries is a concurrent map to store rule engine instances.
	// Uses string keys (rule engine IDs) and *RuleEngine values.
	// entries are concurrent maps that store instances of the rule engine.
	// Use the string key (Rule Engine ID) and the *RuleEngine value.
	entries sync.Map

	// aliases stores the mapping of the alias→ engine, and Pool.Get(alias) can parse the engine.
	// Aliases can be found in RuleEngine.Aliases: When id and def.ruleChain.id are different, the latter is recorded as an alias.
	aliases sync.Map

	// Callbacks provides hooks for rule engine lifecycle events,
	// enabling custom handling of creation, updates, and deletion.
	// Callbacks provide hooks for rule engine lifecycle events,
	// Supports custom processing for creating, updating, and deleting.
	Callbacks types.Callbacks
}

// NewPool creates a new instance of a rule engine pool.
// This function initializes an empty pool ready for use.
//
// NewPool creates a new instance of the rule engine pool.
// This function initializes an empty pool to be used.
//
// Returns:
// Returns:
//   - *Pool: New pool instance
//
// Usage:
// Usage:
//
//	pool := NewPool()
//	engine, err := pool.New("engine1", ruleChainBytes)
func NewPool() *Pool {
	return &Pool{}
}

// Load loads all rule chain configurations from a specified folder and its subfolders into the rule engine instance pool.
// The rule chain ID is taken from the configuration file's ruleChain.id.
//
// Load: Load loads all rule chains from the specified folder and its subfolders into the rule engine instance pool.
// The rule chain ID is taken from the ruleChain.id of the configuration file.
//
// Parameters:
// Parameters:
//   - folderPath: Path to the folder containing rule chain files
//   - opts: Optional configuration functions for the rule engines
//
// Returns:
// Returns:
//   - error: Loading error if any
//
// File Processing:
// File Processing:
//   - Supports JSON files (*.json, *.JSON)
//   - Recursively processes subdirectories
//   - Uses glob patterns for file matching
//   - Automatically extracts rule chain ID from file content
//
// Error Handling:
// Error handling:
//   - Individual file errors are logged but don't stop the overall process
//     Individual document errors are recorded, but the entire process is not stopped
//   - Returns error only for critical failures like invalid folder path
//     Returns errors only for critical faults (such as invalid folder paths).
//
// Callback Integration:
// Callback integration:
//   - Triggers OnNew callback for each successfully loaded rule chain
//     Triggers an OnNew callback for each successfully loaded rule chain
//   - Enables custom processing and validation of loaded chains
//     Supports custom processing and verification of loaded chains
func (g *Pool) Load(folderPath string, opts ...types.RuleEngineOption) error {
	// Ensure the folder path ends with a pattern that matches JSON files.
	if !strings.HasSuffix(folderPath, "*.json") && !strings.HasSuffix(folderPath, "*.JSON") {
		if strings.HasSuffix(folderPath, "/") || strings.HasSuffix(folderPath, "\\") {
			folderPath = folderPath + "*.json"
		} else if folderPath == "" {
			folderPath = "./*.json"
		} else {
			folderPath = folderPath + "/*.json"
		}
	}
	// Get all file paths that match the pattern.
	paths, err := fs.GetFilePaths(folderPath)
	if err != nil {
		return err
	}
	// Load each file and create a new rule engine instance from its contents.
	for _, path := range paths {
		b := fs.LoadFile(path)
		if b != nil {
			if e, err := g.New("", b, opts...); err != nil {
				log.Println("Load rule chain error:", err)
			} else {
				if g.Callbacks.OnNew != nil {
					g.Callbacks.OnNew(e.Id(), b)
				}
			}
		}
	}
	return nil
}

// New creates a new RuleEngine instance and stores it in the rule chain pool.
// If the specified id is empty, the ruleChain.id from the rule chain file is used.
func (g *Pool) New(id string, rootRuleChainSrc []byte, opts ...types.RuleEngineOption) (types.RuleEngine, error) {
	// Check if an instance with the given ID already exists.
	if v, ok := g.entries.Load(id); ok {
		return v.(*RuleEngine), nil
	} else {
		opts = append(opts, types.WithRuleEnginePool(g))
		// Create a new rule engine instance.
		if ruleEngine, err := NewRuleEngine(id, rootRuleChainSrc, opts...); err != nil {
			return nil, err
		} else {
			// Store the new rule engine instance in the pool.
			if ruleEngine.Id() != "" {
				g.entries.Store(ruleEngine.Id(), ruleEngine)
			}
			// Register aliases so that they can also address the engine.
			for _, alias := range ruleEngine.Aliases() {
				if alias != "" && alias != ruleEngine.Id() {
					g.aliases.Store(alias, ruleEngine)
				}
			}
			if g.Callbacks.OnUpdated != nil {
				ruleEngine.OnUpdated = g.Callbacks.OnUpdated
			}
			if g.Callbacks.OnNew != nil {
				g.Callbacks.OnNew(id, rootRuleChainSrc)
			}
			return ruleEngine, err
		}

	}
}

// Get retrieves a rule engine instance by its ID or any registered alias.
// Analysis order: first search by main id (entries), if not accurate, then search by aliases.
func (g *Pool) Get(id string) (types.RuleEngine, bool) {
	if v, ok := g.entries.Load(id); ok {
		return v.(*RuleEngine), ok
	}
	if v, ok := g.aliases.Load(id); ok {
		return v.(*RuleEngine), true
	}
	return nil, false
}

// Del deletes a rule engine instance by its ID (or any of its aliases).
// After parsing the engine, it cleans its primary keys and aliases at the same time.
func (g *Pool) Del(id string) {
	var engine *RuleEngine
	if v, ok := g.entries.Load(id); ok {
		engine = v.(*RuleEngine)
	} else if v, ok := g.aliases.Load(id); ok {
		engine = v.(*RuleEngine)
	}
	if engine == nil {
		return
	}
	engine.Stop(context.Background())
	g.entries.Delete(engine.Id())
	for _, alias := range engine.Aliases() {
		// Only delete aliases that still point to the engine to avoid accidentally deleting names with the same name that have already been overwritten by other engines.
		if v, ok := g.aliases.Load(alias); ok && v.(*RuleEngine) == engine {
			g.aliases.Delete(alias)
		}
	}
	if g.Callbacks.OnDeleted != nil {
		g.Callbacks.OnDeleted(engine.Id())
	}
}

// Stop releases all rule engine instances in the pool.
func (g *Pool) Stop() {
	g.entries.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.Stop(context.Background())
			// Synchronously cleans up the engine's aliases to prevent lingering aliases from pointing to a stopped engine.
			for _, alias := range item.Aliases() {
				g.aliases.Delete(alias)
			}
		}
		g.entries.Delete(key)
		if g.Callbacks.OnDeleted != nil {
			g.Callbacks.OnDeleted(str.ToString(key))
		}
		return true
	})
}

// Range iterates over all rule engine instances in the pool.
func (g *Pool) Range(f func(key, value any) bool) {
	g.entries.Range(f)
}

// Reload reloads all rule engine instances in the pool with the given options.
func (g *Pool) Reload(opts ...types.RuleEngineOption) {
	g.entries.Range(func(key, value any) bool {
		_ = value.(*RuleEngine).Reload(opts...)
		return true
	})
}

// OnMsg invokes all rule engine instances to process a message.
// All rule chains in the rule engine instance pool will attempt to process the message.
func (g *Pool) OnMsg(msg types.RuleMsg) {
	g.entries.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.OnMsg(msg)
		}
		return true
	})
}

func (g *Pool) SetCallbacks(callbacks types.Callbacks) {
	g.Callbacks = callbacks
}

// Load loads all rule chain configurations from the specified folder and its subfolders into the default rule engine instance pool.
// The rule chain ID is taken from the configuration file's ruleChain.id.
//
// Load: Loads all rule chain configurations from the specified folder and its subfolders into the default rule engine instance pool.
// The rule chain ID is taken from the ruleChain.id of the configuration file.
//
// Parameters:
// Parameters:
//   - folderPath: Path to the folder containing rule chain files
//   - opts: Optional configuration functions for the rule engines
//
// Returns:
// Returns:
//   - error: Loading error if any
//
// Usage:
// Usage:
//
//	err := Load("path/to/rulechains", types.WithRuleEnginePool(pool))
func Load(folderPath string, opts ...types.RuleEngineOption) error {
	return DefaultPool.Load(folderPath, opts...)
}

// New creates a new RuleEngine and stores it in the default rule chain pool.
//
// New creates a new rules engine and stores it in the default rules chain pool.
//
// Parameters:
// Parameters:
//   - id: ID of the rule engine
//   - rootRuleChainSrc: Raw bytes of the rule chain
//   - opts: Optional configuration functions for the rule engine
//
// Returns:
// Returns:
//   - types.RuleEngine: New rule engine instance
//   - error: Loading error if any
//
// Usage:
// Usage:
//
//	engine, err := New("engine1", ruleChainBytes)
func New(id string, rootRuleChainSrc []byte, opts ...types.RuleEngineOption) (types.RuleEngine, error) {
	return DefaultPool.New(id, rootRuleChainSrc, opts...)
}

// Get retrieves a specified ID rule engine instance from the default rule chain pool.
//
// Get retrieves the specified ID rule engine instance from the default rule chain pool.
//
// Parameters:
// Parameters:
//   - id: ID of the rule engine
//
// Returns:
// Returns:
//   - types.RuleEngine: Rule engine instance
//   - bool: Existence of the rule engine
//
// Usage:
// Usage:
//
//	engine, exists := Get("engine1")
func Get(id string) (types.RuleEngine, bool) {
	return DefaultPool.Get(id)
}

// Del deletes a specified ID rule engine instance from the default rule chain pool.
//
// Del removes the specified ID rule engine instance from the default rule chain pool.
//
// Parameters:
// Parameters:
//   - id: ID of the rule engine
//
// Usage:
// Usage:
//
//	Del("engine1")
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop releases all rule engine instances in the default rule chain pool.
//
// Stop releases all rule engine instances in the default rule chain pool.
//
// Usage:
// Usage:
//
//	Stop()
func Stop() {
	DefaultPool.Stop()
}

// OnMsg calls all rule engine instances in the default rule chain pool to process a message.
// All rule chains in the rule engine instance pool will attempt to process the message.
//
// OnMsg calls all rule engine instances in the default rule chain pool to process messages.
// All rule chains in the pool of all rule engine instances will attempt to process messages.
//
// Parameters:
// Parameters:
//   - msg: Rule message to be processed
//
// Usage:
// Usage:
//
//	OnMsg(ruleMsg)
func OnMsg(msg types.RuleMsg) {
	DefaultPool.OnMsg(msg)
}

// Reload reloads all rule engine instances in the default rule chain pool.
//
// Reload all rule engine instances in the default rule chain pool.
//
// Parameters:
// Parameters:
//   - opts: Optional configuration functions for the rule engines
//
// Usage:
// Usage:
//
//	Reload(types.WithRuleEnginePool(pool))
func Reload(opts ...types.RuleEngineOption) {
	DefaultPool.entries.Range(func(key, value any) bool {
		_ = value.(types.RuleEngine).Reload(opts...)
		return true
	})
}

// Range iterates over all rule engine instances in the default rule chain pool.
//
// Range traverses all rule engine instances in the default rule chain pool.
//
// Parameters:
// Parameters:
//   - f: Function to apply to each rule engine instance
//
// Usage:
// Usage:
//
//	Range(func(key, value any) bool {
//	  // Use key and value as needed
//	  return true
//	})
func Range(f func(key, value any) bool) {
	DefaultPool.entries.Range(f)
}
