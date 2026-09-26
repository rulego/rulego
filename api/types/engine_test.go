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

package types

import (
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

// mockRuleEngine 仅记录被测选项会调用的 setter，其余接口方法由内嵌零值兜底。
type mockRuleEngine struct {
	RuleEngine
	config      Config
	aspects     []Aspect
	pool        RuleEnginePool
	maxWaiters  int64
	setWaitersN int
}

func (m *mockRuleEngine) SetConfig(config Config)               { m.config = config }
func (m *mockRuleEngine) SetAspects(aspects ...Aspect)          { m.aspects = aspects }
func (m *mockRuleEngine) SetRuleEnginePool(pool RuleEnginePool) { m.pool = pool }
func (m *mockRuleEngine) SetMaxReloadWaiters(maxWaiters int64) {
	m.maxWaiters = maxWaiters
	m.setWaitersN++
}

type stubEnginePool struct{ RuleEnginePool }

// TestRuleEngineOptions 验证各 RuleEngineOption 把参数透传给引擎 setter
func TestRuleEngineOptions(t *testing.T) {
	config := NewConfig()
	pool := &stubEnginePool{}
	aspect := &testAspect{order: 100}

	re := &mockRuleEngine{}
	assert.Nil(t, WithConfig(config)(re))
	assert.Equal(t, 2*time.Second, re.config.ScriptMaxExecutionTime)

	assert.Nil(t, WithAspects(aspect)(re))
	assert.Equal(t, 1, len(re.aspects))
	assert.Equal(t, aspect, re.aspects[0])

	assert.Nil(t, WithRuleEnginePool(pool)(re))
	assert.Equal(t, pool, re.pool)

	assert.Nil(t, WithMaxReloadWaiters(500)(re))
	assert.Equal(t, int64(500), re.maxWaiters)
	assert.Equal(t, 1, re.setWaitersN)
}
