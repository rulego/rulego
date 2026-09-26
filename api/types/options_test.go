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
	"time"

	"github.com/rulego/rulego/test/assert"
)

type stubOptionPool struct{}

func (p *stubOptionPool) Submit(task func()) error { task(); return nil }
func (p *stubOptionPool) Release()                 {}

type stubOptionNodePool struct{ NodePool }

type stubOptionParser struct{ Parser }

type stubOptionCache struct{ Cache }

type stubOptionRegistry struct{ ComponentRegistry }

// TestConfigOptions 表驱动验证各 Option 对 Config 字段的赋值
func TestConfigOptions(t *testing.T) {
	onDebug := func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error) {
	}
	onEnd := func(ctx RuleContext, msg RuleMsg, err error, relationType string) {}
	onCompleted := func(ctx RuleContext, snapshot RuleChainRunSnapshot) {}
	logger := NewStdLogger(nil)
	parser := &stubOptionParser{}
	nodePool := &stubOptionNodePool{}
	cache := &stubOptionCache{}
	registry := &stubOptionRegistry{}
	locker := NewLocalLocker()

	tests := []struct {
		name   string
		opt    Option
		verify func(t *testing.T, c Config)
	}{
		{
			name: "WithComponentsRegistry",
			opt:  WithComponentsRegistry(registry),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, registry, c.ComponentsRegistry)
			},
		},
		{
			name: "WithOnDebug",
			opt:  WithOnDebug(onDebug),
			verify: func(t *testing.T, c Config) {
				assert.NotNil(t, c.OnDebug)
			},
		},
		{
			name: "WithOnEndGlobal",
			opt:  WithOnEndGlobal(onEnd),
			verify: func(t *testing.T, c Config) {
				assert.NotNil(t, c.OnEnd)
			},
		},
		{
			name: "WithOnRuleChainCompletedGlobal",
			opt:  WithOnRuleChainCompletedGlobal(onCompleted),
			verify: func(t *testing.T, c Config) {
				assert.NotNil(t, c.OnRuleChainCompleted)
			},
		},
		{
			name: "WithOnEndWithFailure",
			opt:  WithOnEndWithFailure(false),
			verify: func(t *testing.T, c Config) {
				assert.False(t, c.OnEndWithFailure)
			},
		},
		{
			name: "WithPool",
			opt:  WithPool(&stubOptionPool{}),
			verify: func(t *testing.T, c Config) {
				assert.NotNil(t, c.Pool)
			},
		},
		{
			name: "WithNodePool",
			opt:  WithNodePool(nodePool),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, nodePool, c.NodePool)
			},
		},
		{
			name: "WithMsgMaxHops",
			opt:  WithMsgMaxHops(100),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, int64(100), c.MsgMaxHops)
			},
		},
		{
			name: "WithScriptMaxExecutionTime",
			opt:  WithScriptMaxExecutionTime(5 * time.Second),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, 5*time.Second, c.ScriptMaxExecutionTime)
			},
		},
		{
			name: "WithParser",
			opt:  WithParser(parser),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, parser, c.Parser)
			},
		},
		{
			name: "WithLogger",
			opt:  WithLogger(logger),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, logger, c.Logger)
			},
		},
		{
			name: "WithSecretKey",
			opt:  WithSecretKey("0123456789abcdef0123456789abcdef"),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, "0123456789abcdef0123456789abcdef", c.SecretKey)
			},
		},
		{
			name: "WithEndpointEnabled",
			opt:  WithEndpointEnabled(false),
			verify: func(t *testing.T, c Config) {
				assert.False(t, c.EndpointEnabled)
			},
		},
		{
			name: "WithLocker",
			opt:  WithLocker(locker),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, locker, c.Locker)
			},
		},
		{
			name: "WithOwner",
			opt:  WithOwner("tenant-a"),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, "tenant-a", c.Owner)
			},
		},
		{
			name: "WithCache",
			opt:  WithCache(cache),
			verify: func(t *testing.T, c Config) {
				assert.Equal(t, cache, c.Cache)
			},
		},
	}
	for _, tt := range tests {
		c := Config{}
		assert.Nil(t, tt.opt(&c))
		tt.verify(t, c)
	}
}

// TestWithRunLogMode 合法模式原样生效；未知模式回退 off（有/无 logger 两条路径）
func TestWithRunLogMode(t *testing.T) {
	valid := []RunLogMode{RunLogModeOff, RunLogModeSummary, RunLogModeDetail}
	for _, mode := range valid {
		c := Config{}
		assert.Nil(t, WithRunLogMode(mode)(&c))
		assert.Equal(t, mode, c.RunLogMode)
	}

	c := Config{Logger: NewStdLogger(nil)}
	assert.Nil(t, WithRunLogMode("typo")(&c))
	assert.Equal(t, RunLogModeOff, c.RunLogMode)

	c2 := Config{}
	assert.Nil(t, WithRunLogMode("typo")(&c2))
	assert.Equal(t, RunLogModeOff, c2.RunLogMode)
}

// TestWithDefaultPool 默认池创建后可提交任务并释放
func TestWithDefaultPool(t *testing.T) {
	c := Config{}
	assert.Nil(t, WithDefaultPool()(&c))
	assert.NotNil(t, c.Pool)

	done := make(chan struct{})
	assert.Nil(t, c.Pool.Submit(func() { close(done) }))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("task not executed by default pool")
	}
	c.Pool.Release()
}
