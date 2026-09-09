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
	"errors"
	"sync/atomic"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
)

// testConnEndpoint is a connection-holding endpoint stub for chain-scoped
// ref:// tests: local mode creates a testConn and registers it under the
// endpoint id; ref:// server borrows through the SharedNode resolution chain,
// exercising the same code path as protocol endpoints (kafka/mqtt/nacos).
type testConnEndpoint struct {
	impl.BaseEndpoint
	base.SharedNode[*testConn]
	RuleConfig types.Config
	Server     string
}

func (e *testConnEndpoint) Type() string { return "endpoint/testConn" }
func (e *testConnEndpoint) New() types.Node {
	return &testConnEndpoint{}
}

func (e *testConnEndpoint) Id() string { return e.Server }

func (e *testConnEndpoint) Init(rc types.Config, cfg types.Configuration) error {
	if v, ok := cfg["server"]; ok {
		e.Server, _ = v.(string)
	}
	e.RuleConfig = rc
	err := e.SharedNode.InitWithClose(rc, e.Type(), e.Server, true, func() (*testConn, error) {
		if e.Server == "" {
			return nil, errors.New("server is empty")
		}
		atomic.AddInt64(&testConnLiveCount, 1)
		return &testConn{addr: e.Server}, nil
	}, func(c *testConn) error {
		atomic.AddInt64(&testConnLiveCount, -1)
		return nil
	})
	// chainCtx（链上部署时注入）启用链内 ref:// 解析与注册
	e.SharedNode.BindChain(cfg)
	return err
}

func (e *testConnEndpoint) Start() error {
	_, err := e.SharedNode.GetSafely()
	return err
}

func (e *testConnEndpoint) Destroy() { _ = e.Close() }

func (e *testConnEndpoint) Close() error {
	_ = e.SharedNode.Close()
	e.BaseEndpoint.Destroy()
	return nil
}

func (e *testConnEndpoint) AddRouter(_ endpointApi.Router, _ ...interface{}) (string, error) {
	// 挂路由即触发连接解析：链部署两阶段下，这里验证 ref:// 在订阅期能拿到连接
	if _, err := e.SharedNode.GetSafely(); err != nil {
		return "", err
	}
	return "1", nil
}

func (e *testConnEndpoint) RemoveRouter(_ string, _ ...interface{}) error { return nil }

// Conn exposes the endpoint's resolved connection for test assertions.
func (e *testConnEndpoint) Conn() (*testConn, error) { return e.SharedNode.GetSafely() }

func init() {
	_ = rulego.Registry.Register(&testConnEndpoint{})
	_ = endpoint.Registry.Register(&testConnEndpoint{})
}
