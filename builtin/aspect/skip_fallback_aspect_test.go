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
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

type fallbackRuleContext struct {
	types.RuleContext
	chainId  string
	selfId   string
	mu       sync.Mutex
	failures []struct {
		msg types.RuleMsg
		err error
	}
}

func (c *fallbackRuleContext) RuleChain() types.NodeCtx {
	return &stubNodeCtx{nodeId: types.RuleNodeId{Id: c.chainId, Type: types.CHAIN}}
}
func (c *fallbackRuleContext) GetSelfId() string { return c.selfId }
func (c *fallbackRuleContext) TellFailure(msg types.RuleMsg, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = append(c.failures, struct {
		msg types.RuleMsg
		err error
	}{msg, err})
}

func newFallbackCtx(chainId, selfId string) *fallbackRuleContext {
	base := test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {})
	return &fallbackRuleContext{RuleContext: base, chainId: chainId, selfId: selfId}
}

func TestSkipFallbackAspectConfig(t *testing.T) {
	// defaults applied when zero values are provided
	def := (&SkipFallbackAspect{}).New().(*SkipFallbackAspect)
	assert.Equal(t, int64(3), def.ErrorCountLimit)
	assert.Equal(t, time.Second*10, def.LimitDuration)
	assert.Equal(t, 10, def.Order())
	assert.Equal(t, "fallback", def.Type())

	// custom values are preserved
	custom := (&SkipFallbackAspect{ErrorCountLimit: 5, LimitDuration: time.Minute}).New().(*SkipFallbackAspect)
	assert.Equal(t, int64(5), custom.ErrorCountLimit)
	assert.Equal(t, time.Minute, custom.LimitDuration)

	ctx := newFallbackCtx("chain1", "node1")
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// PointCut defaults to true and delegates to PointCutFunc when set
	assert.True(t, def.PointCut(ctx, msg, ""))
	targeted := &SkipFallbackAspect{PointCutFunc: func(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
		return relationType == types.True
	}}
	assert.False(t, targeted.PointCut(ctx, msg, types.False))
	assert.True(t, targeted.PointCut(ctx, msg, types.True))
}

func TestSkipFallbackAspectCircuitBreaker(t *testing.T) {
	aspect := (&SkipFallbackAspect{ErrorCountLimit: 2, LimitDuration: time.Second * 60}).New().(*SkipFallbackAspect)
	ctx := newFallbackCtx("chain1", "node1")
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// no error recorded yet: node executes normally
	out, proceed := aspect.Around(ctx, msg, types.Success)
	assert.Equal(t, msg, out)
	assert.True(t, proceed)

	// non-failure relation types are not recorded
	aspect.After(ctx, msg, nil, types.Success)
	_, ok := aspect.getChainError("chain1")
	assert.False(t, ok)

	// first failure records errorCount=1, below the threshold
	aspect.After(ctx, msg, nil, types.Failure)
	nodeErr := mustNodeError(t, aspect, "chain1", "node1")
	assert.Equal(t, int64(1), nodeErr.errorCount)
	out, proceed = aspect.Around(ctx, msg, types.Success)
	assert.Equal(t, msg, out)
	assert.True(t, proceed)

	// second failure reaches the threshold: execution is skipped
	aspect.After(ctx, msg, nil, types.Failure)
	nodeErr = mustNodeError(t, aspect, "chain1", "node1")
	assert.Equal(t, int64(2), nodeErr.errorCount)
	out, proceed = aspect.Around(ctx, msg, types.Success)
	assert.Equal(t, msg, out)
	assert.False(t, proceed)
	assert.Equal(t, 1, len(ctx.failures))
	assert.Equal(t, FallbackErr, ctx.failures[0].err)

	// another node on the same chain is unaffected
	otherCtx := newFallbackCtx("chain1", "node2")
	_, proceed = aspect.Around(otherCtx, msg, types.Success)
	assert.True(t, proceed)

	// another chain is unaffected
	chain2Ctx := newFallbackCtx("chain2", "node1")
	_, proceed = aspect.Around(chain2Ctx, msg, types.Success)
	assert.True(t, proceed)
}

func TestSkipFallbackAspectRecovery(t *testing.T) {
	aspect := (&SkipFallbackAspect{ErrorCountLimit: 2, LimitDuration: time.Second * 60}).New().(*SkipFallbackAspect)
	ctx := newFallbackCtx("chain1", "node1")
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// expired error record is cleared and the node executes again
	chainErr := &chainNodeErrorCache{}
	chainErr.nodeErrorCache.Store("node1", &NodeError{
		errorCount:    5,
		lastErrorTime: time.Now().UnixMilli() - aspect.LimitDuration.Milliseconds() - 1000,
	})
	aspect.chainNodeErrorCache.Store("chain1", chainErr)

	out, proceed := aspect.Around(ctx, msg, types.Success)
	assert.Equal(t, msg, out)
	assert.True(t, proceed)
	_, ok := aspect.getNodeError(chainErr, "node1")
	assert.False(t, ok)

	// short LimitDuration: record expires by sleeping
	fast := (&SkipFallbackAspect{ErrorCountLimit: 1, LimitDuration: time.Millisecond * 50}).New().(*SkipFallbackAspect)
	fast.After(ctx, msg, nil, types.Failure)
	_, proceed = fast.Around(ctx, msg, types.Success)
	assert.False(t, proceed)
	time.Sleep(time.Millisecond * 100)
	_, proceed = fast.Around(ctx, msg, types.Success)
	assert.True(t, proceed)
}

func TestSkipFallbackAspectReloadAndDestroy(t *testing.T) {
	aspect := (&SkipFallbackAspect{ErrorCountLimit: 1, LimitDuration: time.Second * 60}).New().(*SkipFallbackAspect)
	ctx := newFallbackCtx("chain1", "node1")
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// node reload clears only the node record
	aspect.After(ctx, msg, nil, types.Failure)
	err := aspect.OnReload(
		&stubNodeCtx{nodeId: types.RuleNodeId{Id: "chain1", Type: types.CHAIN}},
		&stubNodeCtx{nodeId: types.RuleNodeId{Id: "node1", Type: types.NODE}})
	assert.Nil(t, err)
	_, ok := aspect.getChainError("chain1")
	assert.True(t, ok) // chain cache entry itself survives

	// node reload against an unknown chain is a no-op
	err = aspect.OnReload(
		&stubNodeCtx{nodeId: types.RuleNodeId{Id: "unknown", Type: types.CHAIN}},
		&stubNodeCtx{nodeId: types.RuleNodeId{Id: "node1", Type: types.NODE}})
	assert.Nil(t, err)

	// chain reload clears the whole chain record
	aspect.After(ctx, msg, nil, types.Failure)
	err = aspect.OnReload(nil, &stubNodeCtx{nodeId: types.RuleNodeId{Id: "chain1", Type: types.CHAIN}})
	assert.Nil(t, err)
	_, ok = aspect.getChainError("chain1")
	assert.False(t, ok)

	// destroy removes the chain record; node-type destroy is a no-op
	aspect.After(ctx, msg, nil, types.Failure)
	aspect.OnDestroy(&stubNodeCtx{nodeId: types.RuleNodeId{Id: "node1", Type: types.NODE}})
	_, ok = aspect.getChainError("chain1")
	assert.True(t, ok)
	aspect.OnDestroy(&stubNodeCtx{nodeId: types.RuleNodeId{Id: "chain1", Type: types.CHAIN}})
	_, ok = aspect.getChainError("chain1")
	assert.False(t, ok)
}

func TestSkipFallbackAspectConcurrentFailures(t *testing.T) {
	aspect := (&SkipFallbackAspect{ErrorCountLimit: 1000, LimitDuration: time.Second * 60}).New().(*SkipFallbackAspect)
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := newFallbackCtx("chain1", "node1")
			aspect.After(ctx, msg, nil, types.Failure)
		}()
	}
	wg.Wait()
	nodeErr := mustNodeError(t, aspect, "chain1", "node1")
	assert.Equal(t, int64(20), nodeErr.errorCount)
}

func mustChainError(t *testing.T, aspect *SkipFallbackAspect, chainId string) *chainNodeErrorCache {
	t.Helper()
	chainErr, ok := aspect.getChainError(chainId)
	if !ok {
		t.Fatalf("chain error cache for %s not found", chainId)
	}
	return chainErr
}

func mustNodeError(t *testing.T, aspect *SkipFallbackAspect, chainId, nodeId string) *NodeError {
	t.Helper()
	chainErr := mustChainError(t, aspect, chainId)
	nodeErr, ok := aspect.getNodeError(chainErr, nodeId)
	if !ok {
		t.Fatalf("node error cache for %s not found", nodeId)
	}
	return nodeErr
}
