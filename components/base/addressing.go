/*
 * Copyright 2024 The RuleGo Authors.
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

package base

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/el"
)

// addressing.go 提供 ref:// 寻址推送的通用解析工具，供官方与第三方寻址型节点（net/ws 等）复用。
//
// 放在 components/base 而非 components/external：寻址解析是节点侧基础设施，第三方扩展寻址型
// 节点时本就要 import base（其节点嵌入 base.SharedNode），可避免为复用本工具而拉入整个
// components/external 及其全部组件依赖。寻址能力接口 TargetSender / ResourceLookup 定义在 api/types。

// ResolveRefEndpoint 解析 ref:// 目标实例：同链 ChainCtx.Resources() 优先 → NodePool 回退。
// 读路径均走 sync.Map（同链 resourceRegistry + NodePool entries），无锁、零分配。
func ResolveRefEndpoint(ctx types.RuleContext, pool types.NodePool, id string) (any, bool) {
	if chain, ok := ctx.RuleChain().(types.ChainCtx); ok {
		if inst, found := chain.Resources().Lookup(id); found {
			return inst, true
		}
	}
	if pool != nil {
		if inst, found := pool.Lookup(id); found {
			return inst, true
		}
	}
	return nil, false
}

// TargetResolver 预编译 ${} 表达式模板，提供按消息内容解析寻址目标的能力。
// 寻址型节点共用，避免每条消息重复编译模板。
type TargetResolver struct {
	template el.Template // Init 时编译，nil 表示纯字面量或空
	literal  string      // 原始配置值（空串 / 字面量 / * 等）
}

// NewTargetResolver 创建解析器。cfg 为配置中的 target 字段（支持 ${} 表达式或字面量）。
// 编译失败时降级为字面量（不报错，与原行为一致）。
func NewTargetResolver(cfg string) *TargetResolver {
	r := &TargetResolver{literal: cfg}
	if cfg == "" {
		return r
	}
	t, err := el.NewTemplate(cfg)
	if err == nil && t != nil {
		r.template = t
	}
	return r
}

// Resolve 从消息中解析寻址目标，复用 ctx.GetEnv 标准环境（msg/metadata/vars/global）。
// 返回空串表示表达式解析结果为空（调用方需判断是否允许空目标广播）。
func (r *TargetResolver) Resolve(ctx types.RuleContext, msg types.RuleMsg) string {
	if r.template == nil {
		return r.literal
	}
	return r.template.ExecuteAsString(NodeUtils.GetEvnAndMetadata(ctx, msg))
}

// Literal 返回原始配置值（用于错误消息等场景）。
func (r *TargetResolver) Literal() string {
	return r.literal
}

// IsEmpty 配置是否为空。
func (r *TargetResolver) IsEmpty() bool {
	return r.literal == ""
}
