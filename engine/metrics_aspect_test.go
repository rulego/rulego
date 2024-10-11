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

package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test/assert"
)

func TestMetricsAspect(t *testing.T) {
	ruleFile := loadFile("./test_metrics_chain.json")
	//测试函数
	action.Functions.Register("doErr", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(time.Millisecond * 100)
		ctx.TellFailure(msg, errors.New("error"))
	})
	action.Functions.Register("doSuccess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(time.Millisecond * 100)
		ctx.TellNext(msg, types.Success)
	})

	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("testMetricsAspect", ruleFile, WithConfig(config))
	assert.Nil(t, err)

	metaData := types.NewMetadata()
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg)
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 50)
	metrics := ruleEngine.GetMetrics().Get()
	//正在执行
	assert.Equal(t, int64(2), metrics.Current)

	time.Sleep(time.Millisecond * 500)
	//等待所有规则链支持完
	metrics = ruleEngine.GetMetrics().Get()
	assert.Equal(t, int64(0), metrics.Current)
	assert.Equal(t, int64(2), metrics.Total)
	assert.Equal(t, int64(2), metrics.Failed)
	assert.Equal(t, int64(4), metrics.Success)
	//重置
	ruleEngine.GetMetrics().Reset()
	assert.Equal(t, int64(0), ruleEngine.GetMetrics().Get().Total)
	assert.Equal(t, int64(0), ruleEngine.GetMetrics().Get().Failed)
	assert.Equal(t, int64(0), ruleEngine.GetMetrics().Get().Success)

	ruleEngine.OnMsg(msg)
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 500)
	metrics = ruleEngine.GetMetrics().Get()
	assert.Equal(t, int64(0), metrics.Current)
	assert.Equal(t, int64(2), metrics.Total)
	assert.Equal(t, int64(2), metrics.Failed)
	assert.Equal(t, int64(4), metrics.Success)

	//刷新，指标不变
	_ = ruleEngine.Reload()
	metrics = ruleEngine.GetMetrics().Get()
	assert.Equal(t, int64(0), metrics.Current)
	assert.Equal(t, int64(2), metrics.Total)
	assert.Equal(t, int64(2), metrics.Failed)
	assert.Equal(t, int64(4), metrics.Success)

}
