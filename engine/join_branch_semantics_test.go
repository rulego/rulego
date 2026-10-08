package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
