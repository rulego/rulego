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

package action

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"sync/atomic"
	"testing"
	"time"
)

func TestDelayNodeOnMsg(t *testing.T) {
	var node DelayNode
	var configuration = make(types.Configuration)
	configuration["periodInSeconds"] = 1
	configuration["maxPendingMsgs"] = 1
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	var count int64
	ctx := test.NewRuleContextFull(config, &node, func(msg types.RuleMsg, relationType string, err2 error) {
		atomic.AddInt64(&count, 1)
		if count == 1 {
			assert.Equal(t, types.Failure, relationType)
		} else {
			assert.Equal(t, types.Success, relationType)
		}

	})
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "test")

	//第1条消息
	msg := ctx.NewMsg("ACTIVITY_EVENT", metaData, "AA")
	node.OnMsg(ctx, msg)

	time.Sleep(time.Millisecond * 200)
	//第2条消息，因为队列已经满，报错
	msg = ctx.NewMsg("ACTIVITY_EVENT", metaData, "BB")
	node.OnMsg(ctx, msg)

	time.Sleep(time.Second * 1)

	//第3条消息，因为队列已经消费，成功
	msg = ctx.NewMsg("ACTIVITY_EVENT", metaData, "CC")
	node.OnMsg(ctx, msg)

	time.Sleep(time.Second * 1)

}

func TestDelayNodeByPattern(t *testing.T) {
	var node DelayNode
	var configuration = make(types.Configuration)
	configuration["PeriodInSecondsPattern"] = "${period}"
	configuration["maxPendingMsgs"] = 1
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContextFull(config, &node, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "test")
	metaData.PutValue("period", "2")

	msg := ctx.NewMsg("ACTIVITY_EVENT", metaData, "AA")
	node.OnMsg(ctx, msg)

	time.Sleep(3)
}
