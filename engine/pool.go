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

// Pool 规则引擎实例池
type Pool struct {
	entries sync.Map
}

func NewPool() *Pool {
	return &Pool{}
}

// Load 加载指定文件夹及其子文件夹所有规则链配置（与.json结尾文件），到规则引擎实例池
// 规则链ID，使用规则链文件配置的ruleChain.id
func (g *Pool) Load(folderPath string, opts ...types.RuleEngineOption) error {
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
				log.Println("Load rule chain error:", err)
			}
		}
	}
	return nil
}

// New 创建一个新的RuleEngine并将其存储在RuleGo规则链池中
// 如果指定id="",则使用规则链文件的ruleChain.id
func (g *Pool) New(id string, rootRuleChainSrc []byte, opts ...types.RuleEngineOption) (types.RuleEngine, error) {
	if v, ok := g.entries.Load(id); ok {
		return v.(*RuleEngine), nil
	} else {
		if ruleEngine, err := newRuleEngine(id, rootRuleChainSrc, opts...); err != nil {
			return nil, err
		} else {
			if ruleEngine.Id() != "" {
				// Store the new RuleEngine in the entries map with the Id as the key.
				g.entries.Store(ruleEngine.Id(), ruleEngine)
			}
			ruleEngine.RuleChainPool = g
			return ruleEngine, err
		}

	}
}

// Get 获取指定ID规则引擎实例
func (g *Pool) Get(id string) (types.RuleEngine, bool) {
	v, ok := g.entries.Load(id)
	if ok {
		return v.(*RuleEngine), ok
	} else {
		return nil, false
	}
}

// Del 删除指定ID规则引擎实例
func (g *Pool) Del(id string) {
	v, ok := g.entries.Load(id)
	if ok {
		v.(*RuleEngine).Stop()
		g.entries.Delete(id)
	}
}

// Stop 释放所有规则引擎实例
func (g *Pool) Stop() {
	g.entries.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.Stop()
		}
		g.entries.Delete(key)
		return true
	})
}

// Range 遍历所有规则引擎实例
func (g *Pool) Range(f func(key, value any) bool) {
	g.entries.Range(f)
}

func (g *Pool) Reload(opts ...types.RuleEngineOption) {
	g.entries.Range(func(key, value any) bool {
		_ = value.(*RuleEngine).Reload(opts...)
		return true
	})
}

// OnMsg 调用所有规则引擎实例处理消息
// 规则引擎实例池所有规则链都会去尝试处理该消息
func (g *Pool) OnMsg(msg types.RuleMsg) {
	g.entries.Range(func(key, value any) bool {
		if item, ok := value.(*RuleEngine); ok {
			item.OnMsg(msg)
		}
		return true
	})
}

func (g *Pool) NewConfig(opts ...types.Option) types.Config {
	return NewConfig(opts...)
}

// Load 加载指定文件夹及其子文件夹所有规则链配置（与.json结尾文件），到规则引擎实例池
// 规则链ID，使用文件配置的 ruleChain.id
func Load(folderPath string, opts ...types.RuleEngineOption) error {
	return DefaultPool.Load(folderPath, opts...)
}

// New 创建一个新的RuleEngine并将其存储在RuleGo规则链池中
func New(id string, rootRuleChainSrc []byte, opts ...types.RuleEngineOption) (types.RuleEngine, error) {
	return DefaultPool.New(id, rootRuleChainSrc, opts...)
}

// Get 获取指定ID规则引擎实例
func Get(id string) (types.RuleEngine, bool) {
	return DefaultPool.Get(id)
}

// Del 删除指定ID规则引擎实例
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop 释放所有规则引擎实例
func Stop() {
	DefaultPool.Stop()
}

// OnMsg 调用所有规则引擎实例处理消息
// 规则引擎实例池所有规则链都会去尝试处理该消息
func OnMsg(msg types.RuleMsg) {
	DefaultPool.OnMsg(msg)
}

// Reload 重新加载所有规则引擎实例
func Reload(opts ...types.RuleEngineOption) {
	DefaultPool.entries.Range(func(key, value any) bool {
		_ = value.(types.RuleEngine).Reload(opts...)
		return true
	})
}

// Range 遍历所有规则引擎实例
func Range(f func(key, value any) bool) {
	DefaultPool.entries.Range(f)
}
