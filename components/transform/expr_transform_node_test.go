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

package transform

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestExprTransformNode(t *testing.T) {
	var targetNodeType = "exprTransform"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ExprTransformNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
			"expr": "msg",
		}, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
			"expr": "msg",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"id":          "id",
				"ts":          "ts",
				"dataType":    "dataType",
				"type":        "type",
				"name":        "upper(msg.name)",
				"tmp":         "msg.temperature",
				"alarm":       "msg.temperature>50",
				"productType": "metadata.productType",
			},
		}, Registry)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg.name)",
		}, Registry)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg[:1])",
		}, Registry)
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg[:1])",
			"mapping": map[string]string{
				"name":        "upper(msg.name)",
				"tmp":         "msg.temperature",
				"alarm":       "msg.temperature>50",
				"productType": "metadata.productType",
			},
		}, Registry)

		node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "replace(toJSON(msg),'name','productName')",
		}, Registry)

		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.aa+'xx'",
		}, Registry)

		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.aa;",
		}, Registry)
		assert.NotNil(t, err)
		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name": "msg.aa;",
			},
		}, Registry)
		assert.NotNil(t, err)
		node7, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name": "msg.aa+1",
			},
		}, Registry)
		node8, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		msg := test.Msg{
			Id:         "226a05f1-9464-43b6-881e-b1629f1b030d",
			Ts:         1719024872741,
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg1 := test.Msg{
			MetaData:   metaData,
			DataType:   types.TEXT,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "aa",
			AfterSleep: time.Millisecond * 200,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, types.JSON, msg.DataType)
					assert.Equal(t, "{\"alarm\":true,\"dataType\":\"JSON\",\"id\":\"226a05f1-9464-43b6-881e-b1629f1b030d\",\"name\":\"AA\",\"productType\":\"test\",\"tmp\":60,\"ts\":1719024872741,\"type\":\"ACTIVITY_EVENT\"}", msg.Data)
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "AA", msg.Data)
				},
			},
			{
				Node:    node3,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "A", msg.Data)
				},
			},
			{
				Node:    node4,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "A", msg.Data)
				},
			},
			{
				Node:    node5,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "{\n  \"humidity\": 30,\n  \"productName\": \"aa\",\n  \"temperature\": 60\n}", msg.Data)
				},
			},
			{
				Node:    node6,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node7,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node8,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "{}", msg.Data)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)
	})
}
