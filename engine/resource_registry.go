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

package engine

import "sync"

// resourceRegistry 是 types.ResourceRegistry 的引擎内默认实现。
// 用 sync.Map：读路径（Lookup）完全无锁，适合底层库高频 ref:// 解析；
// 写路径（Register/Unregister）低频（组件部署/重载）。
//
// 零值可用：RuleChainCtx 可直接以值字段持有，无需构造初始化。
type resourceRegistry struct {
	items sync.Map // id -> any
}

func (r *resourceRegistry) Lookup(id string) (any, bool) {
	v, ok := r.items.Load(id)
	return v, ok
}

func (r *resourceRegistry) Register(id string, resource any) {
	r.items.Store(id, resource)
}

func (r *resourceRegistry) Unregister(id string) {
	r.items.Delete(id)
}
