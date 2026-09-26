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

package aspect

import (
	"errors"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/metrics"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestMetricsAspect(t *testing.T) {
	// nil metrics instance is replaced with a fresh one
	aspect := NewMetricsAspect(nil)
	assert.NotNil(t, aspect.GetMetrics())

	// provided metrics instance is reused
	m := metrics.NewEngineMetrics()
	m.IncrementTotal()
	aspectWithMetrics := NewMetricsAspect(m)
	assert.Equal(t, m, aspectWithMetrics.GetMetrics())

	assert.Equal(t, 20, aspect.Order())
	assert.True(t, aspect.PointCut(nil, types.RuleMsg{}, ""))

	// New resets counters of the shared metrics instance
	clone := aspectWithMetrics.New().(*MetricsAspect)
	assert.Equal(t, int64(0), clone.GetMetrics().Get().Total)

	// New on a zero-value aspect creates its own metrics
	bare := &MetricsAspect{}
	fromBare := bare.New().(*MetricsAspect)
	assert.NotNil(t, fromBare.GetMetrics())

	ctx := test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {})
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	out, err := clone.Start(ctx, msg)
	assert.Nil(t, err)
	assert.Equal(t, msg, out)
	snapshot := clone.GetMetrics().Get()
	assert.Equal(t, int64(1), snapshot.Current)
	assert.Equal(t, int64(1), snapshot.Total)

	out = clone.End(ctx, msg, nil, types.Success)
	assert.Equal(t, msg, out)
	out = clone.End(ctx, msg, errors.New("fail"), types.Failure)
	assert.Equal(t, msg, out)
	snapshot = clone.GetMetrics().Get()
	assert.Equal(t, int64(1), snapshot.Success)
	assert.Equal(t, int64(1), snapshot.Failed)

	out = clone.Completed(ctx, msg)
	assert.Equal(t, msg, out)
	assert.Equal(t, int64(0), clone.GetMetrics().Get().Current)
}
