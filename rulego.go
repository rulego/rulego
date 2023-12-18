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

// Package rulego provides a lightweight, high-performance, embedded, orchestrable component-based rule engine.
//
// # Usage
//
// Implement your business requirements by configuring components in the rule chain, and support dynamic modification. Rule chain definition format:
//
//	{
//		  "ruleChain": {
//			"id":"rule01"
//		  },
//		  "metadata": {
//		    "nodes": [
//		    ],
//			"connections": [
//			]
//		 }
//	}
//
// nodes:configure components. You can use built-in components and third-party extension components without writing any code.
//
// connections:configure the relation type between components. Determine the data flow.
//
// Example:
//
//	var ruleFile = `
//	{
//		"ruleChain": {
//		"id":"rule02",
//		"name": "test",
//		"root": true
//		},
//		"metadata": {
//		"nodes": [
//			{
//			"id": "s1",
//			"type": "jsTransform",
//			"name": "transform",
//			"debugMode": true,
//			"configuration": {
//				"jsScript": "metadata['state']='modify by js';\n msg['addField']='addValueFromJs'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
//				}
//			},
//			{
//				"id": "s2",
//				"type": "restApiCall",
//				"name": "push data",
//				"debugMode": true,
//				"configuration": {
//					"restEndpointUrlPattern": "http://127.0.0.1:9090/api/msg",
//					"requestMethod": "POST",
//				}
//			}
//		],
//		"connections": [
//			{
//				"fromId": "s1",
//				"toId": "s2",
//				"type": "Success"
//			}
//		]
//		}
//	}
//	`
//
// Create Rule Engine Instance
//
//	ruleEngine, err := rulego.New("rule01", []byte(ruleFile))
//
// Define Message Metadata
//
//	metaData := types.NewMetadata()
//	metaData.PutValue("productType", "test01")
//
// Define Message Payload And Type
//
//	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")
//
// Processing Message
//
//	ruleEngine.OnMsg(msg)
//
// Update Rule Chain
//
//	err := ruleEngine.ReloadSelf([]byte(ruleFile))
//
// Load All Rule Chain
//
//	err := ruleEngine.Load("./rulechains")
//
// Get Engine Instance
//
//	ruleEngine, ok := rulego.Get("rule01")
package rulego

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"strings"
	"sync"
)

var DefaultRuleGo = &RuleGo{}

// RuleGo 规则引擎实例池
type RuleGo struct {
	ruleEngines sync.Map
}

// Load 加载指定文件夹及其子文件夹所有规则链配置（与.json结尾文件），到规则引擎实例池
// 规则链ID，使用规则链文件配置的ruleChain.id
func (g *RuleGo) Load(folderPath string, opts ...RuleEngineOption) error {
	if !strings.HasSuffix(folderPath, "*.json") && !strings.HasSuffix(folderPath, "*.JSON") {
		if strings.HasSuffix(folderPath, "/") || strings.HasSuffix(folderPath, "\\") {
			folderPath = folderPath + "*.json"
		} else if folderPath == "" {
			folderPath = "./*.json"
		} else {
			folderPath = folderPath + "/*.json"
		}
	}
	paths, err := fs.GetFilePaths(folderPath)
	if err != nil {
		return err
	}
	for _, path := range paths {
		b := fs.LoadFile(path)
		if b != nil {
			if _, err = g.New("", b, opts...); err != nil {
				return err
			}
		}
	}
	return nil
}

// New 创建一个新的RuleEngine并将其存储在RuleGo规则链池中
// 如果指定id="",则使用规则链文件的ruleChain.id
func (g *RuleGo) New(id string, rootRuleChainSrc []byte, opts ...RuleEngineOption) (*RuleEngine, error) {
	if v, ok := g.ruleEngines.Load(id); ok {
		return v.(*RuleEngine), nil
	} else {
		if ruleEngine, err := newRuleEngine(id, rootRuleChainSrc, opts...); err != nil {
			return nil, err
		} else {
			if ruleEngine.Id != "" {
				// Store the new RuleEngine in the ruleEngines map with the Id as the key.
				g.ruleEngines.Store(ruleEngine.Id, ruleEngine)
			}
			ruleEngine.RuleChainPool = g
			return ruleEngine, err
		}

	}
}

// Get 获取指定ID规则引擎实例
func (g *RuleGo) Get(id string) (*RuleEngine, bool) {
	v, ok := g.ruleEngines.Load(id)
	if ok {
		return v.(*RuleEngine), ok
	} else {
		return nil, false
	}
}

// Del 删除指定ID规则引擎实例
func (g *RuleGo) Del(id string) {
	v, ok := g.ruleEngines.Load(id)
	if ok {
		v.(*RuleEngine).Stop()
		g.ruleEngines.Delete(id)
	}
}

// Stop 释放所有规则引擎实例
func (g *RuleGo) Stop() {
	g.ruleEngines.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.Stop()
		}
		g.ruleEngines.Delete(key)
		return true
	})
}

// OnMsg 调用所有规则引擎实例处理消息
// 规则引擎实例池所有规则链都会去尝试处理该消息
func (g *RuleGo) OnMsg(msg types.RuleMsg) {
	g.ruleEngines.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.OnMsg(msg)
		}
		return true
	})
}

// Load 加载指定文件夹及其子文件夹所有规则链配置（与.json结尾文件），到规则引擎实例池
// 规则链ID，使用文件配置的 ruleChain.id
func Load(folderPath string, opts ...RuleEngineOption) error {
	return DefaultRuleGo.Load(folderPath, opts...)
}

// New 创建一个新的RuleEngine并将其存储在RuleGo规则链池中
func New(id string, rootRuleChainSrc []byte, opts ...RuleEngineOption) (*RuleEngine, error) {
	return DefaultRuleGo.New(id, rootRuleChainSrc, opts...)
}

// Get 获取指定ID规则引擎实例
func Get(id string) (*RuleEngine, bool) {
	return DefaultRuleGo.Get(id)
}

// Del 删除指定ID规则引擎实例
func Del(id string) {
	DefaultRuleGo.Del(id)
}

// Stop 释放所有规则引擎实例
func Stop() {
	DefaultRuleGo.Stop()
}

// OnMsg 调用所有规则引擎实例处理消息
// 规则引擎实例池所有规则链都会去尝试处理该消息
func OnMsg(msg types.RuleMsg) {
	DefaultRuleGo.OnMsg(msg)
}
