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

package external

import (
	"errors"
	"strings"

	"github.com/rulego/rulego/utils/json"

	"github.com/rulego/rulego/utils/el"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// Register the node
func init() {
	Registry.Add(&CacheGetNode{})
	Registry.Add(&CacheSetNode{})
	Registry.Add(&CacheDeleteNode{})
}

const (
	CacheLevelChain                = "chain"
	CacheLevelGlobal               = "global"
	CacheOutputModeMergeToMetadata = 0   // Merge into the current message metadata
	CacheOutputModeMergeToMsg      = 1   //Merge into the current message load
	CacheOutputModeNewMsg          = 2   //Override the original message load output
	KeyMatchAll                    = "*" //wildcard
)

// LevelKey cache key
type LevelKey struct {
	Level string `json:"level" label:"Level" desc:"Cache level"`
	Key   string `json:"key" label:"Key" desc:"Cache key, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
}

// LevelKeyTemplate caches key templates
type LevelKeyTemplate struct {
	level       string      // Cache level
	keyTemplate el.Template // Cache key template
}

const (
	// WhenKeyNotFoundFailure If the key cannot be found, the chain of failure occurs
	WhenKeyNotFoundFailure = "failure"
	// WhenKeyNotFoundSuccess: If the key cannot be found, proceed to the success chain
	WhenKeyNotFoundSuccess = "success"
)

// CacheGetNodeConfiguration retrieves the node configuration from the cache
type CacheGetNodeConfiguration struct {
	Keys            []LevelKey `json:"keys" label:"Keys" desc:"Cache keys to query" required:"true"`
	OutputMode      int        `json:"outputMode" label:"Output Mode" desc:"0=merge to metadata, 1=replace msg"`
	WhenKeyNotFound string     `json:"whenKeyNotFound" label:"When Not Found" desc:"Behavior when key not found: ignore, error, default(return empty)"`
}

// CacheGetNode retrieves data from cache storage at different levels (chain or global).
// It supports wildcard pattern matching and multiple output modes for flexible data integration.
//
// CacheGetNode retrieves data from cache storage at different levels (chain-level or global).
// Supports wildcard pattern matching and multiple output modes for flexible data integration.
//
// Configuration:
// Configuration:
//
//	{
//		"keys": [                        // Keys to retrieve
//			{
//				"level": "chain",        // Cache level: "chain" or "global"
//				"key": "sensor_${metadata.deviceId}"  // Key with variable substitution
//			},
//			{
//				"level": "global",
//				"key": "config_*"        // Wildcard pattern for multiple keys
//			}
//		],
//		"outputMode": 0                  // Output mode: 0=metadata, 1=merge to msg, 2=replace msg
//	}
//
// Cache Levels:
// Cache level:
//
//   - "chain": Rule chain scoped cache for data sharing within the same rule chain instance
//     Rule chain-level caching, used for data sharing within the same rule chain instance
//   - "global": Global cache for cross-chain data sharing across all rule chain instances
//     Global caching, used for cross-chain data sharing among all rule chain instances
//
// Key Pattern Matching:
// Key mode matching:
//
//   - Exact match: "user:123" retrieves specific key
//   - Wildcard: "user:*" retrieves all keys with prefix "user:"
//   - Variable substitution: "data_${metadata.id}" uses runtime values
//
// Output Modes:
// Output Mode:
//
//   - 0 (CacheOutputModeMergeToMetadata): Merge results to message metadata
//     Merge results into message metadata
//   - 1 (CacheOutputModeMergeToMsg): Merge to message payload (requires JSON data type)
//     Merge into message load (requires JSON data type)
//   - 2 (CacheOutputModeNewMsg): Replace message payload with cache results
//     Replace message loads with cached results
//
// Usage Example:
// Example:
//
//	// Retrieve user session and global config
//	Retrieve user sessions and global configurations
//	{
//		"id": "cacheGet",
//		"type": "cacheGet",
//		"configuration": {
//			"keys": [
//				{"level": "chain", "key": "session_${metadata.userId}"},
//				{"level": "global", "key": "app_config_*"}
//			],
//			"outputMode": 1
//		}
//	}
type CacheGetNode struct {
	//Node configuration
	Config CacheGetNodeConfiguration
	//keys template
	keysTemplate []LevelKeyTemplate
}

func (x *CacheGetNode) Type() string {
	return "cacheGet"
}

func (x *CacheGetNode) New() types.Node {
	return &CacheGetNode{Config: CacheGetNodeConfiguration{
		Keys: []LevelKey{
			{Level: CacheLevelChain, Key: "key1"},
		},
		OutputMode: CacheOutputModeNewMsg,
	}}
}

// Init initializes the component
func (x *CacheGetNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	//Initialize the keys template
	for _, item := range x.Config.Keys {
		template, err := el.NewTemplate(item.Key)
		if err != nil {
			return err
		}
		x.keysTemplate = append(x.keysTemplate, LevelKeyTemplate{
			level:       item.Level,
			keyTemplate: template,
		})
	}

	return nil
}

func (x *CacheGetNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	env := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	//Handle keys templates
	var keys []LevelKey
	for _, item := range x.keysTemplate {
		keys = append(keys, LevelKey{Level: item.level, Key: item.keyTemplate.ExecuteAsString(env)})
	}
	x.handleGet(ctx, msg, keys)
}

// Destroy releases component resources
func (x *CacheGetNode) Destroy() {
}

func (x *CacheGetNode) handleGet(ctx types.RuleContext, msg types.RuleMsg, keys []LevelKey) {
	values := make(map[string]interface{})
	var c types.Cache
	for _, item := range keys {
		if item.Level == CacheLevelGlobal {
			c = ctx.GlobalCache()
		} else {
			c = ctx.ChainCache()
		}
		if strings.HasSuffix(item.Key, KeyMatchAll) {
			matchValues := c.GetByPrefix(item.Key[:len(item.Key)-1])
			for k, v := range matchValues {
				values[k] = v
			}
		} else {
			value, err := c.Get(item.Key)
			if err != nil {
				if errors.Is(err, types.ErrCacheMiss) {
					// cache miss, controlled by whenKeyNotFound
					values[item.Key] = nil
				} else {
					// Errors at the bottom level always go down the chain of failure
					ctx.TellFailure(msg, err)
					return
				}
			} else {
				values[item.Key] = value
			}
		}
	}
	x.outputResult(ctx, msg, values)
}

func (x *CacheGetNode) outputResult(ctx types.RuleContext, msg types.RuleMsg, values map[string]interface{}) {
	// Check if all are missing
	var notFound = true
	for _, v := range values {
		if v != nil {
			notFound = false
			break
		}
	}

	if notFound {
		whenKeyNotFound := strings.ToLower(x.Config.WhenKeyNotFound)
		switch whenKeyNotFound {
		case WhenKeyNotFoundFailure:
			ctx.TellFailure(msg, types.ErrCacheMiss)
			return
		case WhenKeyNotFoundSuccess:
			// Follow the chain of success, continuing the output logic below
		default:
			// Null value: Maintains the original behavior, Mode 2 follows the failure chain, other modes follow the success chain
			if x.Config.OutputMode == CacheOutputModeNewMsg {
				ctx.TellFailure(msg, types.ErrCacheMiss)
				return
			}
		}
	}

	if x.Config.OutputMode == CacheOutputModeMergeToMetadata {
		for key, value := range values {
			msg.Metadata.PutValue(key, str.ToString(value))
		}
		ctx.TellSuccess(msg)
	} else if x.Config.OutputMode == CacheOutputModeMergeToMsg {
		if msg.DataType == types.JSON {
			var dataMap map[string]interface{}
			if err := json.Unmarshal([]byte(msg.GetData()), &dataMap); err == nil {
				for key, value := range values {
					dataMap[key] = value
				}
				msg.SetData(str.ToString(dataMap))
				ctx.TellSuccess(msg)
			} else {
				ctx.TellFailure(msg, errors.New("data must be able to be serialized into a map structure"))
			}
		} else {
			ctx.TellFailure(msg, errors.New("data type must be JSON type"))
		}
	} else {
		msg.SetData(str.ToString(values))
		ctx.TellSuccess(msg)
	}
}

// CacheSetNodeConfiguration cache node configuration
type CacheSetNodeConfiguration struct {
	Items []CacheItem `json:"items" label:"Items" desc:"Cache items to set" required:"true"`
}

type CacheItem struct {
	Level string      `json:"level" label:"Level" desc:"Cache level"`
	Key   string      `json:"key" label:"Key" desc:"Cache key, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
	Value interface{} `json:"value" label:"Value" desc:"Cache value, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
	Ttl   string      `json:"ttl" label:"TTL" desc:"Cache TTL, e.g. 10s, 5m, 1h"`
}

type CacheItemTemplate struct {
	level         string
	keyTemplate   el.Template
	valueTemplate el.Template
	ttl           string
}

// CacheSetNode stores data in cache storage at different levels with TTL support.
// It supports multiple cache items, variable substitution, and automatic expiration.
//
// CacheSetNode stores data in different levels of cache and supports TTL.
// Supports multiple cache items, variable replacement, and automatic expiration.
//
// Configuration:
// Configuration:
//
//	{
//		"items": [                       // Cache items to set
//			{
//				"level": "chain",        // Cache level: "chain" or "global"
//				"key": "user_${metadata.userId}",     // Key with variable substitution
//				"value": "${msg.userData}",           // Value with variable substitution
//				"ttl": "1h30m" // TTL format: 1h30m, 10m, 30s, empty=no expiration TTL format
//			}
//		]
//	}
//
// TTL Format:
// TTL format:
//
//   - "1h": 1 hour 1 hour
//   - "30m": 30 minutes
//   - "10s": 10 seconds
//   - "1h30m": 1 hour 30 minutes
//   - "": No expiration (permanent storage)
//
// Variable Substitution:
// Variable Substitution:
//
// Both keys and values support runtime variable substitution:
// Both keys and values support runtime variable substitution:
//   - ${metadata.key}: Access message metadata
//   - ${msg.key}: Access message payload fields
//
// Usage Example:
// Example:
//
//	// Store user session with 1-hour expiration
//	Stored user sessions, expires after 1 hour
//	{
//		"id": "cacheSet",
//		"type": "cacheSet",
//		"configuration": {
//			"items": [
//				{
//					"level": "chain",
//					"key": "session_${metadata.userId}",
//					"value": "${msg.sessionData}",
//					"ttl": "1h"
//				},
//				{
//					"level": "global",
//					"key": "last_activity_${metadata.userId}",
//					"value": "${metadata.timestamp}",
//					"ttl": "24h"
//				}
//			]
//		}
//	}
type CacheSetNode struct {
	// Node configuration
	Config CacheSetNodeConfiguration
	// Cache item list template
	itemsTemplate []CacheItemTemplate
	// Does the cached item list template contain variables?
	hasVar bool
}

// Type returns the component type
func (x *CacheSetNode) Type() string {
	return "cacheSet"
}

func (x *CacheSetNode) New() types.Node {
	return &CacheSetNode{Config: CacheSetNodeConfiguration{
		Items: []CacheItem{
			{Level: CacheLevelChain, Key: "key1", Value: "value1", Ttl: "1h"},
		},
	}}
}

// Init initializes the component
func (x *CacheSetNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	var hasVar = false
	//Initialize the cache item list template
	for _, item := range x.Config.Items {
		keyTemplate, err := el.NewTemplate(item.Key)
		if err != nil {
			return err
		}

		if keyTemplate.HasVar() {
			hasVar = true
		}

		valueTemplate, err := el.NewTemplate(item.Value)
		if err != nil {
			return err
		}
		if valueTemplate.HasVar() {
			hasVar = true
		}
		x.itemsTemplate = append(x.itemsTemplate, CacheItemTemplate{
			level:         item.Level,
			keyTemplate:   keyTemplate,
			valueTemplate: valueTemplate,
			ttl:           item.Ttl,
		})
	}
	x.hasVar = hasVar
	return nil
}

func (x *CacheSetNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var evn map[string]interface{}
	if x.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	var err error
	var value interface{}
	for _, item := range x.itemsTemplate {
		key := item.keyTemplate.ExecuteAsString(evn)
		if key == "" {
			err = errors.New("key is empty")
			break
		}
		value, err = item.valueTemplate.Execute(evn)
		if err == nil {
			var c = ctx.GlobalCache()
			if item.level == CacheLevelGlobal {
				c = ctx.GlobalCache()
			} else {
				c = ctx.ChainCache()
			}
			err = c.Set(key, value, item.ttl)
		} else {
			break
		}
	}

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		ctx.TellSuccess(msg)
	}
}

// Destroy releases component resources
func (x *CacheSetNode) Destroy() {
}

// CacheDeleteNodeConfiguration Cache delete node configuration
type CacheDeleteNodeConfiguration struct {
	Keys []LevelKey `json:"keys" label:"Keys" desc:"Cache keys to delete" required:"true"`
}

// CacheDeleteNode removes data from cache storage at different levels.
// It supports exact key deletion and prefix-based batch deletion with wildcard patterns.
//
// CacheDeleteNode deletes data from cache storage at different levels.
// Supports precise key deletion and batch deletion of wildcards based on prefixes.
//
// Configuration:
// Configuration:
//
//	{
//		"keys": [                        // Keys to delete
//			{
//				"level": "chain",        // Cache level: "chain" or "global"
//				"key": "session_${metadata.userId}"  // Exact key with variable substitution
//			},
//			{
//				"level": "global",
//				"key": "temp_*"          // Wildcard pattern for batch deletion
//			}
//		]
//	}
//
// Deletion Patterns:
// Delete Mode:
//
//   - Exact deletion: "user:123" removes specific key
//   - Batch deletion: "session:*" removes all keys with prefix "session:"
//   - Variable substitution: "cache_${metadata.id}" uses runtime values
//
// Usage Example:
// Example:
//
//	// Clean up user session and temporary data
//	Clean up user sessions and temporary data
//	{
//		"id": "cacheDelete",
//		"type": "cacheDelete",
//		"configuration": {
//			"keys": [
//				{"level": "chain", "key": "session_${metadata.userId}"},
//				{"level": "global", "key": "temp_${metadata.requestId}_*"}
//			]
//		}
//	}
type CacheDeleteNode struct {
	//Node configuration
	Config CacheDeleteNodeConfiguration
	//keys template
	keysTemplate []LevelKeyTemplate
}

func (x *CacheDeleteNode) Type() string {
	return "cacheDelete"
}

func (x *CacheDeleteNode) New() types.Node {
	return &CacheDeleteNode{Config: CacheDeleteNodeConfiguration{
		Keys: []LevelKey{
			{Level: CacheLevelChain, Key: "key1"},
		},
	}}
}

// Init initializes the component
func (x *CacheDeleteNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	//Initialize the keys template
	for _, item := range x.Config.Keys {
		template, err := el.NewMixedTemplate(item.Key)
		if err != nil {
			return err
		}
		x.keysTemplate = append(x.keysTemplate, LevelKeyTemplate{
			level:       item.Level,
			keyTemplate: template,
		})
	}

	return nil
}

func (x *CacheDeleteNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	env := base.NodeUtils.GetEvnAndMetadata(ctx, msg)

	//Handle keys templates
	var keys []LevelKey
	for _, item := range x.keysTemplate {
		keys = append(keys, LevelKey{Level: item.level, Key: item.keyTemplate.ExecuteAsString(env)})
	}

	x.handleDelete(ctx, msg, keys)
}

func (x *CacheDeleteNode) handleDelete(ctx types.RuleContext, msg types.RuleMsg, keys []LevelKey) {
	var c types.Cache
	for _, item := range keys {
		if item.Level == CacheLevelGlobal {
			c = ctx.GlobalCache()
		} else {
			c = ctx.ChainCache()
		}
		if strings.HasSuffix(item.Key, "*") {
			if err := c.DeleteByPrefix(item.Key[:len(item.Key)-1]); err != nil {
				ctx.TellFailure(msg, err)
				return
			}
		} else if err := c.Delete(item.Key); err != nil {
			ctx.TellFailure(msg, err)
			return
		}
	}
	ctx.TellSuccess(msg)
}

// Destroy releases component resources
func (x *CacheDeleteNode) Destroy() {
}

// Desc returns the component description
func (x *CacheGetNode) Desc() string {
	return "Retrieve data from chain/global cache. Supports wildcard keys (*), variable substitution. outputMode: 0=metadata, 1=merge to msg, 2=replace msg. Routes to Success/Failure"
}

// Desc returns the component description
func (x *CacheSetNode) Desc() string {
	return "Store data in chain/global cache with TTL. Keys and values support ${metadata.key} and ${msg.key} substitution. TTL format: 1h, 30m, 10s. Routes to Success/Failure"
}

// Desc returns the component description
func (x *CacheDeleteNode) Desc() string {
	return "Delete data from chain/global cache. Supports exact keys and wildcard prefix deletion (*). Routes to Success/Failure"
}
