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

	// Test the new DelayMs field (numeric mode)
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

	// Testing the new DelayMs field (template mode)
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

		// Test template parsing error
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

	// Delayed offset: The offset time is greater than or equal to the delay and executed immediately
	t.Run("DelayOffsetImmediate", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"delayMs":        "1000",
			"maxPendingMsgs": 1,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		// This equals delay
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

		// Greater than delay
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

	// Delay offset: If the offset time is less than the delay, the remaining time is used for delay
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

	// Delay offset: Metadata values are invalid and follow the failure route
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

	// Compatible with older configurations: periodInSeconds + delay offset
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

	// Testing backward compatibility - the old periodInSeconds field
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
		//The first message: success
		//Message 2: Because the queue is full, an error is reported
		//The third message succeeds, because the first message has already been consumed
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
			// Judge the expected outcome based on the message data
			if msg.Data.Get() == "AA" || msg.Data.Get() == "CC" {
				assert.Equal(t, types.Success, relationType)
			} else {
				// The second message BB should fail because the queue is full (maxPendingMsgs: 1)
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

		//Test error
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

	//Coverage mode
	t.Run("Overlay", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"periodInSeconds": 2,
			"overwrite":       true,
		}, Registry)
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		//Message 2, override the previous one
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
				AfterSleep: time.Millisecond * 2500,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "CC",
				AfterSleep: time.Second * 3,
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
