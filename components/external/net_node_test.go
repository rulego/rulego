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
	//Start the server
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
			"heartbeatInterval": 5, // Increase heartbeat intervals to reduce log output
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:6666",
			"connectTimeout":    60,
			"heartbeatInterval": 0, // Disable heartbeats to avoid infinite reconnection
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
			// Test binary data processing
			{
				MetaData:   metaData,
				MsgType:    "BINARY_EVENT1",
				DataType:   types.BINARY,
				Data:       string([]byte{0x01, 0x02, 0x03, 0x04}), // Binary data
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

// Create NET services
func createNetServer(config NetNodeConfiguration, stop chan struct{}) {
	//var err error
	// Create a server listener based on the configured protocol and address
	listener, err := net.Listen(config.Protocol, config.Server)
	if err != nil {
		return
	}
	go func() {
		for {
			select {
			case <-stop:
				// Receive an interrupt signal and exit the loop
				listener.Close()
				return
			default:
			}
		}
	}()
	// Loop to accept connection requests from clients
	for {
		// Obtain a client connection from the listener, returning the connection object and error information
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

// Note: External packages are imported by engine, and their tests cannot import endpoint/net/node_pool/engine
// (This will trigger external→... →engine→external loop). Therefore, we used fake SessionRegistry to test NetNode
// Addressing logic (IsFromPool→NodePool.GetInstance→type asserts →Lookup→Send).
// Session maintenance for true endpoint/net is covered by endpoint/net/session_test.go.

// miniPool tests use NodePool, which only implements GetInstance.
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
func (p *miniPool) Get(string) (types.SharedNodeCtx, bool)                         { return nil, false }
func (p *miniPool) AddNode(types.Node) (types.SharedNodeCtx, error)                { return nil, nil }
func (p *miniPool) Load([]byte) (types.NodePool, error)                            { return nil, nil }
func (p *miniPool) LoadFromRuleChain(types.RuleChain) (types.NodePool, error)      { return nil, nil }
func (p *miniPool) NewFromEndpoint(types.EndpointDsl) (types.SharedNodeCtx, error) { return nil, nil }
func (p *miniPool) NewFromRuleNode(types.RuleNode) (types.SharedNodeCtx, error)    { return nil, nil }
func (p *miniPool) Del(string)                                                     {}
func (p *miniPool) Stop()                                                          {}
func (p *miniPool) GetAll() []types.SharedNodeCtx                                  { return nil }
func (p *miniPool) GetAllDef() (map[string][]*types.RuleNode, error)               { return nil, nil }
func (p *miniPool) Range(func(key, value interface{}) bool)                        {}

// fakeRegistry tests SessionRegistry
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

// SendToTarget implements types.TargetSender (for NetNode ref:// addressing push test reuse,
// Shares the same session addressing semantics with Lookup).
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

// fakeSender records the data received
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

// TestNetNodeAddressingPush Address Push by deviceId: NetNode → Lookup(DEV_001) → Send
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

// TestNetNodeAddressingBroadcast target=* Broadcast all sessions
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

// TestNetNodeAddressingNoMatch target missed → TellFailure (not disguised successfully)
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
