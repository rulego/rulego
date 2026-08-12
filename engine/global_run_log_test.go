/*
 * Copyright 2025 The RuleGo Authors.
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

import (
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// globalRunLogChain is a minimal 2-node chain: jsFilter -> jsTransform.
// ruleChain.debugMode is off (to avoid the OnDebug path); node debugMode is off too.
// A message with temperature=41 passes s1's jsFilter (>10 -> True) and reaches s2.
// globalRunLogChain 是一个最小 2 节点链：jsFilter -> jsTransform。
// ruleChain.debugMode 关闭（避免走 OnDebug 路径），节点 debugMode 也关闭。
// 消息 temperature=41 通过 s1 的 jsFilter（>10 为 True）到达 s2。
var globalRunLogChain = `{
  "ruleChain": {
    "id": "r1",
    "name": "globalRunLogChain",
    "debugMode": false,
    "root": true,
    "disabled": false
  },
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "filter",
        "debugMode": false,
        "configuration": {
          "jsScript": "return msg.temperature>10;"
        }
      },
      {
        "id": "s2",
        "type": "jsTransform",
        "name": "transform",
        "debugMode": false,
        "configuration": {
          "jsScript": "msgType='TEST_MSG_TYPE'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "True"
      }
    ]
  }
}`

// chainJSONWithLevel returns globalRunLogChain but with the rule chain's
// additionalInfo.runLogMode set to the given level. Used to test chain-level
// override in both directions (up to detail, down to summary).
// chainJSONWithLevel 返回 globalRunLogChain，但把规则链的
// additionalInfo.runLogMode 设为指定值。用于双向测试链级覆盖（升 detail / 降 summary）。
func chainJSONWithLevel(chainId, level string) string {
	return `{
  "ruleChain": {
    "id": "` + chainId + `",
    "name": "globalRunLogLevelChain",
    "debugMode": false,
    "root": true,
    "disabled": false,
    "additionalInfo": {
      "runLogMode": "` + level + `"
    }
  },
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "filter",
        "debugMode": false,
        "configuration": {
          "jsScript": "return msg.temperature>10;"
        }
      },
      {
        "id": "s2",
        "type": "jsTransform",
        "name": "transform",
        "debugMode": false,
        "configuration": {
          "jsScript": "msgType='TEST_MSG_TYPE'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "True"
      }
    ]
  }
}`
}

// newGlobalRunLogMsg builds a temperature=41 test message (passes jsFilter's >10).
// newGlobalRunLogMsg 构造一条 temperature=41 的测试消息（会通过 jsFilter 的 >10 条件）。
func newGlobalRunLogMsg() types.RuleMsg {
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	return types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, `{"temperature":41,"humidity":90}`)
}

// completedRecorder is a tiny helper that captures the latest snapshot under a mutex.
// completedRecorder 是一个在互斥锁下记录最近一次 snapshot 的小工具。
type completedRecorder struct {
	mu       sync.Mutex
	called   bool
	latest   types.RuleChainRunSnapshot
}

func (r *completedRecorder) onCompleted(_ types.RuleContext, snapshot types.RuleChainRunSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	r.latest = snapshot
}

// captured returns copies of the captured state. captured 返回已捕获状态的副本。
func (r *completedRecorder) captured() (bool, types.RuleChainRunSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.called, r.latest
}

// TestGlobalRunLog_Detail: Config.OnRuleChainCompleted + RunLogMode="detail".
// The callback fires and snapshot.Logs is non-empty (per-node logs collected).
// TestGlobalRunLog_Detail：设置 Config.OnRuleChainCompleted + RunLogMode="detail"。
// 回调被调用，且 snapshot.Logs 非空（收集了逐节点日志）。
func TestGlobalRunLog_Detail(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	// Unique id + defer Del so DefaultPool does not reuse a stale instance
	// (one created with an old, callback-less Config) across -count=N runs.
	// 使用唯一 id + defer Del，避免 DefaultPool 在 -count=N 重跑时命中旧实例（旧 Config 无回调）
	chainId := "testGlobalRunLogDetail"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "Config.OnRuleChainCompleted callback should be invoked")
	assert.True(t, len(snap.Logs) > 0, "snapshot.Logs should contain per-node logs at detail level")
	// Both nodes s1/s2 should be collected. 两个节点 s1/s2 都应被收集。
	assert.Equal(t, 2, len(snap.Logs))
}

// TestGlobalRunLog_Summary: Config.OnRuleChainCompleted + RunLogMode="summary".
// The callback fires but snapshot.Logs is empty (per-node collection skipped).
// TestGlobalRunLog_Summary：设置 Config.OnRuleChainCompleted + RunLogMode="summary"。
// 回调被调用，但 snapshot.Logs 为空（跳过了逐节点收集）。
func TestGlobalRunLog_Summary(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeSummary),
	)

	chainId := "testGlobalRunLogSummary"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "Config.OnRuleChainCompleted callback should be invoked even at summary level")
	// At summary level collectDetail=false, per-node logs are not collected;
	// snapshot.Logs is nil or an empty slice.
	// summary 级别 collectDetail=false，逐节点日志不收集；snapshot.Logs 为 nil 或空切片
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty at summary level (per-node logs skipped)")
}

// TestGlobalRunLog_Off: Config.OnRuleChainCompleted + RunLogMode="off".
// The callback still fires (whether a callback fires is orthogonal to RunLogMode),
// but snapshot.Logs is empty because off != detail.
// TestGlobalRunLog_Off：设置 Config.OnRuleChainCompleted + RunLogMode="off"。
// 回调仍会触发（回调是否触发与 RunLogMode 正交），但因 off != detail，snapshot.Logs 为空。
func TestGlobalRunLog_Off(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeOff),
	)

	chainId := "testGlobalRunLogOff"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "callback should still fire at off level (triggering is orthogonal to RunLogMode)")
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty at off level")
}

// TestGlobalRunLog_NoCallback: no completion callback registered at all.
// Verifies the engine runs without error and nothing fires. Constructs the
// config WITHOUT WithOnRuleChainCompletedGlobal (rather than registering then
// clearing), so the test would actually catch a "fires even when never set" bug.
// TestGlobalRunLog_NoCallback：完全不注册任何完成回调。
// 验证引擎正常运行不报错、且无回调触发。构造 Config 时不注册
// WithOnRuleChainCompletedGlobal（而不是注册后再清空），从而能真正捕获"未设置却触发"的 bug。
func TestGlobalRunLog_NoCallback(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithRunLogMode(types.RunLogModeDetail),
		// Deliberately no WithOnRuleChainCompletedGlobal here.
		// 故意不注册 WithOnRuleChainCompletedGlobal。
	)

	chainId := "testGlobalRunLogNoCallback"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, _ := rec.captured()
	assert.False(t, called, "Config.OnRuleChainCompleted callback should NOT be invoked when not set")
}

// TestGlobalRunLog_PerCallPriority: both per-call WithOnRuleChainCompleted and
// Config.OnRuleChainCompleted are set. Only the per-call one fires; the
// Config-level one is suppressed.
// TestGlobalRunLog_PerCallPriority：同时设置 per-call WithOnRuleChainCompleted 和
// Config.OnRuleChainCompleted。只有 per-call 触发，Config 级被抑制不触发。
func TestGlobalRunLog_PerCallPriority(t *testing.T) {
	var (
		mu            sync.Mutex
		globalCalled  bool
		perCallCalled bool
		perCallSnap   types.RuleChainRunSnapshot
	)
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(func(_ types.RuleContext, _ types.RuleChainRunSnapshot) {
			mu.Lock()
			defer mu.Unlock()
			globalCalled = true
		}),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	chainId := "testGlobalRunLogPerCall"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg(), types.WithOnRuleChainCompleted(func(_ types.RuleContext, snapshot types.RuleChainRunSnapshot) {
		mu.Lock()
		defer mu.Unlock()
		perCallCalled = true
		perCallSnap = snapshot
	}))

	mu.Lock()
	globalCalledCopy := globalCalled
	perCallCalledCopy := perCallCalled
	snap := perCallSnap
	mu.Unlock()

	assert.True(t, perCallCalledCopy, "per-call OnRuleChainCompleted should be invoked")
	assert.False(t, globalCalledCopy, "Config-level OnRuleChainCompleted should NOT be invoked when per-call is set")
	// detail level + per-call callback present -> collectDetail=true, per-node logs collected.
	// detail 级别 + per-call 回调存在 -> collectDetail=true，逐节点日志被收集
	assert.True(t, len(snap.Logs) > 0, "snapshot.Logs should contain per-node logs at detail level with per-call callback")
	assert.Equal(t, 2, len(snap.Logs))
}

// TestGlobalRunLog_ChainLevelDowngrade: global RunLogMode="detail" but the chain's
// additionalInfo.runLogMode="summary". snapshot.Logs should be empty (chain-level
// overrides global, downgrading to summary). The global callback still fires.
// TestGlobalRunLog_ChainLevelDowngrade：全局 RunLogMode="detail"，但链定义的
// additionalInfo.runLogMode="summary"。snapshot.Logs 应为空（链级覆盖全局降级为 summary）。
// 全局回调仍会触发。
func TestGlobalRunLog_ChainLevelDowngrade(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	chainId := "testGlobalRunLogChainDowngrade"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(chainJSONWithLevel(chainId, "summary")), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	// Chain-level summary overrides global detail -> collectDetail=false; but the
	// global callback still exists and still fires (the else-if branch).
	// 链级 summary 覆盖全局 detail -> collectDetail=false；但全局回调仍存在，仍会触发（走 else-if 分支）
	assert.True(t, called, "Config.OnRuleChainCompleted callback should still be invoked")
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty when chain-level runLogMode=summary overrides global detail")
}

// TestGlobalRunLog_ChainLevelUpgrade: global RunLogMode="off" but the chain's
// additionalInfo.runLogMode="detail". snapshot.Logs should be non-empty (chain-level
// overrides global, upgrading to detail). Confirms chain-level precedence is symmetric.
// TestGlobalRunLog_ChainLevelUpgrade：全局 RunLogMode="off"，但链定义的
// additionalInfo.runLogMode="detail"。snapshot.Logs 应非空（链级覆盖全局升 detail）。
// 验证链级优先是对称的。
func TestGlobalRunLog_ChainLevelUpgrade(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeOff),
	)

	chainId := "testGlobalRunLogChainUpgrade"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(chainJSONWithLevel(chainId, "detail")), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "Config.OnRuleChainCompleted callback should be invoked")
	assert.True(t, len(snap.Logs) > 0, "snapshot.Logs should contain per-node logs when chain-level runLogMode=detail overrides global off")
	assert.Equal(t, 2, len(snap.Logs))
}

// TestGlobalRunLog_InvalidGlobalMode: an unrecognized global mode is normalized to off.
// The callback still fires but no per-node logs are collected.
// TestGlobalRunLog_InvalidGlobalMode：无法识别的全局 mode 被规范化为 off。
// 回调仍触发，但不收集逐节点日志。
func TestGlobalRunLog_InvalidGlobalMode(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogMode("verbose")), // typo / unknown value
	)

	chainId := "testGlobalRunLogInvalidMode"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "callback should still fire even with an invalid mode")
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty: invalid mode normalized to off, which is not detail")
	// And the Config should reflect the normalization.
	// 同时 Config 应反映规范化后的值。
	assert.Equal(t, types.RunLogModeOff, config.RunLogMode)
}

// TestGlobalRunLog_Concurrent: many messages processed concurrently through a shared
// engine each produce an isolated snapshot for their own message id. Guards against
// runSnapshot state leaking across messages (each msg gets a fresh RunSnapshot).
// TestGlobalRunLog_Concurrent：多条消息并发流经同一个引擎，每条消息各自产生隔离的
// snapshot（对应自己的 msg id）。防止 runSnapshot 状态跨消息泄漏（每条消息新建 RunSnapshot）。
func TestGlobalRunLog_Concurrent(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	chainId := "testGlobalRunLogConcurrent"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())
		}()
	}
	wg.Wait()

	called, snap := rec.captured()
	assert.True(t, called, "callback should have fired at least once")
	// Each OnMsgAndWait builds its own snapshot with exactly 2 node logs; the
	// recorder only keeps the last one, so it must still show 2 (never more, never less).
	// 每次 OnMsgAndWait 都新建自己的 snapshot，恰好 2 条节点日志；recorder 只保留最后一次，
	// 因此仍然必须是 2（不多不少）。
	assert.Equal(t, 2, len(snap.Logs), "each message's snapshot should be isolated with exactly 2 node logs")
}
