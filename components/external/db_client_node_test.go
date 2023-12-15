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

package external

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"net"
	"testing"
	"time"
)

func TestDbClientNode(t *testing.T) {
	var targetNodeType = "dbClient"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &DbClientNode{}, types.Configuration{
			"sql":        "select * from test",
			"driverName": "mysql",
			"dsn":        "root:root@tcp(127.0.0.1:3306)/test",
		}, Registry)
	})
	t.Run("defaultConfig", func(t *testing.T) {
		node := &DbClientNode{}
		err := node.Init(types.NewConfig(), types.Configuration{})
		assert.NotNil(t, err)
		assert.Equal(t, "mysql", node.Config.DriverName)

		node2 := &DbClientNode{}
		err = node2.Init(types.NewConfig(), types.Configuration{"sql": "xx"})
		assert.Equal(t, "unsupported sql statement: xx", err.Error())
	})

	t.Run("OnMsgMysql", func(t *testing.T) {
		testDbClientNodeOnMsg(t, targetNodeType, "mysql", "root:root@tcp(127.0.1.1:3306)/test")
	})
	t.Run("OnMsgPostgres", func(t *testing.T) {
		testDbClientNodeOnMsg(t, targetNodeType, "postgres", "postgres://postgres:postgres@127.0.1.1:5432/test?sslmode=disable")
	})
}

func testDbClientNodeOnMsg(t *testing.T, targetNodeType, driverName, dsn string) {

	node1 := &DbClientNode{}
	err := node1.Init(types.NewConfig(), types.Configuration{})
	assert.NotNil(t, err)
	assert.Equal(t, "mysql", node1.Config.DriverName)

	node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "insert into users (id,name, age) values (?,?,?)",
		"params":     []interface{}{"${id}", "${name}", 18},
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "update users set age = ? where id = ?",
		"params":     []interface{}{"${age}", "${id}"},
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "select id,name,age from users",
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "select * from users where id = ?",
		"params":     []interface{}{"${id}"},
		"getOne":     true,
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)
	node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "delete from users",
		"params":     nil,
		"poolSize":   10,
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("id", "1")
	metaData.PutValue("name", "test01")
	metaData.PutValue("age", "18")

	updateMetaData := types.BuildMetadata(make(map[string]string))
	updateMetaData.PutValue("id", "1")
	updateMetaData.PutValue("name", "test01")
	updateMetaData.PutValue("age", "21")

	msgList := []test.Msg{
		{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT2",
			Data:       "{\"temperature\":60}",
			AfterSleep: time.Millisecond * 200,
		},
	}

	var nodeList = []test.NodeAndCallback{
		{
			Node:    node1,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				assert.Equal(t, types.Failure, relationType)
				assert.Equal(t, "unsupported sql statement: ", err.Error())
			},
		},
		{
			Node:    node2,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
				}
			},
		},
		{
			Node: node3,
			MsgList: []test.Msg{
				{
					MetaData:   updateMetaData,
					MsgType:    "ACTIVITY_EVENT2",
					Data:       "{\"temperature\":60}",
					AfterSleep: time.Millisecond * 200,
				},
			},
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
				}
			},
		},
		{
			Node:    node4,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "[{\"age\":\"21\",\"id\":\"1\",\"name\":\"test01\"}]", msg.Data)
				}
			},
		},
		{
			Node:    node5,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "{\"age\":21,\"id\":1,\"name\":\"test01\"}", msg.Data)
				}
			},
		},
		{
			Node:    node6,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
				}
			},
		},
	}
	for _, item := range nodeList {
		test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
	}
}
