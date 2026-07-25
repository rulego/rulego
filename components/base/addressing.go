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
	"errors"
	"fmt"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/el"
)

// addressing.go 提供 ref:// 寻址与连接借用的通用解析工具，供官方与第三方节点复用。
//
// 放在 components/base 而非 components/external：寻址解析是节点侧基础设施，第三方扩展
// 寻址/连接型节点时本就要 import base（其节点嵌入 base.SharedNode）。能力接口
// TargetSender / ResourceLookup 定义在 api/types。

var (
	// ErrResourceNotFound ref:// 目标在同链目录与 NodePool 均未找到。
	ErrResourceNotFound = errors.New("ref resource not found in chain or node pool")
	// ErrNotTargetSender ref:// 目标不支持寻址推送（非 TargetSender），调用方可据此走连接借用兜底。
	ErrNotTargetSender = errors.New("ref resource does not support addressing")
)

// ResolveResource 解析 ref:// 目标实例：reg 目录优先 → NodePool 回退。纯解析，读路径无锁、零分配。
func ResolveResource(reg types.ResourceLookup, pool types.NodePool, id string) (any, bool) {
	if reg != nil {
		if inst, found := reg.Lookup(id); found {
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

// ResolveResourceFromCtx 从消息 ctx 取所属链目录后解析（寻址型节点 net/ws 用，跟随消息链）。
func ResolveResourceFromCtx(ctx types.RuleContext, pool types.NodePool, id string) (any, bool) {
	if chain, ok := ctx.RuleChain().(types.ChainCtx); ok {
		return ResolveResource(chain.Resources(), pool, id)
	}
	return ResolveResource(nil, pool, id)
}

// LoadConn 解析 ref:// 目标并从 connHolder 取最新连接（连接持有型借用）。
// 命中但类型不符（跨类型 ref）或连接为零值，返回 (zero, false)，调用方据此回退或报错。
func LoadConn[T any](reg types.ResourceLookup, pool types.NodePool, id string) (T, bool) {
	v, found := ResolveResource(reg, pool, id)
	if !found {
		var zero T
		return zero, false
	}
	h, ok := v.(*connHolder[T])
	if !ok {
		var zero T
		return zero, false
	}
	c := h.load()
	if isZeroValue(c) {
		var zero T
		return zero, false
	}
	return c, true
}

// LoadConnFromCtx 从消息 ctx 取所属链目录后借用连接（net_node 的 net.Conn 借用兜底用）。
func LoadConnFromCtx[T any](ctx types.RuleContext, pool types.NodePool, id string) (T, bool) {
	if chain, ok := ctx.RuleChain().(types.ChainCtx); ok {
		return LoadConn[T](chain.Resources(), pool, id)
	}
	return LoadConn[T](nil, pool, id)
}

// SendToRefTarget 解析 ref:// 目标并按 target 寻址推送（寻址型节点 net/ws 共用）。
// 仅处理 TargetSender；非 TargetSender 返回包裹 ErrNotTargetSender 的错误，调用方可据此走连接借用兜底。
// 返回 sent/failed（成功/失败投递数）与首个错误（全部失败时也返回）。
func SendToRefTarget(ctx types.RuleContext, pool types.NodePool, id, target string, data []byte) (sent, failed int, err error) {
	inst, found := ResolveResourceFromCtx(ctx, pool, id)
	if !found {
		return 0, 0, fmt.Errorf("%w: ref://%s", ErrResourceNotFound, id)
	}
	sender, ok := inst.(types.TargetSender)
	if !ok {
		return 0, 0, fmt.Errorf("%w: ref://%s type %T", ErrNotTargetSender, id, inst)
	}
	return sender.SendToTarget(target, data)
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
