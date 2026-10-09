package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// joinBranchRecorder 记录节点完成时刻与 join 输出，用于断言放行时机与合并内容
type joinBranchRecorder struct {
	start time.Time
	mu    sync.Mutex
	done  map[string]time.Duration
	outs  map[string]string
}

func newJoinBranchRecorder() *joinBranchRecorder {
	return &joinBranchRecorder{
		start: time.Now(),
		done:  make(map[string]time.Duration),
		outs:  make(map[string]string),
	}
}

func (r *joinBranchRecorder) onComplete(ctx types.RuleContext, log types.RuleNodeRunLog) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done[log.Id] = time.Since(r.start)
	if data := log.OutMsg.GetData(); data != "" {
		r.outs[log.Id] = data
	}
}

func (r *joinBranchRecorder) elapsed(nodeId string) (time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.done[nodeId]
	return d, ok
}

func (r *joinBranchRecorder) output(nodeId string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outs[nodeId]
}

func runJoinBranchChain(t *testing.T, chainId string, dsl []byte, wait time.Duration) *joinBranchRecorder {
	t.Helper()
	rec := newJoinBranchRecorder()
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New(chainId, dsl, WithConfig(config))
	assert.Nil(t, err)
	ruleEngine.OnMsg(types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{"temperature":60}`),
		types.WithOnNodeCompleted(rec.onComplete))
	time.Sleep(wait)
	return rec
}

func parseMergeMap(t *testing.T, data string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	assert.Nil(t, json.Unmarshal([]byte(data), &m))
	return m
}

// TestJoinIgnoresBypassBranch join 只等连入自己的分支，不连入的旁路分支不阻塞放行
//
//	fork → A → J → tail
//	fork → DLY(1.2s) → B（旁路，不连入 J）
func TestJoinIgnoresBypassBranch(t *testing.T) {
	dsl := `{
	  "ruleChain": {"id": "join_bypass_test", "name": "join_bypass"},
	  "metadata": {
	    "nodes": [
	      {"id":"fork","type":"fork","name":"并行分支"},
	      {"id":"A","type":"jsTransform","name":"A","configuration":{"jsScript":"msg='A'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}},
	      {"id":"DLY","type":"delay","name":"DLY","configuration":{"delayMs":"1200"}},
	      {"id":"B","type":"jsTransform","name":"B","configuration":{"jsScript":"msg='B'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}},
	      {"id":"J","type":"join","name":"合并","configuration":{"mergeToMap":true,"timeout":5}},
	      {"id":"tail","type":"jsTransform","name":"tail","configuration":{"jsScript":"msg='tail'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}
	    ],
	    "connections": [
	      {"fromId":"fork","toId":"A","type":"Success"},
	      {"fromId":"fork","toId":"DLY","type":"Success"},
	      {"fromId":"A","toId":"J","type":"Success"},
	      {"fromId":"DLY","toId":"B","type":"Success"},
	      {"fromId":"J","toId":"tail","type":"Success"}
	    ]
	  }
	}`
	rec := runJoinBranchChain(t, "join_bypass_test", []byte(dsl), 3*time.Second)

	jAt, ok := rec.elapsed("J")
	assert.True(t, ok, "join 应完成")
	assert.True(t, jAt < time.Second, "join 应在旁路分支(1.2s)完成前放行，实际 %v", jAt)
	merged := parseMergeMap(t, rec.output("J"))
	_, hasA := merged["A"]
	assert.True(t, hasA)
	_, hasB := merged["B"]
	assert.False(t, hasB, "旁路分支不连入 join，不应出现在合并结果中")
	_, ok = rec.elapsed("tail")
	assert.True(t, ok, "join 下游应执行")
}

// TestJoinMultiLevelWaitsAllIncoming 多级合并：二级 join 必须等齐全部前驱，
// 包括位于一级 join 续流上的前驱（旁路支先排空也不得提前放行）
//
//	fork → L ──────────────→ J1 → P(800ms) → H ──→ J2 → out
//	fork → W → NF ─────────→ J1
//	fork → W → N1 → W2 → C2 ─────────────────────→ J2
func TestJoinMultiLevelWaitsAllIncoming(t *testing.T) {
	js := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"jsTransform","name":%q,"configuration":{"jsScript":"msg='%s'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`, id, id, id)
	}
	nodes := []string{
		`{"id":"fork","type":"fork","name":"并行分支"}`,
		js("L"), js("W"), js("NF"), js("N1"), js("W2"), js("C2"),
		`{"id":"J1","type":"join","name":"合并1","configuration":{"mergeToMap":false,"timeout":5}}`,
		`{"id":"P","type":"delay","name":"慢支","configuration":{"delayMs":"800"}}`,
		js("H"),
		`{"id":"J2","type":"join","name":"合并2","configuration":{"mergeToMap":true,"timeout":5}}`,
		js("out"),
	}
	conns := [][2]string{
		{"fork", "L"}, {"fork", "W"},
		{"W", "NF"}, {"W", "N1"},
		{"L", "J1"}, {"NF", "J1"},
		{"J1", "P"}, {"P", "H"}, {"H", "J2"},
		{"N1", "W2"}, {"W2", "C2"}, {"C2", "J2"},
		{"J2", "out"},
	}
	connList := make([]string, 0, len(conns))
	for _, c := range conns {
		connList = append(connList, fmt.Sprintf(`{"fromId":%q,"toId":%q,"type":"Success"}`, c[0], c[1]))
	}
	dsl := fmt.Sprintf(`{
	  "ruleChain": {"id": "join_multi_level_test", "name": "join_multi_level"},
	  "metadata": {
	    "nodes": [%s],
	    "connections": [%s]
	  }
	}`, strings.Join(nodes, ","), strings.Join(connList, ","))

	rec := runJoinBranchChain(t, "join_multi_level_test", []byte(dsl), 4*time.Second)

	j1At, ok := rec.elapsed("J1")
	assert.True(t, ok, "一级 join 应完成")
	assert.True(t, j1At < time.Second, "一级 join 前驱(L、NF)送达即放行，不受旁路支影响，实际 %v", j1At)

	hAt, ok := rec.elapsed("H")
	assert.True(t, ok, "一级 join 下游应执行")
	j2At, ok := rec.elapsed("J2")
	assert.True(t, ok, "二级 join 应完成")
	assert.True(t, j2At >= hAt, "二级 join 必须等齐 H 与 C2，不得提前放行")

	merged := parseMergeMap(t, rec.output("J2"))
	_, hasC2 := merged["C2"]
	assert.True(t, hasC2, "旁路支数据应进入二级合并")
	_, hasH := merged["H"]
	assert.True(t, hasH, "一级 join 续流上的前驱数据不得丢失")
	_, ok = rec.elapsed("out")
	assert.True(t, ok, "二级 join 下游应执行")
}

// TestJoinConditionalBranchFallback 条件分支只有一路执行时，join 靠 LCA 排空兜底放行
//
//	swich(temperature>50) -True-> T1 → J → out
//	                       -False-> T2 → J
func TestJoinConditionalBranchFallback(t *testing.T) {
	dsl := `{
	  "ruleChain": {"id": "join_conditional_test", "name": "join_conditional"},
	  "metadata": {
	    "nodes": [
	      {"id":"swich","type":"jsFilter","name":"过滤","configuration":{"jsScript":"return msg.temperature > 50;"}},
	      {"id":"T1","type":"jsTransform","name":"T1","configuration":{"jsScript":"msg='T1'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}},
	      {"id":"T2","type":"jsTransform","name":"T2","configuration":{"jsScript":"msg='T2'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}},
	      {"id":"J","type":"join","name":"合并","configuration":{"mergeToMap":true,"timeout":5}},
	      {"id":"out","type":"jsTransform","name":"out","configuration":{"jsScript":"msg='out'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}
	    ],
	    "connections": [
	      {"fromId":"swich","toId":"T1","type":"True"},
	      {"fromId":"swich","toId":"T2","type":"False"},
	      {"fromId":"T1","toId":"J","type":"Success"},
	      {"fromId":"T2","toId":"J","type":"Success"},
	      {"fromId":"J","toId":"out","type":"Success"}
	    ]
	  }
	}`
	for _, temp := range []int{60, 30} {
		rec := newJoinBranchRecorder()
		config := NewConfig(types.WithDefaultPool())
		ruleEngine, err := New("join_conditional_test", []byte(dsl), WithConfig(config))
		assert.Nil(t, err)
		ruleEngine.OnMsg(types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil,
			fmt.Sprintf(`{"temperature":%d}`, temp)), types.WithOnNodeCompleted(rec.onComplete))
		time.Sleep(time.Millisecond * 500)

		merged := parseMergeMap(t, rec.output("J"))
		assert.True(t, len(merged) == 1, "temperature=%d 只应有一路进入合并", temp)
		expect := "T1"
		if temp <= 50 {
			expect = "T2"
		}
		_, hasExpect := merged[expect]
		assert.True(t, hasExpect)
		_, ok := rec.elapsed("out")
		assert.True(t, ok, "temperature=%d join 下游应执行", temp)
	}
}

// TestJoinComplexTopologyWithFlowNodes 复刻用户"生产流程"拓扑：业务节点全部为
// flow 子链调用，两级 join + 旁路支，验证全链执行且二级合并数据齐全
func TestJoinComplexTopologyWithFlowNodes(t *testing.T) {
	sub := `{
	  "ruleChain": {"id": "join_flow_sub", "name": "测试子链", "root": false},
	  "metadata": {
	    "nodes": [
	      {"id":"node_3","type":"jsTransform","name":"js转换","configuration":{"jsScript":"msg = '已运行' + metadata.fromNodeId;\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}
	    ],
	    "connections": []
	  }
	}`
	config := NewConfig(types.WithDefaultPool())
	_, err := New("join_flow_sub", []byte(sub), WithConfig(config))
	assert.Nil(t, err)

	fnode := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"flow","name":%q,"configuration":{"extend":false,"targetId":"join_flow_sub"}}`, id, id)
	}
	nodes := []string{
		`{"id":"fork","type":"fork","name":"并行分支"}`,
		fnode("laser"), fnode("flange"), fnode("ndt_flange"), fnode("ndt_2flange_1"),
		`{"id":"J1","type":"join","name":"合并","configuration":{"mergeToMap":false,"timeout":5}}`,
		fnode("pipe_welding"),
		`{"id":"slow","type":"delay","name":"慢支","configuration":{"delayMs":"800"}}`,
		fnode("2flange_welding"), fnode("ndt_2flange_2"), fnode("cnc_2flange"),
		fnode("ndt_pipe"), fnode("cnc_pipe"), fnode("holder"),
		`{"id":"J2","type":"join","name":"合并","configuration":{"mergeToMap":true,"timeout":5}}`,
		fnode("assembly"),
	}
	conns := [][2]string{
		{"fork", "laser"}, {"fork", "flange"},
		{"flange", "ndt_flange"}, {"flange", "ndt_2flange_1"},
		{"laser", "J1"}, {"ndt_flange", "J1"},
		{"J1", "pipe_welding"}, {"pipe_welding", "slow"}, {"slow", "ndt_pipe"},
		{"ndt_pipe", "cnc_pipe"}, {"cnc_pipe", "holder"}, {"holder", "J2"},
		{"ndt_2flange_1", "2flange_welding"}, {"2flange_welding", "ndt_2flange_2"},
		{"ndt_2flange_2", "cnc_2flange"}, {"cnc_2flange", "J2"},
		{"J2", "assembly"},
	}
	connList := make([]string, 0, len(conns))
	for _, c := range conns {
		connList = append(connList, fmt.Sprintf(`{"fromId":%q,"toId":%q,"type":"Success"}`, c[0], c[1]))
	}
	dsl := fmt.Sprintf(`{
	  "ruleChain": {"id": "join_flow_topology_test", "name": "生产流程-test", "debugMode": true},
	  "metadata": {
	    "nodes": [%s],
	    "connections": [%s]
	  }
	}`, strings.Join(nodes, ","), strings.Join(connList, ","))

	rec := runJoinBranchChain(t, "join_flow_topology_test", []byte(dsl), 4*time.Second)

	for _, id := range []string{"laser", "flange", "ndt_flange", "ndt_2flange_1",
		"2flange_welding", "ndt_2flange_2", "cnc_2flange",
		"pipe_welding", "ndt_pipe", "cnc_pipe", "holder", "assembly", "J1", "J2"} {
		_, ok := rec.elapsed(id)
		assert.True(t, ok, "节点 %s 应执行", id)
	}

	merged := parseMergeMap(t, rec.output("J2"))
	_, hasCnc := merged["cnc_2flange"]
	assert.True(t, hasCnc)
	_, hasHolder := merged["holder"]
	assert.True(t, hasHolder, "直管焊接支结果必须进入最终合并")

	holderAt, _ := rec.elapsed("holder")
	assemblyAt, _ := rec.elapsed("assembly")
	assert.True(t, assemblyAt >= holderAt, "部装区必须在两支都到齐后执行")
}

// TestJoinBypassSlowPredNotReleasedEarly 二级 join 的未送达前驱是纯旁路慢支
// （不在任何 join 下游）时，不得被提前放行。join 续流消息的到达级联若把已
// 结清的边再扣一次，fork 会被提前标记完成，二级 join 的 LCA 排空兜底就会
// 带不完整集合提前放行、慢支后到消息被丢弃：
//
//	fork → A1 → J1        fork → Bd(800ms) → B → J2
//	fork → A2 → J1        J1 → M → J2
func TestJoinBypassSlowPredNotReleasedEarly(t *testing.T) {
	js := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"jsTransform","name":%q,"configuration":{"jsScript":"msg='%s'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`, id, id, id)
	}
	nodes := []string{
		`{"id":"fork","type":"fork","name":"fork"}`,
		js("A1"), js("A2"),
		`{"id":"J1","type":"join","name":"J1","configuration":{"mergeToMap":false,"timeout":5}}`,
		js("M"),
		`{"id":"Bd","type":"delay","name":"Bd","configuration":{"delayMs":"800"}}`,
		js("B"),
		`{"id":"J2","type":"join","name":"J2","configuration":{"mergeToMap":true,"timeout":5}}`,
		js("out"),
	}
	conns := [][2]string{
		{"fork", "A1"}, {"fork", "A2"}, {"fork", "Bd"},
		{"A1", "J1"}, {"A2", "J1"},
		{"J1", "M"}, {"M", "J2"},
		{"Bd", "B"}, {"B", "J2"},
		{"J2", "out"},
	}
	connList := make([]string, 0, len(conns))
	for _, c := range conns {
		connList = append(connList, fmt.Sprintf(`{"fromId":%q,"toId":%q,"type":"Success"}`, c[0], c[1]))
	}
	dsl := fmt.Sprintf(`{"ruleChain":{"id":"join_bypass_slow_pred_test"},"metadata":{"nodes":[%s],"connections":[%s]}}`,
		strings.Join(nodes, ","), strings.Join(connList, ","))

	rec := runJoinBranchChain(t, "join_bypass_slow_pred_test", []byte(dsl), 2*time.Second)

	j2At, ok := rec.elapsed("J2")
	assert.True(t, ok, "二级 join 应完成")
	assert.True(t, j2At >= 500*time.Millisecond, "J2 必须等纯旁路慢支 B(800ms) 送达才放行，实际 %v", j2At)
	merged := parseMergeMap(t, rec.output("J2"))
	_, hasM := merged["M"]
	assert.True(t, hasM, "一级 join 续流数据应进入二级合并")
	_, hasB := merged["B"]
	assert.True(t, hasB, "旁路慢支数据不得因提前放行被丢弃")
	_, ok = rec.elapsed("out")
	assert.True(t, ok, "二级 join 下游应执行")
}

// 循环体每轮迭代经 TellNode 全新 observer 执行，体内 join 的等待与触发
// 不跨迭代残留，也不会阻塞迭代推进。TellNode 子执行不携带 runSnapshot，
// 体内节点对 WithOnNodeCompleted 不可见，断言走 for 节点 mode=1 收集的
// 各轮结果（join 合并输出经循环体终点汇入 out 节点）。

// runLoopJoinChain 执行循环+join 链，返回 out 节点（for 下游）的输出数组
func runLoopJoinChain(t *testing.T, chainId string, dsl string) ([]map[string]interface{}, time.Duration) {
	t.Helper()
	rec := newJoinBranchRecorder()
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New(chainId, []byte(dsl), WithConfig(config))
	assert.Nil(t, err)
	ruleEngine.OnMsg(types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`),
		types.WithOnNodeCompleted(rec.onComplete))
	time.Sleep(2 * time.Second)

	outAt, ok := rec.elapsed("out")
	assert.True(t, ok, "循环完成后下游 out 应执行")
	var list []interface{}
	assert.Nil(t, json.Unmarshal([]byte(rec.output("out")), &list))
	merged := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		assert.True(t, ok, "每轮结果应为对象，实际 %v", item)
		merged = append(merged, m)
	}
	return merged, outAt
}

// TestForLoopBodyJoinPerIteration 循环体内 fork/join 每轮独立合并：
//
//	for(1..3, mode=1) → bodyFork → bA → J_body → tail
//	                         └───→ bB → J_body
//	for → out
//
// 每轮 J_body 必须集齐当轮的 bA、bB（值带当轮 _loopItem），不得跨轮混数或丢支
func TestForLoopBodyJoinPerIteration(t *testing.T) {
	js := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"jsTransform","name":%q,"configuration":{"jsScript":"msg='%s'+metadata._loopItem; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`, id, id, id)
	}
	dsl := fmt.Sprintf(`{
	  "ruleChain": {"id": "join_loop_body_test", "name": "join_loop_body"},
	  "metadata": {
	    "nodes": [
	      {"id":"forNode","type":"for","name":"循环","configuration":{"range":"1..3","do":"bodyFork","mode":1}},
	      {"id":"bodyFork","type":"fork","name":"循环体分支"},
	      %s,%s,
	      {"id":"J_body","type":"join","name":"体内合并","configuration":{"mergeToMap":true,"timeout":5}},
	      {"id":"tail","type":"jsTransform","name":"tail","configuration":{"jsScript":"return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}},
	      {"id":"out","type":"jsTransform","name":"out","configuration":{"jsScript":"return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}
	    ],
	    "connections": [
	      {"fromId":"bodyFork","toId":"bA","type":"Success"},
	      {"fromId":"bodyFork","toId":"bB","type":"Success"},
	      {"fromId":"bA","toId":"J_body","type":"Success"},
	      {"fromId":"bB","toId":"J_body","type":"Success"},
	      {"fromId":"J_body","toId":"tail","type":"Success"},
	      {"fromId":"forNode","toId":"out","type":"Success"}
	    ]
	  }
	}`, js("bA"), js("bB"))

	mergedList, outAt := runLoopJoinChain(t, "join_loop_body_test", dsl)

	assert.True(t, len(mergedList) == 3, "三轮迭代应各产出一次合并，实际 %d 次", len(mergedList))
	seen := make(map[string]bool)
	for _, m := range mergedList {
		a, hasA := m["bA"]
		b, hasB := m["bB"]
		assert.True(t, hasA && hasB, "每轮合并必须集齐 bA、bB 两支，实际 %v", m)
		seen[fmt.Sprintf("%v|%v", a, b)] = true
	}
	assert.True(t, len(seen) == 3, "三轮合并值应各不相同（各带当轮 item），实际 %v", seen)
	assert.True(t, outAt < 1500*time.Millisecond, "循环不应阻塞等待 join 超时，实际 %v", outAt)
}

// TestForLoopBodyJoinConditionalFallback 循环体内条件分支只有一路执行时，
// join 靠 LCA 排空兜底当轮放行，不得等超时、不得跨轮残留：
//
//	for(1..3, mode=1) → swich ─True→ tA → J_body2 → tail2
//	                   └─False→ tB → J_body2
//	for → out
func TestForLoopBodyJoinConditionalFallback(t *testing.T) {
	js := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"jsTransform","name":%q,"configuration":{"jsScript":"msg='%s'+metadata._loopItem; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`, id, id, id)
	}
	dsl := fmt.Sprintf(`{
	  "ruleChain": {"id": "join_loop_cond_test", "name": "join_loop_cond"},
	  "metadata": {
	    "nodes": [
	      {"id":"forNode","type":"for","name":"循环","configuration":{"range":"1..3","do":"swich","mode":1}},
	      {"id":"swich","type":"jsFilter","name":"按item分流","configuration":{"jsScript":"return Number(metadata._loopItem) %% 2 == 1;"}},
	      %s,%s,
	      {"id":"J_body2","type":"join","name":"体内合并","configuration":{"mergeToMap":true,"timeout":5}},
	      {"id":"tail2","type":"jsTransform","name":"tail2","configuration":{"jsScript":"return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}},
	      {"id":"out","type":"jsTransform","name":"out","configuration":{"jsScript":"return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}
	    ],
	    "connections": [
	      {"fromId":"swich","toId":"tA","type":"True"},
	      {"fromId":"swich","toId":"tB","type":"False"},
	      {"fromId":"tA","toId":"J_body2","type":"Success"},
	      {"fromId":"tB","toId":"J_body2","type":"Success"},
	      {"fromId":"J_body2","toId":"tail2","type":"Success"},
	      {"fromId":"forNode","toId":"out","type":"Success"}
	    ]
	  }
	}`, js("tA"), js("tB"))

	mergedList, outAt := runLoopJoinChain(t, "join_loop_cond_test", dsl)

	assert.True(t, len(mergedList) == 3, "三轮迭代应各产出一次合并，实际 %d 次", len(mergedList))
	found := map[string]bool{}
	for _, m := range mergedList {
		_, hasA := m["tA"]
		_, hasB := m["tB"]
		assert.True(t, hasA != hasB, "每轮只有一路进入合并，实际 %v", m)
		if hasA {
			found[fmt.Sprintf("%v", m["tA"])] = true
		}
		if hasB {
			found[fmt.Sprintf("%v", m["tB"])] = true
		}
	}
	assert.True(t, found["tA1"] && found["tB2"] && found["tA3"],
		"item=1,3 走 True 支、item=2 走 False 支，实际 %v", found)
	assert.True(t, outAt < 1500*time.Millisecond, "条件分支兜底应当轮即时放行，不等超时，实际 %v", outAt)
}

// TestJoinHighVolumeNoPartialMerge 大批量消息下 join 不得出现部分合并。
// 分支派发与分支执行并发进行，join 的放行判定不得早于全部前驱送达，
// 否则合并结果缺支且后到消息被丢弃。
func TestJoinHighVolumeNoPartialMerge(t *testing.T) {
	js := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"jsTransform","name":%q,"configuration":{"jsScript":"msg='%s'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`, id, id, id)
	}
	nodes := []string{
		`{"id":"fork","type":"fork","name":"fork"}`,
		js("b1"), js("b2"), js("b3"), js("b4"),
		`{"id":"J","type":"join","name":"J","configuration":{"mergeToMap":true,"timeout":5}}`,
		`{"id":"tail","type":"jsTransform","name":"tail","configuration":{"jsScript":"return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`,
	}
	conns := [][2]string{
		{"fork", "b1"}, {"fork", "b2"}, {"fork", "b3"}, {"fork", "b4"},
		{"b1", "J"}, {"b2", "J"}, {"b3", "J"}, {"b4", "J"},
		{"J", "tail"},
	}
	connList := make([]string, 0, len(conns))
	for _, c := range conns {
		connList = append(connList, fmt.Sprintf(`{"fromId":%q,"toId":%q,"type":"Success"}`, c[0], c[1]))
	}
	dsl := fmt.Sprintf(`{"ruleChain":{"id":"join_high_volume_test"},"metadata":{"nodes":[%s],"connections":[%s]}}`,
		strings.Join(nodes, ","), strings.Join(connList, ","))

	config := NewConfig(types.WithDefaultPool())
	e, err := New("join_high_volume_test", []byte(dsl), WithConfig(config))
	assert.Nil(t, err)

	var badOutputs int32
	var wg sync.WaitGroup
	workers := 16
	perWorker := 3125
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				done := make(chan struct{}, 1)
				e.OnMsg(types.NewMsg(0, "T", types.JSON, nil, "{}"),
					types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
						if ctx.GetSelfId() != "tail" {
							return
						}
						data := msg.GetData()
						for _, k := range []string{"b1", "b2", "b3", "b4"} {
							if !strings.Contains(data, fmt.Sprintf("%q:%q", k, k)) {
								atomic.AddInt32(&badOutputs, 1)
								break
							}
						}
						select {
						case done <- struct{}{}:
						default:
						}
					}))
				<-done
			}
		}()
	}
	wg.Wait()
	assert.True(t, atomic.LoadInt32(&badOutputs) == 0,
		"join 在高并发下出现部分合并，badOutputs=%d", atomic.LoadInt32(&badOutputs))
}

// TestJoinHighVolumeWaitModeNoHang OnMsgAndWait 高压版。wait 模式阻塞在链完成
// 回调上，异步 OnMsg 版（上方用例）观测不到回调丢失。回归场景：join 续流的
// 完成传播与最后到达分支的边结清交错时，完成回调不会被任何路径触发，wait 永久挂死
func TestJoinHighVolumeWaitModeNoHang(t *testing.T) {
	js := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"type":"jsTransform","name":%q,"configuration":{"jsScript":"msg='%s'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`, id, id, id)
	}
	nodes := []string{
		`{"id":"fork","type":"fork","name":"fork"}`,
		js("b1"), js("b2"), js("b3"), js("b4"),
		`{"id":"J","type":"join","name":"J","configuration":{"mergeToMap":true,"timeout":5}}`,
		`{"id":"tail","type":"jsTransform","name":"tail","configuration":{"jsScript":"return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"}}`,
	}
	conns := [][2]string{
		{"fork", "b1"}, {"fork", "b2"}, {"fork", "b3"}, {"fork", "b4"},
		{"b1", "J"}, {"b2", "J"}, {"b3", "J"}, {"b4", "J"},
		{"J", "tail"},
	}
	connList := make([]string, 0, len(conns))
	for _, c := range conns {
		connList = append(connList, fmt.Sprintf(`{"fromId":%q,"toId":%q,"type":"Success"}`, c[0], c[1]))
	}
	dsl := fmt.Sprintf(`{"ruleChain":{"id":"join_wait_high_volume_test"},"metadata":{"nodes":[%s],"connections":[%s]}}`,
		strings.Join(nodes, ","), strings.Join(connList, ","))

	config := NewConfig(types.WithDefaultPool())
	e, err := New("join_wait_high_volume_test", []byte(dsl), WithConfig(config))
	assert.Nil(t, err)

	// 每条消息限时：join 兜底超时 5s，正常完成在毫秒级，10s 仍不返回即视为挂死
	const waitTimeout = 10 * time.Second
	var hangs int32
	var wg sync.WaitGroup
	workers := 8
	perWorker := 500
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				done := make(chan struct{})
				go func() {
					defer close(done)
					e.OnMsgAndWait(types.NewMsg(0, "T", types.JSON, nil, "{}"))
				}()
				select {
				case <-done:
				case <-time.After(waitTimeout):
					atomic.AddInt32(&hangs, 1)
					return
				}
			}
		}()
	}
	wg.Wait()
	assert.True(t, atomic.LoadInt32(&hangs) == 0,
		"wait 模式挂死 %d 次（链完成回调丢失）", atomic.LoadInt32(&hangs))
}
