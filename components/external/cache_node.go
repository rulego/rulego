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
	// Level 缓存级别，chain或global
	// chain: 规则链缓存，在当前规则链命名空间下操作，用于规则链实例内不同执行上下文之间的数据共享。如果规则链实例被销毁，会自动删除该规则链命名空间下所有缓存
	// global: 全局缓存，在全局命名空间下操作，用于跨规则链间的数据共享
	Level string `json:"level"`
	// 可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	// 支持 * 通配符查找，例如：test:* 表示以test: 开头的所有key
	Key string `json:"key"`
}

// LevelKeyTemplate 缓存key模板
type LevelKeyTemplate struct {
	level       string            // 缓存级别
	keyTemplate *el.MixedTemplate // 缓存key模板
}

// CacheGetNodeConfiguration 缓存获取节点配置
type CacheGetNodeConfiguration struct {
	// Keys 获取key列表
	Keys []LevelKey `json:"keys"`
	// OutputMode 输出模式
	// 0:查询结果，合并到当前消息元数据
	// 1:查询结果，合并到当前消息负荷。要求输入消息负荷`DataType`必须是JSON类型，并且消息负荷`Data`可以解析为map结构
	// 2:查询结果，转成JSON，覆盖原消息负荷输出
	OutputMode int `json:"outputMode"`
}

// CacheGetNode 缓存获取节点
// global: 全局缓存，在全局缓存实例中操作
// chain: 规则链缓存，在当前规则链命名空间下操作
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
			value := c.Get(item.Key)
			values[item.Key] = value
		}
	}
	x.outputResult(ctx, msg, values)
}

func (x *CacheGetNode) outputResult(ctx types.RuleContext, msg types.RuleMsg, values map[string]interface{}) {

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
	// Items 缓存项列表
	Items []CacheItem `json:"items"`
}

type CacheItem struct {
	// Level 缓存级别，chain或global
	// chain: 规则链缓存，在当前规则链命名空间下操作，用于规则链实例内不同执行上下文之间的数据共享。如果规则链实例被销毁，会自动删除该规则链命名空间下所有缓存
	// global: 全局缓存，在全局命名空间下操作，用于跨规则链间的数据共享
	Level string `json:"level"`
	//  key 键
	// 可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Key string `json:"key"`
	// value 值
	// 可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Value interface{} `json:"value"`
	// Ttl 过期时间
	// 示例：1h(1小时) 1h30m(1小时30分钟) 10m(10分钟) 10s(10秒)，如果为空或者0，则表示永不过期
	Ttl string `json:"ttl"`
}

type CacheItemTemplate struct {
	level         string
	keyTemplate   *el.MixedTemplate
	valueTemplate el.Template
	ttl           string
}

//	CacheSetNode 缓存设置节点
//
// 缓存实例通过 type.Cache 设置
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
		keyTemplate, err := el.NewMixedTemplate(item.Key)
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
	// Keys 删除的键列表
	Keys []LevelKey `json:"keys"`
}

// CacheDeleteNode 缓存删除节点
// 缓存实例通过 type.Cache 设置
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
