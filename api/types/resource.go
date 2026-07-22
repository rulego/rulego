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

package types

// resource.go 定义 ref:// 引用机制的通用抽象（底层库，性能敏感）：
//   - ResourceLookup / ResourceRegistry：资源目录（按 id 解析/注册实例）
//   - TargetSender：寻址发送能力接口
//
// 设计目标：
//   - 读路径无锁、零分配（实现方用 sync.Map 等）
//   - 不绑定具体组件类型：任何“容器”实现 ResourceLookup 即可成为 ref:// 解析源；
//     任何寻址型组件实现 TargetSender 即可被 net 等节点寻址推送（开闭原则）
//   - 避免 types 主包 ↔ endpoint 子包循环依赖：接口参数用 any，消费方做能力断言

// ResourceLookup 是 ref:// 引用的只读解析视图。
// 实现方须保证：Lookup 读路径无锁、零分配。
//
// 已知实现：*node_pool.NodePool（共享池）、*engine.RuleChainCtx（同链 endpoint 等）。
type ResourceLookup interface {
	// Lookup 按 id 查找资源实例。找不到返回 (nil, false)。
	Lookup(id string) (resource any, found bool)
}

// ResourceRegistry 是可写的资源目录，内嵌 ResourceLookup。
// 写（Register/Unregister）低频（组件部署/重载时）；读（Lookup）高频（ref:// 解析）。
type ResourceRegistry interface {
	ResourceLookup
	// Register 注册/覆盖一个资源实例。
	Register(id string, resource any)
	// Unregister 注销一个资源实例。
	Unregister(id string)
}

// TargetSender 是“按 target 寻址发送”的能力接口。
// 寻址型资源（如 endpoint/net）实现此接口，即可被 net 节点的 ref:// 寻址推送复用。
//
// 实现方须保证：零分配（复用入参 data）、并发安全。
type TargetSender interface {
	// SendToTarget 按 target 寻址发送 data。
	//   - target：IP / deviceId / *（广播） / 空（广播）
	// 返回 sent（成功数）、failed（失败数）、err（首个错误；全部失败时也返回）。
	SendToTarget(target string, data []byte) (sent, failed int, err error)
}
