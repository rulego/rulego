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
	"sync"
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
			"delayMs":        "60000",
			"maxPendingMsgs": 1000,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1,
		}, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": -1,
		}, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1000,
		}, Registry)
	})

	// 测试新的DelayMs字段（数值模式）
	t.Run("DelayMsNumeric", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 1200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})
	})

	// 测试新的DelayMs字段（模板模式）
	t.Run("DelayMsTemplate", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"delayMs":        "${delayTime}",
			"maxPendingMsgs": 1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue("delayTime", "2000")

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

		// 测试模板解析错误
		metaData.PutValue("delayTime", "invalid")
		msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "BB",
				AfterSleep: time.Second * 1,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Failure, relationType)
		})
	})

	// 延迟偏移：偏移时间大于或等于延迟，立即执行
	t.Run("DelayOffsetImmediate", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		// 等于延迟
		metaData.PutValue(KeyDelayOffsetMs, "1000")

		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})

		// 大于延迟
		metaData.PutValue(KeyDelayOffsetMs, "1500")
		msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "BB",
				AfterSleep: time.Millisecond * 200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})
	})

	// 延迟偏移：偏移时间小于延迟，按剩余时间延迟
	t.Run("DelayOffsetReduced", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"delayMs":        "2000",
			"maxPendingMsgs": 1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue(KeyDelayOffsetMs, "500")

		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 1800,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})
	})

	// 延迟偏移：元数据值非法，走失败链路
	t.Run("DelayOffsetInvalid", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue(KeyDelayOffsetMs, "invalid")

		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Failure, relationType)
		})
	})

	// 兼容旧配置：periodInSeconds + 延迟偏移
	t.Run("DelayOffsetWithPeriodInSeconds", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"periodInSeconds": 2,
			"maxPendingMsgs":  1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue(KeyDelayOffsetMs, "1000")

		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 1200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})
	})

	// 测试向后兼容性 - 旧的periodInSeconds字段
	t.Run("BackwardCompatibility", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 1200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})
	})

	t.Run("OnMsg", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"periodInSeconds": 1,
			"maxPendingMsgs":  1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		//第1条消息，成功
		//第2条消息，因为队列已经满，报错
		//第3条消息，成功，因为第1条消息已经被消费
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
		var wg sync.WaitGroup
		wg.Add(3)
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			// 根据消息数据判断期望的结果
			if msg.Data.Get() == "AA" || msg.Data.Get() == "CC" {
				assert.Equal(t, types.Success, relationType)
			} else {
				// 第二条消息BB应该失败，因为队列已满（maxPendingMsgs: 1）
				assert.Equal(t, types.Failure, relationType)
			}
			wg.Done()
		})
		wg.Wait()
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
