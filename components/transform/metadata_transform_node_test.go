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

func TestMetadataTransformNode(t *testing.T) {
	var targetNodeType = "metadataTransform"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &MetadataTransformNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
		}, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"temperature": "msg.temperature",
			},
		}, types.Configuration{
			"mapping": map[string]string{
				"temperature": "msg.temperature",
			},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
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
		var nodeList = []test.NodeAndCallback{
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"mapping": map[string]string{
						"id":           "id",
						"ts":           "ts",
						"dataType":     "dataType",
						"type":         "type",
						"name":         "upper(msg.name)",
						"tmp":          "msg.temperature",
						"alarm":        "msg.temperature>50",
						"test01":       "'test01'",
						"temperature2": "msg.temperature2",
					},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "226a05f1-9464-43b6-881e-b1629f1b030d", msg.Metadata.GetValue("id"))
					assert.Equal(t, "AA", msg.Metadata.GetValue("name"))
					assert.Equal(t, "test01", msg.Metadata.GetValue("test01"))
					assert.Equal(t, "", msg.Metadata.GetValue("temperature2"))
					assert.Equal(t, "test", msg.Metadata.GetValue("productType"))
				},
			},
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"isNew": true,
					"mapping": map[string]string{
						"id":       "id",
						"ts":       "ts",
						"dataType": "dataType",
						"type":     "type",
						"name":     "upper(msg.name)",
						"tmp":      "msg.temperature",
						"alarm":    "msg.temperature>50",
					},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "226a05f1-9464-43b6-881e-b1629f1b030d", msg.Metadata.GetValue("id"))
					assert.Equal(t, "AA", msg.Metadata.GetValue("name"))
					assert.Equal(t, "", msg.Metadata.GetValue("productType"))
				},
			},
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"isNew": true,
					"mapping": map[string]string{
						"id": "xx+1",
					},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)
	})
}
