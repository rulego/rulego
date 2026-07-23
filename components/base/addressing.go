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

// addressing.go provides a universal parsing tool for ref:// addressing push, which can be reused by official and third-party addressable nodes (such as net/ws).
//
// Placed in components/base rather than components/external: addressing resolution is node-side infrastructure, and third-party extended addressing is the type
// nodes are already required to import the base (the node embeds the base.SharedNode), which can avoid pulling the entire tool for reuse
// components/external and all its component dependencies. The addressing capability interfaces TargetSender / ResourceLookup are defined in api/types.

// ResolveRefEndpoint parsing ref:// Target example: same-chain ChainCtx.Resources() prioritizes → NodePool rollback.
// Read paths all follow sync.Map (same-chain resourceRegistry + NodePool entries), lock-free, zero allocation.
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

// TargetResolver precompiles ${} expression templates and provides the ability to parse and address targets by message content.
// Addressable nodes are shared, avoiding the repeated compilation of templates for every message.
type TargetResolver struct {
	template el.Template // Init compiled, nil represents a pure literal quantity or empty
	literal  string      // Original configuration values (empty strings / literals / *, etc.)
}

// NewTargetResolver creates a parser. cfg is the target field in the configuration (supports ${} expressions or literals).
// If compilation fails, it is downgraded to literal (no error, consistent with the original behavior).
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

// Resolve: parse the address target from the message and reuse ctx.GetEnv standard environment (msg/metadata/vars/global).
// Returning an empty string means the expression parsing result is empty (the caller must decide whether to allow the null target to be broadcast).
func (r *TargetResolver) Resolve(ctx types.RuleContext, msg types.RuleMsg) string {
	if r.template == nil {
		return r.literal
	}
	return r.template.ExecuteAsString(NodeUtils.GetEvnAndMetadata(ctx, msg))
}

// Literal returns the original configuration value (used for scenarios such as error messages).
func (r *TargetResolver) Literal() string {
	return r.literal
}

// Is IsEmpty configured to be empty?
func (r *TargetResolver) IsEmpty() bool {
	return r.literal == ""
}
