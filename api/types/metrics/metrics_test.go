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

package metrics

import (
	"sync"
	"testing"

	"github.com/rulego/rulego/test/assert"
)

// TestEngineMetrics 计数增减、快照隔离与重置
func TestEngineMetrics(t *testing.T) {
	m := NewEngineMetrics()
	assert.Equal(t, EngineMetrics{Current: 0, Total: 0, Failed: 0, Success: 0}, m.Get())

	m.IncrementCurrent()
	m.IncrementCurrent()
	m.DecrementCurrent()
	m.IncrementTotal()
	m.IncrementFailed()
	m.IncrementSuccess()
	got := m.Get()
	assert.Equal(t, int64(1), got.Current)
	assert.Equal(t, int64(1), got.Total)
	assert.Equal(t, int64(1), got.Failed)
	assert.Equal(t, int64(1), got.Success)

	// Get 返回副本，改副本不影响内部计数
	got.Current = 99
	assert.Equal(t, int64(1), m.Get().Current)

	m.Reset()
	assert.Equal(t, EngineMetrics{}, m.Get())
}

// TestEngineMetricsConcurrent 并发计数不丢失
func TestEngineMetricsConcurrent(t *testing.T) {
	m := NewEngineMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.IncrementCurrent()
			m.IncrementTotal()
			m.IncrementSuccess()
			m.DecrementCurrent()
		}()
	}
	wg.Wait()

	got := m.Get()
	assert.Equal(t, int64(0), got.Current)
	assert.Equal(t, int64(50), got.Total)
	assert.Equal(t, int64(50), got.Success)
}
