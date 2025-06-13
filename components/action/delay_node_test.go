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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestDelayNode(t *testing.T) {

	var targetNodeType = "delay"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &DelayNode{}, types.Configuration{
			"periodInSeconds": 60,
			"maxPendingMsgs":  1000,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  1,
		}, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  1,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  -1,
		}, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  1000,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		//第2条消息，因为队列已经满，报错
		//第3条消息，因为队列已经消费，成功
		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "BB",
				AfterSleep: time.Second * 1,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "CC",
				AfterSleep: time.Second * 1,
			},
		}
		var count int64
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			newCount := atomic.AddInt64(&count, 1)
			if newCount == 1 {
				assert.Equal(t, types.Failure, relationType)
			} else {
				assert.Equal(t, types.Success, relationType)
			}
		})
	})

	t.Run("ByPattern", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"PeriodInSecondsPattern": "${period}",
			"maxPendingMsgs":         1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue("period", "2")
		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Second * 3,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})

		//测试错误
		metaData.PutValue("period", "aa")
		msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Second * 3,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Failure, relationType)
		})
	})

	//覆盖模式
	t.Run("Overlay", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"periodInSeconds": 5,
			"overwrite":       true,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		//第2条消息，覆盖上一条
		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "BB",
				AfterSleep: time.Second * 7,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "CC",
				AfterSleep: time.Second * 7,
			},
		}
		var count int64
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			newCount := atomic.AddInt64(&count, 1)
			if newCount == 1 {
				assert.Equal(t, "BB", msg.GetData())
			} else {
				assert.Equal(t, "CC", msg.GetData())
			}
		})
	})
}
