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

package types

import (
	"testing"

	"github.com/rulego/rulego/test/assert"
)

// testAspect 按 order 区分实例，并实现全部增强点接口，用于验证切面分类。
type testAspect struct {
	order int
}

func (a *testAspect) Order() int  { return a.order }
func (a *testAspect) New() Aspect { return a }
func (a *testAspect) PointCut(ctx RuleContext, msg RuleMsg, relationType string) bool {
	return true
}
func (a *testAspect) Around(ctx RuleContext, msg RuleMsg, relationType string) (RuleMsg, bool) {
	return msg, true
}
func (a *testAspect) Before(ctx RuleContext, msg RuleMsg, relationType string) RuleMsg {
	return msg
}
func (a *testAspect) After(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg {
	return msg
}
func (a *testAspect) Start(ctx RuleContext, msg RuleMsg) (RuleMsg, error) {
	return msg, nil
}
func (a *testAspect) End(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg {
	return msg
}
func (a *testAspect) Completed(ctx RuleContext, msg RuleMsg) RuleMsg { return msg }
func (a *testAspect) OnChainBeforeInit(config Config, def *RuleChain) error {
	return nil
}
func (a *testAspect) OnNodeBeforeInit(config Config, def *RuleNode) error { return nil }
func (a *testAspect) OnCreated(chainCtx NodeCtx) error                    { return nil }
func (a *testAspect) OnReload(chainCtx NodeCtx, ctx NodeCtx) error        { return nil }
func (a *testAspect) OnDestroy(chainCtx NodeCtx)                          {}

// TestAspectListGetNodeAspects 节点类切面按 Order 升序归类
func TestAspectListGetNodeAspects(t *testing.T) {
	late := &testAspect{order: 200}
	early := &testAspect{order: 100}
	around, before, after := AspectList{late, early}.GetNodeAspects()
	assert.Equal(t, 2, len(around))
	assert.Equal(t, early, around[0])
	assert.Equal(t, late, around[1])
	assert.Equal(t, 2, len(before))
	assert.Equal(t, early, before[0])
	assert.Equal(t, 2, len(after))
	assert.Equal(t, early, after[0])
}

// TestAspectListGetChainAspects 链路类切面按 Order 升序归类
func TestAspectListGetChainAspects(t *testing.T) {
	late := &testAspect{order: 200}
	early := &testAspect{order: 100}
	start, end, completed := AspectList{late, early}.GetChainAspects()
	assert.Equal(t, 2, len(start))
	assert.Equal(t, early, start[0])
	assert.Equal(t, 2, len(end))
	assert.Equal(t, early, end[0])
	assert.Equal(t, 2, len(completed))
	assert.Equal(t, early, completed[0])
}

// TestAspectListGetEngineAspects 引擎生命周期切面按 Order 升序归类；
// 仅实现基础 Aspect 的条目不出现在任何分类里。
type orderOnlyAspect struct {
	order int
}

func (a *orderOnlyAspect) Order() int  { return a.order }
func (a *orderOnlyAspect) New() Aspect { return a }

func TestAspectListGetEngineAspects(t *testing.T) {
	late := &testAspect{order: 200}
	early := &testAspect{order: 100}
	plain := &orderOnlyAspect{order: 50}
	chainBefore, nodeBefore, created, reloaded, destroyed :=
		AspectList{late, plain, early}.GetEngineAspects()
	assert.Equal(t, 2, len(chainBefore))
	assert.Equal(t, early, chainBefore[0])
	assert.Equal(t, 2, len(nodeBefore))
	assert.Equal(t, early, nodeBefore[0])
	assert.Equal(t, 2, len(created))
	assert.Equal(t, early, created[0])
	assert.Equal(t, 2, len(reloaded))
	assert.Equal(t, early, reloaded[0])
	assert.Equal(t, 2, len(destroyed))
	assert.Equal(t, early, destroyed[0])
}
