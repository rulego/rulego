/*
 * Copyright 2026 The RuleGo Authors.
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

import "errors"

var (
	// ErrRuleChainNotFound 目标规则链不存在，宿主用 errors.Is 识别该类失败
	ErrRuleChainNotFound = errors.New("rule chain not found")
	// ErrNodeNotFound 目标节点不存在，宿主用 errors.Is 识别该类失败
	ErrNodeNotFound = errors.New("node not found")
	// ErrMsgHopBudgetExceeded 单条消息跳数超过 Config.MsgMaxHops 被终止，宿主用 errors.Is 识别该类失败
	ErrMsgHopBudgetExceeded = errors.New("message hop budget exceeded")
)
