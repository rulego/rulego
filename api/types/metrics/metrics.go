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
	"sync/atomic"
)

// EngineMetrics holds various metrics for the rule engine execution.
type EngineMetrics struct {
	Current int64 // Number of currently executing engine
	Total   int64 // Total number of engine executions
	Failed  int64 // Number of failed chains executions
	Success int64 // Number of successful chains executions
}

// NewEngineMetrics creates a new instance of EngineMetrics.
func NewEngineMetrics() *EngineMetrics {
	m := &EngineMetrics{}
	return m
}

// IncrementCurrent increases the count of current executions.
func (m *EngineMetrics) IncrementCurrent() {
	atomic.AddInt64(&m.Current, 1)
}

// DecrementCurrent decreases the count of current executions.
func (m *EngineMetrics) DecrementCurrent() {
	atomic.AddInt64(&m.Current, -1)
}

// IncrementTotal increases the total count of executions.
func (m *EngineMetrics) IncrementTotal() {
	atomic.AddInt64(&m.Total, 1)
}

// IncrementFailed increases the count of failed executions.
func (m *EngineMetrics) IncrementFailed() {
	atomic.AddInt64(&m.Failed, 1)
}

// IncrementSuccess increases the count of successful executions.
func (m *EngineMetrics) IncrementSuccess() {
	atomic.AddInt64(&m.Success, 1)
}

// Get returns a copy of the current metrics.
func (m *EngineMetrics) Get() EngineMetrics {
	return EngineMetrics{
		Current: atomic.LoadInt64(&m.Current),
		Total:   atomic.LoadInt64(&m.Total),
		Failed:  atomic.LoadInt64(&m.Failed),
		Success: atomic.LoadInt64(&m.Success),
	}
}

// Reset resets all metrics to zero.
func (m *EngineMetrics) Reset() {
	atomic.StoreInt64(&m.Current, 0)
	atomic.StoreInt64(&m.Total, 0)
	atomic.StoreInt64(&m.Failed, 0)
	atomic.StoreInt64(&m.Success, 0)
}
