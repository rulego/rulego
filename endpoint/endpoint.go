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

// Package endpoint /**

package endpoint

import (
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
)

type Endpoint = endpoint.Endpoint

// Deprecated: Use Flow github.com/rulego/rulego/api/types/endpoint.Exchange instead.
type Exchange = endpoint.Exchange

// NewRouter 创建新的路由
func NewRouter(opts ...endpoint.RouterOption) endpoint.Router {
	return impl.NewRouter(opts...)
}
