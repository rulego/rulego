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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"log"
	"strings"
	"sync"
)

var _ types.RuleEnginePool = (*Pool)(nil)

var DefaultPool = &Pool{}

// Pool is a pool of rule engine instances.
type Pool struct {
	// A concurrent map to store rule engine instances.
	entries sync.Map
}

// NewPool creates a new instance of a rule engine pool.
func NewPool() *Pool {
	return &Pool{}
}

// Load loads all rule chain configurations from a specified folder and its subfolders into the rule engine instance pool.
// The rule chain ID is taken from the configuration file's ruleChain.id.
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
			if _, err = g.New("", b, opts...); err != nil {
				log.Println("Load rule chain error:", err)
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
			return ruleEngine, err
		}

	}
}

// Get retrieves a rule engine instance by its ID.
func (g *Pool) Get(id string) (types.RuleEngine, bool) {
	v, ok := g.entries.Load(id)
	if ok {
		return v.(*RuleEngine), ok
	} else {
		return nil, false
	}
}

// Del deletes a rule engine instance by its ID.
func (g *Pool) Del(id string) {
	v, ok := g.entries.Load(id)
	if ok {
		v.(*RuleEngine).Stop()
		g.entries.Delete(id)
	}
}

// Stop releases all rule engine instances in the pool.
func (g *Pool) Stop() {
	g.entries.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.Stop()
		}
		g.entries.Delete(key)
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

// Load loads all rule chain configurations from the specified folder and its subfolders into the default rule engine instance pool.
// The rule chain ID is taken from the configuration file's ruleChain.id.
func Load(folderPath string, opts ...types.RuleEngineOption) error {
	return DefaultPool.Load(folderPath, opts...)
}

// New creates a new RuleEngine and stores it in the default rule chain pool.
func New(id string, rootRuleChainSrc []byte, opts ...types.RuleEngineOption) (types.RuleEngine, error) {
	return DefaultPool.New(id, rootRuleChainSrc, opts...)
}

// Get retrieves a specified ID rule engine instance from the default rule chain pool.
func Get(id string) (types.RuleEngine, bool) {
	return DefaultPool.Get(id)
}

// Del deletes a specified ID rule engine instance from the default rule chain pool.
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop releases all rule engine instances in the default rule chain pool.
func Stop() {
	DefaultPool.Stop()
}

// OnMsg calls all rule engine instances in the default rule chain pool to process a message.
// All rule chains in the rule engine instance pool will attempt to process the message.
func OnMsg(msg types.RuleMsg) {
	DefaultPool.OnMsg(msg)
}

// Reload reloads all rule engine instances in the default rule chain pool.
func Reload(opts ...types.RuleEngineOption) {
	DefaultPool.entries.Range(func(key, value any) bool {
		_ = value.(types.RuleEngine).Reload(opts...)
		return true
	})
}

// Range iterates over all rule engine instances in the default rule chain pool.
func Range(f func(key, value any) bool) {
	DefaultPool.entries.Range(f)
}
