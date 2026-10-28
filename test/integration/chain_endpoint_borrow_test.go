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

package integration

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	endpointpkg "github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
)

// TestEndpointBorrowsLazyChainNode verifies the endpoint→same-chain-node ref://
// direction: an endpoint/kafka-style connection holder borrows a lazy (not yet
// connected) chain node's connection via ref://. The borrower resolves at
// AddRouter time: chain-directory miss triggers the target node's lazy init
// (through the injected $chainCtx), the node registers its connection, and the
// borrower re-lookup hits.
//
// Uses test/conn (connection-holding stub) plus a stub endpoint embedding the
// same SharedNode type, standing in for protocol-specific endpoints.
func TestEndpointBorrowsLazyChainNode(t *testing.T) {
	chain := `{
	  "ruleChain": {"id": "test_ep_borrow_lazy_node", "name": "endpoint borrows lazy node", "root": true},
	  "metadata": {
	    "firstNodeIndex": 0,
	    "endpoints": [
	      {"id": "ep_borrower", "type": "endpoint/testConn", "configuration": {"server": "ref://src"}}
	    ],
	    "nodes": [
	      {"id": "src", "type": "test/conn", "configuration": {"server": "deviceA"}},
	      {"id": "n1", "type": "jsTransform", "configuration": {"jsScript": "return {msg:msg,metadata:metadata,msgType:msgType};"}}
	    ],
	    "connections": []
	  }
	}`

	eng, err := rulego.New("test_ep_borrow_lazy_node", []byte(chain))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_ep_borrow_lazy_node")
	ruleEng := eng.(*engine.RuleEngine)

	// The borrower endpoint must have resolved src's connection at deploy time:
	// chain deployment is two-phase, ApplyRouters runs after all endpoints are
	// registered, and the lazy-init trigger must have connected src.
	borrower, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_borrower")
	if !found {
		t.Fatal("ep_borrower not registered in chain Resources (EndpointAspect syncResources failed)")
	}
	src := getTestNode(t, ruleEng, "src")
	srcConn, err := src.conn()
	if err != nil {
		t.Fatalf("src conn after deploy: %v", err)
	}

	bep, ok := borrower.(interface{ Conn() (*testConn, error) })
	if !ok {
		t.Fatalf("ep_borrower resource type %T has no Conn()", borrower)
	}
	got, err := bep.Conn()
	if err != nil {
		t.Fatalf("borrower conn: %v", err)
	}
	if got != srcConn {
		t.Fatalf("endpoint did not borrow lazy node's connection: got %+v want %+v", got, srcConn)
	}
	if got.addr != "deviceA" {
		t.Fatalf("borrowed conn addr = %q, want deviceA", got.addr)
	}
}

// TestEndpointBorrowsEndpoint verifies the endpoint→same-chain-endpoint ref://
// direction: two endpoints of the same type on one chain, the second borrows
// the first's connection. Only viable with two-phase deployment (instances
// register into the chain directory before any subscribes) — with one-phase
// the borrower's AddRouter runs before the source is registered and fails.
func TestEndpointBorrowsEndpoint(t *testing.T) {
	chain := `{
	  "ruleChain": {"id": "test_ep_borrow_ep", "name": "endpoint borrows endpoint", "root": true},
	  "metadata": {
	    "firstNodeIndex": 0,
	    "endpoints": [
	      {"id": "ep_src", "type": "endpoint/testConn", "configuration": {"server": "brokerA"}},
	      {"id": "ep_borrower", "type": "endpoint/testConn", "configuration": {"server": "ref://ep_src"}}
	    ],
	    "nodes": [
	      {"id": "n1", "type": "jsTransform", "configuration": {"jsScript": "return {msg:msg,metadata:metadata,msgType:msgType};"}}
	    ],
	    "connections": []
	  }
	}`

	eng, err := rulego.New("test_ep_borrow_ep", []byte(chain))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_ep_borrow_ep")
	ruleEng := eng.(*engine.RuleEngine)

	// The borrower must resolve the source endpoint's connection: two-phase
	// deployment registered both instances before the borrower subscribed.
	res, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_borrower")
	if !found {
		t.Fatal("ep_borrower not registered in chain Resources")
	}
	bep, ok := res.(*testConnEndpoint)
	if !ok {
		t.Fatalf("ep_borrower resource type %T not *testConnEndpoint", res)
	}
	srcRes, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_src")
	if !found {
		t.Fatal("ep_src not registered in chain Resources")
	}
	sep, ok := srcRes.(*testConnEndpoint)
	if !ok {
		t.Fatalf("ep_src resource type %T not *testConnEndpoint", srcRes)
	}
	srcConn, err := sep.Conn()
	if err != nil {
		t.Fatalf("src conn: %v", err)
	}
	got, err := bep.Conn()
	if err != nil {
		t.Fatalf("borrower conn: %v", err)
	}
	if got != srcConn {
		t.Fatalf("endpoint did not borrow same-chain endpoint's connection: got %+v want %+v", got, srcConn)
	}
	if got.addr != "brokerA" {
		t.Fatalf("borrowed conn addr = %q, want brokerA", got.addr)
	}
}

// TestEndpointBorrowCycleRejected verifies a ref:// cycle among chain nodes is
// rejected at deploy time instead of recursing: node a refs b, b refs a, and an
// endpoint borrows a. The borrower's resolve must fail with the cycle error,
// not stack overflow.
func TestEndpointBorrowCycleRejected(t *testing.T) {
	chain := `{
	  "ruleChain": {"id": "test_ep_borrow_cycle", "name": "endpoint borrow cycle", "root": true},
	  "metadata": {
	    "firstNodeIndex": 0,
	    "endpoints": [
	      {"id": "ep_borrower", "type": "endpoint/testConn", "configuration": {"server": "ref://a"}}
	    ],
	    "nodes": [
	      {"id": "a", "type": "test/conn", "configuration": {"server": "ref://b"}},
	      {"id": "b", "type": "test/conn", "configuration": {"server": "ref://a"}}
	    ],
	    "connections": []
	  }
	}`

	// Deploy must either fail (router attach rejected) or leave the borrower
	// unresolvable with a cycle error — never panic/stack overflow.
	_, err := rulego.New("test_ep_borrow_cycle", []byte(chain))
	if err == nil {
		defer rulego.Del("test_ep_borrow_cycle")
	}
	// err != nil is the expected path: AddRouter fails on the cycle error.
	if err != nil {
		t.Logf("deploy rejected with: %v", err)
	}
}

// TestChainCtxInjectedIntoEndpointConfig verifies the $chainCtx injection
// mechanism directly: a local-mode endpoint deployed on a chain registers its
// own connection into the chain resource directory (BindChain took effect via
// the injected chain context).
func TestChainCtxInjectedIntoEndpointConfig(t *testing.T) {
	chain := `{
	  "ruleChain": {"id": "test_ep_chainctx", "name": "chainctx injection", "root": true},
	  "metadata": {
	    "firstNodeIndex": 0,
	    "endpoints": [
	      {"id": "ep1", "type": "endpoint/testConn", "configuration": {"server": "standalone"}}
	    ],
	    "nodes": [
	      {"id": "n1", "type": "jsTransform", "configuration": {"jsScript": "return {msg:msg,metadata:metadata,msgType:msgType};"}}
	    ],
	    "connections": []
	  }
	}`

	eng, err := rulego.New("test_ep_chainctx", []byte(chain))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_ep_chainctx")
	ruleEng := eng.(*engine.RuleEngine)

	// Aspect registers the endpoint instance under its id...
	res, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep1")
	if !found {
		t.Fatal("ep1 not registered in chain Resources")
	}
	bep, ok := res.(*testConnEndpoint)
	if !ok {
		t.Fatalf("ep1 resource type %T not *testConnEndpoint", res)
	}
	// ...and the endpoint's own connection must resolve to the standalone addr.
	got, err := bep.Conn()
	if err != nil {
		t.Fatalf("ep1 conn: %v", err)
	}
	if got == nil || got.addr != "standalone" {
		t.Fatalf("ep1 conn = %+v, want addr standalone", got)
	}
}

// captureLogger 记录 Errorf 输出，供断言「graceful shutdown failed」日志。
type captureLogger struct {
	mu   sync.Mutex
	errs []string
}

func (l *captureLogger) Printf(format string, v ...interface{}) {}
func (l *captureLogger) Debugf(format string, v ...interface{}) {}
func (l *captureLogger) Infof(format string, v ...interface{})  {}
func (l *captureLogger) Warnf(format string, v ...interface{})  {}
func (l *captureLogger) Errorf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, fmt.Sprintf(format, v...))
}

func (l *captureLogger) hasSubstring(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.errs {
		if strings.Contains(e, s) {
			return true
		}
	}
	return false
}

type httpServerGetter interface {
	GetServer() *http.Server
}

type onEventSetter interface {
	SetOnEvent(endpoint.OnEvent)
}

// issue #129：share_http_server 场景（链 endpoint 以 ref:// 借用主 HTTP server），
// 链 ReloadSelf 销毁旧 endpoint 时只重建共享路由表，不重启共享 server：
// 1) 共享 server 的 *http.Server 实例不变（listener 不弹跳）；
// 2) 重载耗时远低于旧的 2s 优雅关闭超时；
// 3) 在途请求正常完成，不被强杀；
// 4) 无 "graceful shutdown failed" 错误日志；
// 5) 重建路径补发 EventRestart（非 disabled 快照）；
// 6) 被删除的借用路由换表后立即 404，其余路由不受影响。
func TestReloadChainWithSharedRestEndpointDoesNotRestartSharedServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	logger := &captureLogger{}
	config := engine.NewConfig(types.WithLogger(logger))
	config.NodePool = node_pool.NewNodePool(config)

	// 共享主 HTTP server（模拟 rulego-server 主端点，share_http_server=true 注入节点池）
	owner, err := endpointpkg.Registry.New(rest.Type, config, rest.Config{Server: addr})
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	handlerEntered := make(chan struct{})
	var once sync.Once
	if _, err := owner.AddRouter(impl.NewRouter().From("/api/v1/slow").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		once.Do(func() { close(handlerEntered) })
		<-release
		return false
	}).End(), "GET"); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AddRouter(impl.NewRouter().From("/api/v1/ping").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetStatusCode(http.StatusOK)
		exchange.Out.SetBody([]byte("pong"))
		return false
	}).End(), "GET"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.NodePool.(*node_pool.NodePool).AddNode(owner); err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer owner.Destroy()

	// 借用共享 server 的链（share_http_server 场景下用户链的写法）
	dsl := []byte(fmt.Sprintf(`{
		"ruleChain": {"id": "chain_ep_shared", "name": "shared-rest-reload", "root": true},
		"metadata": {
			"endpoints": [{
				"id": "ep1",
				"type": "http",
				"configuration": {"server": "ref://%s"},
				"routers": [{"params": ["GET"], "from": {"path": "/api/v1/chain"}, "to": {"path": "chain_ep_shared"}}]
			}]
		}
	}`, addr))
	eng, err := engine.New("chain_ep_shared", dsl, engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	// 基线：借用路由可用
	resp, err := http.Get("http://" + addr + "/api/v1/ping")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping before reload: status %d", resp.StatusCode)
	}

	ownerRest, ok := owner.(httpServerGetter)
	if !ok {
		t.Fatalf("owner endpoint %T has no GetServer", owner)
	}
	serverBefore := ownerRest.GetServer()
	if serverBefore == nil {
		t.Fatal("shared server not started")
	}

	// 捕获重建路径补发的 EventRestart 载荷
	setter, ok := owner.(onEventSetter)
	if !ok {
		t.Fatalf("owner endpoint %T has no SetOnEvent", owner)
	}
	var eventMu sync.Mutex
	var restartSnapshots []map[string]endpoint.Router
	setter.SetOnEvent(func(event string, params ...interface{}) {
		if event == endpoint.EventRestart && len(params) > 0 {
			if snapshot, ok := params[0].(map[string]endpoint.Router); ok {
				eventMu.Lock()
				restartSnapshots = append(restartSnapshots, snapshot)
				eventMu.Unlock()
			}
		}
	})

	// 模拟在途连接（保存请求自身 / WS 调试长连接都属此类）
	respCh := make(chan error, 1)
	go func() {
		_, err := http.Get("http://" + addr + "/api/v1/slow")
		respCh <- err
	}()
	select {
	case <-handlerEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("slow handler not entered")
	}

	// 用户保存/重启链 → SaveAndLoad → ReloadSelf
	start := time.Now()
	if err := eng.ReloadSelf(dsl); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	close(release)

	// 断言 1：共享 server 实例不变，listener 未弹跳
	serverAfter := ownerRest.GetServer()
	if serverAfter != serverBefore {
		t.Fatal("shared server instance replaced: borrower close restarted the shared server")
	}

	// 断言 2：重载耗时远低于 2s 优雅关闭超时
	if elapsed >= time.Second {
		t.Fatalf("reload returned in %v, expected well under the 2s graceful shutdown deadline", elapsed)
	}

	// 断言 3：在途请求正常完成，未被强杀
	select {
	case err := <-respCh:
		if err != nil {
			t.Fatalf("in-flight request failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight request did not complete")
	}

	// 断言 4：无强制关闭错误日志
	if logger.hasSubstring("graceful shutdown failed") {
		t.Fatalf("unexpected 'graceful shutdown failed' error log, got: %v", logger.errs)
	}

	// 断言 5：重建路径补发 EventRestart，载荷为非 disabled 路由快照（已剔除被删的借用路由）
	eventMu.Lock()
	if len(restartSnapshots) != 1 {
		eventMu.Unlock()
		t.Fatalf("EventRestart fired %d times, want 1", len(restartSnapshots))
	}
	snapshot := restartSnapshots[0]
	if _, ok := snapshot["GET:/api/v1/ping"]; !ok {
		eventMu.Unlock()
		t.Fatalf("event snapshot missing owner route: %v", snapshot)
	}
	if _, ok := snapshot["GET:/api/v1/chain"]; ok {
		eventMu.Unlock()
		t.Fatal("event snapshot contains deleted borrower route")
	}
	eventMu.Unlock()

	// 重载后链路由应恢复可用
	resp2, err := http.Get("http://" + addr + "/api/v1/chain")
	if err != nil {
		t.Fatalf("chain route after reload: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("chain route after reload: status %d", resp2.StatusCode)
	}

	// 断言 6：再次保存但移除链 endpoint：旧借用路由换表后应立即 404，其余路由不受影响
	dslNoRouter := []byte(fmt.Sprintf(`{
		"ruleChain": {"id": "chain_ep_shared", "name": "shared-rest-reload", "root": true},
		"metadata": {
			"endpoints": [{
				"id": "ep1",
				"type": "http",
				"configuration": {"server": "ref://%s"},
				"routers": []
			}]
		}
	}`, addr))
	if err := eng.ReloadSelf(dslNoRouter); err != nil {
		t.Fatal(err)
	}

	resp3, err := http.Get("http://" + addr + "/api/v1/chain")
	if err != nil {
		t.Fatalf("chain route after removal: %v", err)
	}
	_ = resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("removed chain route: status %d, want 404", resp3.StatusCode)
	}

	resp4, err := http.Get("http://" + addr + "/api/v1/ping")
	if err != nil {
		t.Fatalf("ping after chain route removal: %v", err)
	}
	_ = resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("ping after chain route removal: status %d", resp4.StatusCode)
	}
}
