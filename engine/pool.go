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
// DefaultPool 是规则引擎池的默认全局实例。
// 它提供单例池来管理整个应用程序中的规则引擎实例。
var DefaultPool = &Pool{}

// Pool is a pool of rule engine instances.
// It provides centralized management of multiple rule engines, enabling efficient
// resource sharing, batch operations, and coordinated lifecycle management.
//
// Pool 是规则引擎实例的池。
// 它提供多个规则引擎的集中管理，支持高效的资源共享、批量操作和协调的生命周期管理。
//
// Key Features:
// 主要特性：
//   - Concurrent-safe rule engine storage using sync.Map  使用 sync.Map 的并发安全规则引擎存储
//   - Automatic rule chain loading from filesystem  从文件系统自动加载规则链
//   - Callback-based lifecycle events  基于回调的生命周期事件
//   - Batch operations across multiple engines  跨多个引擎的批量操作
//   - Dynamic engine creation and management  动态引擎创建和管理
//
// Use Cases:
// 使用场景：
//   - Multi-tenant rule engine management  多租户规则引擎管理
//   - Rule chain hot reloading and deployment  规则链热重载和部署
//   - Distributed rule processing coordination  分布式规则处理协调
//   - Resource sharing between related rule chains  相关规则链之间的资源共享
type Pool struct {
	// entries is a concurrent map to store rule engine instances.
	// Uses string keys (rule engine IDs) and *RuleEngine values.
	// entries 是存储规则引擎实例的并发映射。
	// 使用字符串键（规则引擎 ID）和 *RuleEngine 值。
	entries sync.Map

	// Callbacks provides hooks for rule engine lifecycle events,
	// enabling custom handling of creation, updates, and deletion.
	// Callbacks 为规则引擎生命周期事件提供钩子，
	// 支持创建、更新和删除的自定义处理。
	Callbacks types.Callbacks
}

// NewPool creates a new instance of a rule engine pool.
// This function initializes an empty pool ready for use.
//
// NewPool 创建规则引擎池的新实例。
// 此函数初始化一个准备使用的空池。
//
// Returns:
// 返回：
//   - *Pool: New pool instance  新的池实例
//
// Usage:
// 使用：
//
//	pool := NewPool()
//	engine, err := pool.New("engine1", ruleChainBytes)
func NewPool() *Pool {
	return &Pool{}
}

// Load loads all rule chain configurations from a specified folder and its subfolders into the rule engine instance pool.
// The rule chain ID is taken from the configuration file's ruleChain.id.
//
// Load 从指定文件夹及其子文件夹加载所有规则链配置到规则引擎实例池中。
// 规则链 ID 取自配置文件的 ruleChain.id。
//
// Parameters:
// 参数：
//   - folderPath: Path to the folder containing rule chain files  包含规则链文件的文件夹路径
//   - opts: Optional configuration functions for the rule engines  规则引擎的可选配置函数
//
// Returns:
// 返回：
//   - error: Loading error if any  如果有的话，加载错误
//
// File Processing:
// 文件处理：
//   - Supports JSON files (*.json, *.JSON)  支持 JSON 文件（*.json、*.JSON）
//   - Recursively processes subdirectories  递归处理子目录
//   - Uses glob patterns for file matching  使用 glob 模式进行文件匹配
//   - Automatically extracts rule chain ID from file content  自动从文件内容提取规则链 ID
//
// Error Handling:
// 错误处理：
//   - Individual file errors are logged but don't stop the overall process
//     单个文件错误会被记录但不会停止整个过程
//   - Returns error only for critical failures like invalid folder path
//     仅对关键故障（如无效文件夹路径）返回错误
//
// Callback Integration:
// 回调集成：
//   - Triggers OnNew callback for each successfully loaded rule chain
//     为每个成功加载的规则链触发 OnNew 回调
//   - Enables custom processing and validation of loaded chains
//     支持已加载链的自定义处理和验证
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
		if g.Callbacks.OnDeleted != nil {
			g.Callbacks.OnDeleted(id)
		}
	}
}

// Stop releases all rule engine instances in the pool.
func (g *Pool) Stop() {
	g.entries.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.Stop()
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
// Load 从指定文件夹及其子文件夹加载所有规则链配置到默认规则引擎实例池中。
// 规则链 ID 取自配置文件的 ruleChain.id。
//
// Parameters:
// 参数：
//   - folderPath: Path to the folder containing rule chain files  包含规则链文件的文件夹路径
//   - opts: Optional configuration functions for the rule engines  规则引擎的可选配置函数
//
// Returns:
// 返回：
//   - error: Loading error if any  如果有的话，加载错误
//
// Usage:
// 使用：
//
//	err := Load("path/to/rulechains", types.WithRuleEnginePool(pool))
func Load(folderPath string, opts ...types.RuleEngineOption) error {
	return DefaultPool.Load(folderPath, opts...)
}

// New creates a new RuleEngine and stores it in the default rule chain pool.
//
// New 创建新的规则引擎并将其存储在默认规则链池中。
//
// Parameters:
// 参数：
//   - id: ID of the rule engine  规则引擎的 ID
//   - rootRuleChainSrc: Raw bytes of the rule chain  规则链的原始字节
//   - opts: Optional configuration functions for the rule engine  规则引擎的可选配置函数
//
// Returns:
// 返回：
//   - types.RuleEngine: New rule engine instance  新的规则引擎实例
//   - error: Loading error if any  如果有的话，加载错误
//
// Usage:
// 使用：
//
//	engine, err := New("engine1", ruleChainBytes)
func New(id string, rootRuleChainSrc []byte, opts ...types.RuleEngineOption) (types.RuleEngine, error) {
	return DefaultPool.New(id, rootRuleChainSrc, opts...)
}

// Get retrieves a specified ID rule engine instance from the default rule chain pool.
//
// Get 从默认规则链池中检索指定的 ID 规则引擎实例。
//
// Parameters:
// 参数：
//   - id: ID of the rule engine  规则引擎的 ID
//
// Returns:
// 返回：
//   - types.RuleEngine: Rule engine instance  规则引擎实例
//   - bool: Existence of the rule engine  规则引擎的存在
//
// Usage:
// 使用：
//
//	engine, exists := Get("engine1")
func Get(id string) (types.RuleEngine, bool) {
	return DefaultPool.Get(id)
}

// Del deletes a specified ID rule engine instance from the default rule chain pool.
//
// Del 从默认规则链池中删除指定的 ID 规则引擎实例。
//
// Parameters:
// 参数：
//   - id: ID of the rule engine  规则引擎的 ID
//
// Usage:
// 使用：
//
//	Del("engine1")
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop releases all rule engine instances in the default rule chain pool.
//
// Stop 释放默认规则链池中的所有规则引擎实例。
//
// Usage:
// 使用：
//
//	Stop()
func Stop() {
	DefaultPool.Stop()
}

// OnMsg calls all rule engine instances in the default rule chain pool to process a message.
// All rule chains in the rule engine instance pool will attempt to process the message.
//
// OnMsg 调用默认规则链池中的所有规则引擎实例来处理消息。
// 所有规则引擎实例池中的所有规则链将尝试处理消息。
//
// Parameters:
// 参数：
//   - msg: Rule message to be processed  要处理的消息
//
// Usage:
// 使用：
//
//	OnMsg(ruleMsg)
func OnMsg(msg types.RuleMsg) {
	DefaultPool.OnMsg(msg)
}

// Reload reloads all rule engine instances in the default rule chain pool.
//
// Reload 重新加载默认规则链池中的所有规则引擎实例。
//
// Parameters:
// 参数：
//   - opts: Optional configuration functions for the rule engines  规则引擎的可选配置函数
//
// Usage:
// 使用：
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
// Range 遍历默认规则链池中的所有规则引擎实例。
//
// Parameters:
// 参数：
//   - f: Function to apply to each rule engine instance  要应用于每个规则引擎实例的函数
//
// Usage:
// 使用：
//
//	Range(func(key, value any) bool {
//	  // Use key and value as needed
//	  return true
//	})
func Range(f func(key, value any) bool) {
	DefaultPool.entries.Range(f)
}
