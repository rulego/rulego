/*
 * Copyright 2025 The RuleGo Authors.
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

package external

import (
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestCacheGetNode(t *testing.T) {
	var targetNodeType = "cacheGet"
	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &CacheGetNode{}, types.Configuration{
			"outputMode": CacheOutputModeNewMsg,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "key1"}, {CacheLevelChain, "key2"}},
			"outputMode": CacheOutputModeNewMsg,
		}, types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "key1"}, {CacheLevelChain, "key2"}},
			"outputMode": CacheOutputModeNewMsg,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "key1"}},
			"outputMode": CacheOutputModeNewMsg,
		}, Registry)
	})

	t.Run("OnMsgSGD", func(t *testing.T) {
		chainSetNode, _ := test.CreateAndInitNode("cacheSet", types.Configuration{
			"items": []map[string]interface{}{
				{"level": CacheLevelChain, "key": "testKey", "value": "testValue", "ttl": "1h"},
				{"level": CacheLevelChain, "key": "testKey2", "value": "testValue2", "ttl": "1h"},
			},
		}, Registry)

		globalSetNode, _ := test.CreateAndInitNode("cacheSet", types.Configuration{
			"items": []map[string]interface{}{
				{"level": CacheLevelGlobal, "key": "globalTestKey", "value": "globalTestValue", "ttl": "1h"},
			},
		}, Registry)

		globalGetNode, _ := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelGlobal, "globalTestKey"}},
			"outputMode": 0,
		}, Registry)

		globalGetNodeFromChain, _ := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "globalTestKey"}},
			"outputMode": 0,
		}, Registry)

		//合并到元数据
		node1, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "testKey"}},
			"outputMode": 0,
		}, Registry)
		assert.Nil(t, err)

		//合并到消息负荷
		node2, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "testKey"}},
			"outputMode": 1,
		}, Registry)
		assert.Nil(t, err)
		//新消息负荷
		node3, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "testKey"}},
			"outputMode": 2,
		}, Registry)
		assert.Nil(t, err)

		nodeByPrefix, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "test*"}},
			"outputMode": 2,
		}, Registry)
		assert.Nil(t, err)
		deleteNode, _ := test.CreateAndInitNode("cacheDelete", types.Configuration{
			"keys": []LevelKey{{CacheLevelChain, "testKey"}},
		}, Registry)

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{{
			MetaData:   metaData,
			MsgType:    "TEST_MSG",
			Data:       "{\"oldKey\":\"oldValue\"}",
			DataType:   types.JSON,
			AfterSleep: time.Millisecond * 200,
		}}
		textMsgList := []test.Msg{{
			MetaData:   metaData,
			MsgType:    "TEST_MSG",
			Data:       "{\"oldKey\":\"oldValue\"}",
			DataType:   types.TEXT,
			AfterSleep: time.Millisecond * 200,
		}}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    chainSetNode,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, relationType, types.Success)
				},
			},
			{
				Node:    globalSetNode,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, relationType, types.Success)
				},
			},
			{
				Node:    globalGetNode,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "globalTestValue", msg.Metadata.GetValue("globalTestKey"))
				},
			},
			{
				Node:    globalGetNodeFromChain,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "", msg.Metadata.GetValue("globalTestKey"))
				},
			},
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "testValue", msg.Metadata.GetValue("testKey"))
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "", msg.Metadata.GetValue("testKey"))
					assert.Equal(t, `{"oldKey":"oldValue","testKey":"testValue"}`, msg.GetData())
				},
			},
			{
				Node:    node2,
				MsgList: textMsgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, relationType, types.Failure)
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "", msg.Metadata.GetValue("testKey"))
					assert.Equal(t, "{\"testKey\":\"testValue\"}", msg.GetData())
				},
			},
			{
				Node:    nodeByPrefix,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "", msg.Metadata.GetValue("testKey"))
					assert.Equal(t, "{\"testKey\":\"testValue\",\"testKey2\":\"testValue2\"}", msg.GetData())
				},
			},
			{
				Node:    deleteNode,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, relationType, types.Success)
				},
			},
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "", msg.Metadata.GetValue("testKey"))
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, nil, item.Callback)
		}
		time.Sleep(time.Second * 1)
	})

	t.Run("CacheGetMissingKeyNewMsgMode", func(t *testing.T) {
		node, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "missingKey"}},
			"outputMode": 2,
		}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{{
			MsgType:    "TEST_MSG",
			Data:       "{}",
			AfterSleep: time.Millisecond * 200,
		}}

		test.NodeOnMsgWithChildren(t, node, msgList, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Failure, relationType)
			assert.Equal(t, types.ErrCacheMiss.Error(), err.Error())
		})
	})

	t.Run("CacheGetMissingKeyMergeModes", func(t *testing.T) {
		// Mode 0: MergeToMetadata
		node0, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "missingKey0"}},
			"outputMode": 0,
		}, Registry)
		assert.Nil(t, err)

		// Mode 1: MergeToMsg
		node1, err := test.CreateAndInitNode("cacheGet", types.Configuration{
			"keys":       []LevelKey{{CacheLevelChain, "missingKey1"}},
			"outputMode": 1,
		}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{{
			MsgType:    "TEST_MSG",
			Data:       "{\"original\":\"data\"}",
			DataType:   types.JSON,
			AfterSleep: time.Millisecond * 200,
		}}

		// Verify Mode 0
		test.NodeOnMsgWithChildren(t, node0, msgList, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
			assert.Equal(t, "", msg.Metadata.GetValue("missingKey0"))
		})

		// Verify Mode 1
		test.NodeOnMsgWithChildren(t, node1, msgList, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
			// Data should remain unchanged
			assert.Equal(t, `{"original":"data"}`, msg.GetData())
		})
	})
}

func TestCacheSetNode(t *testing.T) {
	var targetNodeType = "cacheSet"

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"items": []CacheItem{
				{
					Level: CacheLevelChain,
					Key:   "testKey1",
					Value: "testValue1",
					Ttl:   "1h",
				},
				{
					Level: CacheLevelChain,
					Key:   "testKey2",
					Value: "testValue2",
					Ttl:   "1m",
				},
			},
		}, types.Configuration{
			"items": []CacheItem{
				{
					Level: CacheLevelChain,
					Key:   "testKey1",
					Value: "testValue1",
					Ttl:   "1h",
				},
				{
					Level: CacheLevelChain,
					Key:   "testKey2",
					Value: "testValue2",
					Ttl:   "1m",
				},
			},
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"items": []CacheItem{
				{Level: CacheLevelChain, Key: "key1", Value: "value1", Ttl: "1h"},
			},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"items": []map[string]interface{}{
				{"level": CacheLevelChain, "key": "testKey", "value": "testValue", "ttl": "1h"},
			},
		}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{{
			MsgType:    "TEST_MSG",
			Data:       "{}",
			AfterSleep: time.Millisecond * 200,
		}}

		test.NodeOnMsgWithChildren(t, node, msgList, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
		})
	})
}

func TestCacheDeleteNode(t *testing.T) {
	var targetNodeType = "cacheDelete"

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"keys": []LevelKey{{CacheLevelChain, "key1"}, {CacheLevelChain, "key2"}},
		}, types.Configuration{
			"keys": []LevelKey{{CacheLevelChain, "key1"}, {CacheLevelChain, "key2"}},
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"keys": []LevelKey{{CacheLevelChain, "key1"}},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"keys": []LevelKey{{CacheLevelChain, "testKey"}},
		}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{{
			MetaData:   types.NewMetadata(),
			MsgType:    "TEST_MSG",
			Data:       "{}",
			AfterSleep: time.Millisecond * 200,
		}}

		test.NodeOnMsgWithChildren(t, node, msgList, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
		})
	})
}
