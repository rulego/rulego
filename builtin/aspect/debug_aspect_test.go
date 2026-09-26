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

package aspect

import (
	"errors"
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// stubNodeCtx only implements GetNodeId; other NodeCtx methods are never
// called by the aspects under test.
type stubNodeCtx struct {
	types.NodeCtx
	nodeId types.RuleNodeId
}

func (n *stubNodeCtx) GetNodeId() types.RuleNodeId { return n.nodeId }

type debugRecord struct {
	chainId, flowType, nodeId, relationType string
	err                                     error
}

type debugRuleContext struct {
	types.RuleContext
	chain   types.NodeCtx
	selfId  string
	mu      sync.Mutex
	records []debugRecord
}

func (c *debugRuleContext) RuleChain() types.NodeCtx { return c.chain }
func (c *debugRuleContext) Self() types.NodeCtx {
	return &stubNodeCtx{nodeId: types.RuleNodeId{Id: c.selfId, Type: types.NODE}}
}
func (c *debugRuleContext) OnDebug(chainId, flowType, nodeId string, msg types.RuleMsg, relationType string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, debugRecord{chainId: chainId, flowType: flowType, nodeId: nodeId, relationType: relationType, err: err})
}

func TestDebugAspect(t *testing.T) {
	debug := &Debug{}
	assert.Equal(t, 900, debug.Order())
	assert.Equal(t, "debug", debug.Type())
	assert.NotNil(t, debug.New())
	assert.True(t, debug.PointCut(nil, types.RuleMsg{}, ""))

	base := test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {})
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// with a rule chain context: chainId resolved from RuleChain()
	ctx := &debugRuleContext{
		RuleContext: base,
		chain:       &stubNodeCtx{nodeId: types.RuleNodeId{Id: "chain1", Type: types.CHAIN}},
		selfId:      "node1",
	}
	out := debug.Before(ctx, msg, types.Success)
	assert.Equal(t, msg, out)
	out = debug.After(ctx, msg, nil, types.Success)
	assert.Equal(t, msg, out)
	assert.Equal(t, 2, len(ctx.records))

	first := ctx.records[0]
	assert.Equal(t, "chain1", first.chainId)
	assert.Equal(t, types.In, first.flowType)
	assert.Equal(t, "node1", first.nodeId)
	assert.Equal(t, types.Success, first.relationType)
	assert.Nil(t, first.err)

	second := ctx.records[1]
	assert.Equal(t, types.Out, second.flowType)

	// error propagation through After
	err := errors.New("node error")
	out = debug.After(ctx, msg, err, types.Failure)
	assert.Equal(t, msg, out)
	last := ctx.records[len(ctx.records)-1]
	assert.Equal(t, types.Failure, last.relationType)
	assert.Equal(t, err, last.err)

	// without a rule chain context: chainId is empty
	noChain := &debugRuleContext{RuleContext: base, selfId: "node2"}
	out = debug.Before(noChain, msg, "")
	assert.Equal(t, msg, out)
	assert.Equal(t, 1, len(noChain.records))
	assert.Equal(t, "", noChain.records[0].chainId)
	assert.Equal(t, "node2", noChain.records[0].nodeId)
}
