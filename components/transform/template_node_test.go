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

func TestTemplateNode(t *testing.T) {
	var targetNodeType = "text/template"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &TemplateNode{}, types.Configuration{
			"template": `"id": "{{ .id}}"
"ts": "{{ .ts}}"
"type": "{{ .type}}"
"msgType": "{{ .msgType}}"
"data": "{{ .data | escape}}"
"dataType": "{{ .dataType}}"
`,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"template": "type:{{ .type}}",
		}, types.Configuration{
			"template": "type:{{ .type}}",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1 := test.InitNode(targetNodeType, types.Configuration{
			"template": `
						id:{{ .id}}
						ts:{{ .ts}}
						type:{{ .type}}
						msgType:{{ .msgType}}
						data:{{ .data}}
						msg.name:{{ .msg.name}}
						dataType:{{ .dataType}}
						productType:{{ .metadata.productType}}
						nameList:
						{{- range .msg.nameList }}
						  - {{ . }}
						{{- end }}
				`,
		}, Registry)

		node2 := test.InitNode(targetNodeType, types.Configuration{
			"template": "file:../../testdata/template/template_node_test.tpl",
		}, Registry)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msg := test.Msg{
			Id:         "226a05f1-9464-43b6-881e-b1629f1b030d",
			Ts:         1719024872741,
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"nameList\":[\"aa\",\"bb\"],\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		result := `
						id:226a05f1-9464-43b6-881e-b1629f1b030d
						ts:1719024872741
						type:ACTIVITY_EVENT
						msgType:ACTIVITY_EVENT
						data:{"name":"aa","nameList":["aa","bb"],"temperature":60,"humidity":30}
						msg.name:aa
						dataType:JSON
						productType:test
						nameList:
						  - aa
						  - bb
						`
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.EqualCleanString(t, result, msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.EqualCleanString(t, result, msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"template": "data:\"{{.data | escape}}\"",
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.EqualCleanString(t, "data:\"{\\\"name\\\":\\\"aa\\\",\\\"nameList\\\":[\\\"aa\\\",\\\"bb\\\"],\\\"temperature\\\":60,\\\"humidity\\\":30}\"", msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
	})
}
