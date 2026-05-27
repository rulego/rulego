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

// 注册节点
func init() {
	Registry.Add(&CacheGetNode{})
	Registry.Add(&CacheSetNode{})
	Registry.Add(&CacheDeleteNode{})
}

const (
	CacheLevelChain                = "chain"
	CacheLevelGlobal               = "global"
	CacheOutputModeMergeToMetadata = 0   // 合并到当前消息元数据
	CacheOutputModeMergeToMsg      = 1   //合并到当前消息负荷
	CacheOutputModeNewMsg          = 2   //覆盖原消息负荷输出
	KeyMatchAll                    = "*" //通配符
)

// LevelKey 缓存key
type LevelKey struct {
	Level string `json:"level" label:"Level" desc:"Cache level"`
	Key   string `json:"key" label:"Key" desc:"Cache key, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
}

// LevelKeyTemplate 缓存key模板
type LevelKeyTemplate struct {
	level       string      // 缓存级别
	keyTemplate el.Template // 缓存key模板
}

const (
	// WhenKeyNotFoundFailure 找不到key时走失败链
	WhenKeyNotFoundFailure = "failure"
	// WhenKeyNotFoundSuccess 找不到key时走成功链
	WhenKeyNotFoundSuccess = "success"
)

// CacheGetNodeConfiguration 缓存获取节点配置
type CacheGetNodeConfiguration struct {
	Keys            []LevelKey `json:"keys" label:"Keys" desc:"Cache keys to query" required:"true"`
	OutputMode      int        `json:"outputMode" label:"Output Mode" desc:"0=merge to metadata, 1=replace msg"`
	WhenKeyNotFound string     `json:"whenKeyNotFound" label:"When Not Found" desc:"Behavior when key not found: ignore, error, default(return empty)"`
}

// CacheGetNode retrieves data from cache storage at different levels (chain or global).
// It supports wildcard pattern matching and multiple output modes for flexible data integration.
//
// CacheGetNode 从不同级别（链级或全局）的缓存存储中检索数据。
// 支持通配符模式匹配和多种输出模式以实现灵活的数据集成。
//
// Configuration:
// 配置说明：
//
//	{
//		"keys": [                        // Keys to retrieve  要检索的键列表
//			{
//				"level": "chain",        // Cache level: "chain" or "global"  缓存级别
//				"key": "sensor_${metadata.deviceId}"  // Key with variable substitution  支持变量替换的键
//			},
//			{
//				"level": "global",
//				"key": "config_*"        // Wildcard pattern for multiple keys  多键通配符模式
//			}
//		],
//		"outputMode": 0                  // Output mode: 0=metadata, 1=merge to msg, 2=replace msg  输出模式
//	}
//
// Cache Levels:
// 缓存级别：
//
//   - "chain": Rule chain scoped cache for data sharing within the same rule chain instance
//     规则链级缓存，用于同一规则链实例内的数据共享
//   - "global": Global cache for cross-chain data sharing across all rule chain instances
//     全局缓存，用于所有规则链实例间的跨链数据共享
//
// Key Pattern Matching:
// 键模式匹配：
//
//   - Exact match: "user:123" retrieves specific key  精确匹配：检索特定键
//   - Wildcard: "user:*" retrieves all keys with prefix "user:"  通配符：检索所有前缀为 "user:" 的键
//   - Variable substitution: "data_${metadata.id}" uses runtime values  变量替换：使用运行时值
//
// Output Modes:
// 输出模式：
//
//   - 0 (CacheOutputModeMergeToMetadata): Merge results to message metadata
//     合并结果到消息元数据
//   - 1 (CacheOutputModeMergeToMsg): Merge to message payload (requires JSON data type)
//     合并到消息负荷（需要 JSON 数据类型）
//   - 2 (CacheOutputModeNewMsg): Replace message payload with cache results
//     用缓存结果替换消息负荷
//
// Usage Example:
// 使用示例：
//
//	// Retrieve user session and global config
//	// 检索用户会话和全局配置
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
	//节点配置
	Config CacheGetNodeConfiguration
	//keys模板
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

// Init 初始化组件
func (x *CacheGetNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	//初始化keys模板
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
	//处理keys模板
	var keys []LevelKey
	for _, item := range x.keysTemplate {
		keys = append(keys, LevelKey{Level: item.level, Key: item.keyTemplate.ExecuteAsString(env)})
	}
	x.handleGet(ctx, msg, keys)
}

// Destroy 销毁组件
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
					// cache miss，由 whenKeyNotFound 控制
					values[item.Key] = nil
				} else {
					// 底层报错，始终走失败链
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
	// 检查是否全部 miss
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
			// 走成功链，继续下面的输出逻辑
		default:
			// 空值：保持原有行为，Mode 2 走失败链，其他模式走成功链
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

// CacheSetNodeConfiguration 缓存设置节点配置
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
// CacheSetNode 在不同级别的缓存存储中存储数据，支持 TTL。
// 支持多个缓存项、变量替换和自动过期。
//
// Configuration:
// 配置说明：
//
//	{
//		"items": [                       // Cache items to set  要设置的缓存项
//			{
//				"level": "chain",        // Cache level: "chain" or "global"  缓存级别
//				"key": "user_${metadata.userId}",     // Key with variable substitution  支持变量替换的键
//				"value": "${msg.userData}",           // Value with variable substitution  支持变量替换的值
//				"ttl": "1h30m"          // TTL format: 1h30m, 10m, 30s, empty=no expiration  TTL 格式
//			}
//		]
//	}
//
// TTL Format:
// TTL 格式：
//
//   - "1h": 1 hour  1小时
//   - "30m": 30 minutes  30分钟
//   - "10s": 10 seconds  10秒
//   - "1h30m": 1 hour 30 minutes  1小时30分钟
//   - "": No expiration (permanent storage)  无过期时间（永久存储）
//
// Variable Substitution:
// 变量替换：
//
// Both keys and values support runtime variable substitution:
// 键和值都支持运行时变量替换：
//   - ${metadata.key}: Access message metadata  访问消息元数据
//   - ${msg.key}: Access message payload fields  访问消息负荷字段
//
// Usage Example:
// 使用示例：
//
//	// Store user session with 1-hour expiration
//	// 存储用户会话，1小时过期
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
	// 节点配置
	Config CacheSetNodeConfiguration
	// 缓存项列表模板
	itemsTemplate []CacheItemTemplate
	// 缓存项列表模板是否有变量
	hasVar bool
}

// Type 返回组件类型
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

// Init 初始化组件
func (x *CacheSetNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	var hasVar = false
	//初始化缓存项列表模板
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

// Destroy 销毁组件
func (x *CacheSetNode) Destroy() {
}

// CacheDeleteNodeConfiguration 缓存删除节点配置
type CacheDeleteNodeConfiguration struct {
	Keys []LevelKey `json:"keys" label:"Keys" desc:"Cache keys to delete" required:"true"`
}

// CacheDeleteNode removes data from cache storage at different levels.
// It supports exact key deletion and prefix-based batch deletion with wildcard patterns.
//
// CacheDeleteNode 从不同级别的缓存存储中删除数据。
// 支持精确键删除和基于前缀的通配符批量删除。
//
// Configuration:
// 配置说明：
//
//	{
//		"keys": [                        // Keys to delete  要删除的键列表
//			{
//				"level": "chain",        // Cache level: "chain" or "global"  缓存级别
//				"key": "session_${metadata.userId}"  // Exact key with variable substitution  精确键支持变量替换
//			},
//			{
//				"level": "global",
//				"key": "temp_*"          // Wildcard pattern for batch deletion  批量删除的通配符模式
//			}
//		]
//	}
//
// Deletion Patterns:
// 删除模式：
//
//   - Exact deletion: "user:123" removes specific key  精确删除：删除特定键
//   - Batch deletion: "session:*" removes all keys with prefix "session:"  批量删除：删除所有前缀为 "session:" 的键
//   - Variable substitution: "cache_${metadata.id}" uses runtime values  变量替换：使用运行时值
//
// Usage Example:
// 使用示例：
//
//	// Clean up user session and temporary data
//	// 清理用户会话和临时数据
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
	//节点配置
	Config CacheDeleteNodeConfiguration
	//keys模板
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

// Init 初始化组件
func (x *CacheDeleteNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	//初始化keys模板
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

	//处理keys模板
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

// Destroy 销毁组件
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
