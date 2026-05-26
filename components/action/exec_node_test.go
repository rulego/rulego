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

func TestTemplateNode(t *testing.T) {
	var targetNodeType = "exec"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ExecCommandNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"cmd":  "ls",
			"args": []string{"./data"},
		}, types.Configuration{
			"cmd":  "ls",
			"args": []string{"./data"},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue("dir", ".")
		msg := test.Msg{
			Id:         "226a05f1-9464-43b6-881e-b1629f1b030d",
			Ts:         1719024872741,
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}

		count := int32(0)
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeWhitelist, "ls,cd")
		config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Log, flowType)
			assert.Equal(t, "info", relationType)
			atomic.AddInt32(&count, 1)
		}
		var data1, data2 string
		var data1Mutex, data2Mutex sync.Mutex
		var nodeList = []test.NodeAndCallback{
			{
				Node: test.InitNodeByConfig(types.NewConfig(), targetNodeType, types.Configuration{
					"cmd":  "ls",
					"args": []string{"."},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, ErrCmdNotAllowed.Error(), err.Error())
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"xx"},
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":  "ls",
					"args": []string{"."},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"${dir}"},
					"log":         true,
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.NotEqual(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"${dir} -l"},
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.NotEqual(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
					data1Mutex.Lock()
					data1 = msg.GetData()
					data1Mutex.Unlock()
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"${dir}", "-l"},
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.NotEqual(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
					data2Mutex.Lock()
					data2 = msg.GetData()
					data2Mutex.Unlock()
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildrenAndConfig(t, config, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Second)
		assert.Equal(t, int32(1), atomic.LoadInt32(&count))
		data1Mutex.Lock()
		data2Mutex.Lock()
		assert.Equal(t, data1, data2)
		data2Mutex.Unlock()
		data1Mutex.Unlock()
	})
}

func TestExecNodeAllowMode(t *testing.T) {
	var targetNodeType = "exec"

	t.Run("AllowMode-WhitelistReject", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeAllow))
		config.Properties.PutValue(KeyExecNodeWhitelist, "echo,date")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd": "rm",
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Failure, relationType)
			assert.Equal(t, ErrCmdNotAllowed.Error(), err.Error())
		})
	})

	t.Run("AllowMode-WhitelistPass", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeAllow))
		config.Properties.PutValue(KeyExecNodeWhitelist, "echo,date")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd":         "echo",
			"args":        []string{"hello"},
			"replaceData": true,
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
		})
	})
}

func TestExecNodeDenyMode(t *testing.T) {
	var targetNodeType = "exec"

	t.Run("DenyMode-AllowAll", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeDeny))

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd":         "echo",
			"args":        []string{"hello"},
			"replaceData": true,
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
		})
	})

	t.Run("DenyMode-BlockDeniedCommand", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeDeny))
		config.Properties.PutValue(KeyExecNodeDeny, "rm,format")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd": "rm",
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Failure, relationType)
			assert.Equal(t, ErrCmdDenied.Error(), err.Error())
		})
	})

	t.Run("DenyMode-AllowNonDeniedCommand", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeDeny))
		config.Properties.PutValue(KeyExecNodeDeny, "rm,format")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd":         "echo",
			"args":        []string{"hello"},
			"replaceData": true,
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
		})
	})
}

func TestExecNodeDenyArgs(t *testing.T) {
	var targetNodeType = "exec"

	t.Run("DenyArgs-BlockDangerousArg", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeDeny))
		config.Properties.PutValue(KeyExecNodeDenyArgs, "-rf /,--no-preserve-root")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd":  "rm",
			"args": []string{"-rf /"},
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Failure, relationType)
			assert.Equal(t, ErrCmdDenied.Error(), err.Error())
		})
	})

	t.Run("DenyArgs-AllowSafeArg", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeDeny))
		config.Properties.PutValue(KeyExecNodeDenyArgs, "-rf /,--no-preserve-root")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd":         "echo",
			"args":        []string{"hello"},
			"replaceData": true,
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
		})
	})
}

func TestExecNodeDenyOverridesAllow(t *testing.T) {
	var targetNodeType = "exec"

	t.Run("DenyAlwaysApplies-EvenInAllowMode", func(t *testing.T) {
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeMode, string(ModeAllow))
		config.Properties.PutValue(KeyExecNodeWhitelist, "rm,echo")
		config.Properties.PutValue(KeyExecNodeDeny, "rm")

		msg := test.Msg{
			MetaData:   types.BuildMetadata(make(map[string]string)),
			MsgType:    "TEST",
			Data:       "{}",
			AfterSleep: time.Millisecond * 100,
		}

		node := test.InitNodeByConfig(config, targetNodeType, types.Configuration{
			"cmd": "rm",
		}, Registry)

		test.NodeOnMsgWithChildrenAndConfig(t, config, node, []test.Msg{msg}, nil, func(msg types.RuleMsg, relationType string, err error) {
			// rm is in both whitelist and deny list, deny takes priority
			assert.Equal(t, types.Failure, relationType)
			assert.Equal(t, ErrCmdDenied.Error(), err.Error())
		})
	})
}

func TestSplitAndFilter(t *testing.T) {
	assert.Nil(t, splitAndFilter(""))
	assert.Nil(t, splitAndFilter(",,,"))
	assert.Equal(t, []string{"a", "b", "c"}, splitAndFilter("a,b,c"))
	assert.Equal(t, []string{"a", "b", "c"}, splitAndFilter(" a , b , c "))
}
