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

package external

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestNetNode(t *testing.T) {
	var targetNodeType = "net"
	config := NetNodeConfiguration{
		Protocol: "tcp",
		Server:   "127.0.0.1:9999",
	}
	stop := make(chan struct{})
	//启动服务
	go createNetServer(config, stop)
	time.Sleep(time.Millisecond * 200)

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &NetNode{}, types.Configuration{
			"protocol":          "tcp",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"server":            "127.0.0.1:9999",
			"connectTimeout":    -1,
			"heartbeatInterval": -1,
		}, types.Configuration{
			"server":            "127.0.0.1:9999",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:9999",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:9999",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"server":            "127.0.0.1:9999",
			"heartbeatInterval": 5, // 增加心跳间隔以减少日志输出
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:6666",
			"connectTimeout":    60,
			"heartbeatInterval": 0, // 禁用心跳以避免无限重连
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Second * 3,
			},
			// 测试二进制数据处理
			{
				MetaData:   metaData,
				MsgType:    "BINARY_EVENT1",
				DataType:   types.BINARY,
				Data:       string([]byte{0x01, 0x02, 0x03, 0x04}), // 二进制数据
				AfterSleep: time.Millisecond * 200,
			},
			{
				MetaData:   metaData,
				MsgType:    "JSON_EVENT",
				DataType:   types.JSON,
				Data:       "{\"sensor\":\"temp\",\"value\":25.5}",
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}

		//time.Sleep(time.Second * 10)
		stop <- struct{}{}

	})
}

// 创建net服务
func createNetServer(config NetNodeConfiguration, stop chan struct{}) {
	//var err error
	// 根据配置的协议和地址，创建一个服务器监听器
	listener, err := net.Listen(config.Protocol, config.Server)
	if err != nil {
		return
	}
	go func() {
		for {
			select {
			case <-stop:
				// 接收到中断信号，退出循环
				listener.Close()
				return
			default:
			}
		}
	}()
	// 循环接受客户端的连接请求
	for {
		// 从监听器中获取一个客户端连接，返回连接对象和错误信息
		_, err := listener.Accept()
		if err != nil {
			if opError, ok := err.(*net.OpError); ok && opError.Err == net.ErrClosed {
				return
			} else {
				continue
			}
		}
	}
}

// 说明：external 包被 engine import，其测试不能 import endpoint/net/node_pool/engine
// （会触发 external→...→engine→external 循环）。因此用 fake SessionRegistry 测 NetNode
// 寻址逻辑（IsFromPool→NodePool.GetInstance→类型断言→Lookup→Send）。
// 真 endpoint/net 的 session 维护由 endpoint/net/session_test.go 覆盖。

// miniPool 测试用 NodePool，只实现 GetInstance。
type miniPool struct {
	instances map[string]interface{}
}

func (p *miniPool) GetInstance(id string) (interface{}, error) {
	if v, ok := p.instances[id]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("miniPool: not found %s", id)
}
func (p *miniPool) Lookup(id string) (any, bool) {
	v, err := p.GetInstance(id)
	if err != nil {
		return nil, false
	}
	return v, true
}
func (p *miniPool) Get(string) (types.SharedNodeCtx, bool)                          { return nil, false }
func (p *miniPool) AddNode(types.Node) (types.SharedNodeCtx, error)                 { return nil, nil }
func (p *miniPool) Load([]byte) (types.NodePool, error)                             { return nil, nil }
func (p *miniPool) LoadFromRuleChain(types.RuleChain) (types.NodePool, error)       { return nil, nil }
func (p *miniPool) NewFromEndpoint(types.EndpointDsl) (types.SharedNodeCtx, error)  { return nil, nil }
func (p *miniPool) NewFromRuleNode(types.RuleNode) (types.SharedNodeCtx, error)     { return nil, nil }
func (p *miniPool) Del(string)                                                      {}
func (p *miniPool) Stop()                                                           {}
func (p *miniPool) GetAll() []types.SharedNodeCtx                                   { return nil }
func (p *miniPool) GetAllDef() (map[string][]*types.RuleNode, error)                { return nil, nil }
func (p *miniPool) Range(func(key, value interface{}) bool)                         {}

// fakeRegistry 测试用 SessionRegistry
type fakeRegistry struct {
	mu       sync.Mutex
	sessions map[string]*endpoint.Session
}

func (f *fakeRegistry) Add(s *endpoint.Session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[s.Key()] = s
}
func (f *fakeRegistry) Remove(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, key)
}
func (f *fakeRegistry) Rekey(s *endpoint.Session, newKey string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, s.Key())
	s.SetKey(newKey)
	f.sessions[newKey] = s
}
func (f *fakeRegistry) Lookup(target string) []*endpoint.Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	if target == "" || target == "*" {
		var all []*endpoint.Session
		for _, s := range f.sessions {
			all = append(all, s)
		}
		return all
	}
	var out []*endpoint.Session
	if s, ok := f.sessions[target]; ok {
		out = append(out, s)
	}
	return out
}

// SendToTarget 实现 types.TargetSender（供 NetNode ref:// 寻址推送测试复用，
// 与 Lookup 共享同一份 session 寻址语义）。
func (f *fakeRegistry) SendToTarget(target string, data []byte) (sent, failed int, err error) {
	sessions := f.Lookup(target)
	if len(sessions) == 0 {
		return 0, 0, fmt.Errorf("no session matched target=%q", target)
	}
	for _, s := range sessions {
		if e := s.Sender.Send(data); e != nil {
			failed++
			if err == nil {
				err = e
			}
		} else {
			sent++
		}
	}
	return sent, failed, err
}

// fakeSender 记录收到的数据
type fakeSender struct {
	mu       sync.Mutex
	received [][]byte
}

func (s *fakeSender) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	s.received = append(s.received, cp)
	return nil
}

func newAddrNode(t *testing.T, poolTarget string, target string) (*NetNode, *miniPool) {
	pool := &miniPool{instances: map[string]interface{}{}}
	cfg := types.NewConfig()
	cfg.NodePool = pool
	node := &NetNode{}
	if err := node.Init(cfg, types.Configuration{
		"server": "ref://" + poolTarget,
		"target": target,
	}); err != nil {
		t.Fatalf("NetNode Init: %v", err)
	}
	return node, pool
}

// TestNetNodeAddressingPush 按 deviceId 寻址推送：NetNode → Lookup(DEV_001) → Send
func TestNetNodeAddressingPush(t *testing.T) {
	sender := &fakeSender{}
	reg := &fakeRegistry{sessions: map[string]*endpoint.Session{}}
	reg.Add(endpoint.NewSession("DEV_001", sender))

	node, pool := newAddrNode(t, "ep", "DEV_001")
	defer node.Destroy()
	pool.instances["ep"] = reg

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "HELLO", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("OnMsg err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.received) != 1 || string(sender.received[0]) != "HELLO\n" {
		t.Fatalf("sender received %v, want [\"HELLO\\n\"]", sender.received)
	}
}

// TestNetNodeAddressingBroadcast target=* 广播所有 session
func TestNetNodeAddressingBroadcast(t *testing.T) {
	s1, s2 := &fakeSender{}, &fakeSender{}
	reg := &fakeRegistry{sessions: map[string]*endpoint.Session{}}
	reg.Add(endpoint.NewSession("DEV_A", s1))
	reg.Add(endpoint.NewSession("DEV_B", s2))

	node, pool := newAddrNode(t, "ep", "*")
	defer node.Destroy()
	pool.instances["ep"] = reg

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "BCAST", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("OnMsg err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if len(s1.received) != 1 || len(s2.received) != 1 {
		t.Fatalf("both should receive: s1=%d s2=%d", len(s1.received), len(s2.received))
	}
}

// TestNetNodeAddressingNoMatch target 未命中 → TellFailure（不伪装成功）
func TestNetNodeAddressingNoMatch(t *testing.T) {
	reg := &fakeRegistry{sessions: map[string]*endpoint.Session{}}
	reg.Add(endpoint.NewSession("DEV_001", &fakeSender{}))

	node, pool := newAddrNode(t, "ep", "GHOST")
	defer node.Destroy()
	pool.instances["ep"] = reg

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "X", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expect TellFailure for no-match target, got success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
