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

// resourceRegistry is the default implementation within types.ResourceRegistry's engine.
// Use sync.Map: Lookup path is completely lock-free, suitable for high-frequency ref:// parsing of underlying libraries;
// Write path (Register/Unregister) Low frequency (component deployment/overload).
//
// Zero value available: RuleChainCtx can be held directly as a value field, without the need for initialization.
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
