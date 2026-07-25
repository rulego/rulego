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

package integration

import (
	"sync/atomic"
	"testing"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/engine"
)

// testConn 是可标识的测试连接：addr 标识目标，seq 标识实例唯一性（验证是否复用同一实例）。
type testConn struct {
	addr string
	seq  int
}

var testConnSeq int32

// testConnNode 是测试用连接持有型组件：本地模式按 server 建连并以节点ID注册到同链目录，
// ref:// 则借用同链源的连接。模拟 modbus/db 等真实连接型组件，但不依赖任何外部资源。
type testConnNode struct {
	base.SharedNode[*testConn]
	Server string
}

func (n *testConnNode) Type() string { return "test/conn" }
func (n *testConnNode) New() types.Node { return &testConnNode{} }
func (n *testConnNode) Init(rc types.Config, cfg types.Configuration) error {
	if v, ok := cfg["server"]; ok {
		n.Server, _ = v.(string)
	}
	err := n.SharedNode.InitWithClose(rc, n.Type(), n.Server, rc.NodeClientInitNow,
		func() (*testConn, error) {
			return &testConn{addr: n.Server, seq: int(atomic.AddInt32(&testConnSeq, 1))}, nil
		},
		func(c *testConn) error { return nil })
	// 启用同链连接池：本地模式连接按节点ID注册到链目录
	n.SharedNode.BindChain(cfg)
	return err
}
func (n *testConnNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) { ctx.TellSuccess(msg) }
func (n *testConnNode) Destroy()                                        { _ = n.SharedNode.Close() }

// conn 暴露当前连接供测试断言（复用 / 独立）。
func (n *testConnNode) conn() (*testConn, error) { return n.SharedNode.GetSafely() }

func init() {
	_ = rulego.Registry.Register(&testConnNode{})
}

// getTestNode 从引擎取出指定 id 的 *testConnNode 实例。
func getTestNode(t *testing.T, eng *engine.RuleEngine, id string) *testConnNode {
	t.Helper()
	nc, ok := eng.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: id})
	if !ok {
		t.Fatalf("node %s not found", id)
	}
	rnc, ok := nc.(*engine.RuleNodeCtx)
	if !ok {
		t.Fatalf("node %s ctx type %T not *engine.RuleNodeCtx", id, nc)
	}
	node, ok := rnc.Node.(*testConnNode)
	if !ok {
		t.Fatalf("node %s type %T not *testConnNode", id, rnc.Node)
	}
	return node
}

// TestChainConnectionReuse 验证同链连接池核心：链内 ref://源节点ID 的借用方复用源的同一连接实例。
func TestChainConnectionReuse(t *testing.T) {
	chain := `{
  "ruleChain": {"id": "test_chain_conn_reuse", "name": "chain connection reuse", "root": true},
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {"id":"src","type":"test/conn","configuration":{"server":"deviceA"}},
      {"id":"borrower","type":"test/conn","configuration":{"server":"ref://src"}},
      {"id":"other","type":"test/conn","configuration":{"server":"deviceB"}}
    ]
  }
}`
	eng, err := rulego.New("test_chain_conn_reuse", []byte(chain))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_chain_conn_reuse")
	ruleEng := eng.(*engine.RuleEngine)

	src := getTestNode(t, ruleEng, "src")
	borrower := getTestNode(t, ruleEng, "borrower")
	other := getTestNode(t, ruleEng, "other")

	// 触发源建连 + 注册到同链目录
	srcConn, err := src.conn()
	if err != nil {
		t.Fatalf("src conn: %v", err)
	}
	// 借用方应复用源的同一连接实例
	borrowerConn, err := borrower.conn()
	if err != nil {
		t.Fatalf("borrower conn: %v", err)
	}
	if borrowerConn != srcConn {
		t.Fatalf("borrower did not reuse src connection: src=%+v borrower=%+v", srcConn, borrowerConn)
	}
	// 同链目录应能查到 src 注册的资源
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("src"); !found {
		t.Fatal("src connection not registered in chain Resources")
	}
	// 连不同目标的节点各自独立建连
	otherConn, err := other.conn()
	if err != nil {
		t.Fatalf("other conn: %v", err)
	}
	if otherConn == srcConn {
		t.Fatal("other should have independent connection, but got same as src")
	}
	if otherConn.addr != "deviceB" {
		t.Fatalf("other conn addr = %q, want deviceB", otherConn.addr)
	}
	t.Logf("PASS: src(seq=%d) reused by borrower; other(seq=%d) independent", srcConn.seq, otherConn.seq)
}

// TestChainConnectionCloseUnregister 验证源节点 Destroy 后从同链目录注销（CAS 不误删），借用方不再命中。
func TestChainConnectionCloseUnregister(t *testing.T) {
	chain := `{
  "ruleChain": {"id": "test_chain_conn_close", "name": "chain connection close", "root": true},
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {"id":"src","type":"test/conn","configuration":{"server":"deviceA"}},
      {"id":"borrower","type":"test/conn","configuration":{"server":"ref://src"}}
    ]
  }
}`
	eng, err := rulego.New("test_chain_conn_close", []byte(chain))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_chain_conn_close")
	ruleEng := eng.(*engine.RuleEngine)

	src := getTestNode(t, ruleEng, "src")
	if _, err := src.conn(); err != nil { // 建连 + 注册
		t.Fatalf("src conn: %v", err)
	}
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("src"); !found {
		t.Fatal("src should be registered before close")
	}
	// 销毁源节点：应从同链目录注销
	src.Destroy()
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("src"); found {
		t.Fatal("src should be unregistered after Destroy")
	}
	t.Log("PASS: src unregistered from chain Resources after Destroy")
}

// TestChainConnectionRefresh 验证源节点 Refresh（重连）后，借用方经同链 holder 拿到新连接实例。
// holder 指针（目录条目）不变，仅内部值更新——这是稳定间接层的核心价值。
func TestChainConnectionRefresh(t *testing.T) {
	chain := `{
  "ruleChain": {"id": "test_chain_conn_refresh", "name": "chain connection refresh", "root": true},
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {"id":"src","type":"test/conn","configuration":{"server":"deviceA"}},
      {"id":"borrower","type":"test/conn","configuration":{"server":"ref://src"}}
    ]
  }
}`
	eng, err := rulego.New("test_chain_conn_refresh", []byte(chain))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_chain_conn_refresh")
	ruleEng := eng.(*engine.RuleEngine)

	src := getTestNode(t, ruleEng, "src")
	borrower := getTestNode(t, ruleEng, "borrower")

	srcConn1, err := src.conn()
	if err != nil {
		t.Fatalf("src conn: %v", err)
	}
	borrowerConn1, err := borrower.conn()
	if err != nil {
		t.Fatalf("borrower conn: %v", err)
	}
	if borrowerConn1 != srcConn1 {
		t.Fatalf("borrower did not reuse src connection before refresh")
	}

	// 模拟源重连：Refresh 新连接，目录条目（holder 指针）不变
	srcConn2 := &testConn{addr: src.Server, seq: int(atomic.AddInt32(&testConnSeq, 1))}
	src.SharedNode.Refresh(srcConn2)

	// 借用方再次取连接，应拿到刷新后的新实例
	borrowerConn2, err := borrower.conn()
	if err != nil {
		t.Fatalf("borrower conn after refresh: %v", err)
	}
	if borrowerConn2 != srcConn2 {
		t.Fatalf("borrower did not get refreshed connection: got %+v want %+v", borrowerConn2, srcConn2)
	}
	if borrowerConn2 == srcConn1 {
		t.Fatal("borrower still holds stale connection after refresh")
	}
	t.Logf("PASS: borrower saw refresh src(seq %d -> %d)", srcConn1.seq, srcConn2.seq)
}
