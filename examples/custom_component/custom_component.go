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

package main

import (
	"context"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"strings"
	"time"
)

var (
	shareKey      = "shareKey"
	shareValue    = "shareValue"
	addShareKey   = "addShareKey"
	addShareValue = "addShareValue"
)

//演示自定义组件
func main() {

	//注册自定义组件
	rulego.Registry.Register(&UpperNode{})
	rulego.Registry.Register(&TimeNode{})

	config := rulego.NewConfig()
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsgWithOptions(msg, types.WithContext(context.WithValue(context.Background(), shareKey, shareValue)), types.WithEndFunc(func(msg types.RuleMsg, err error) {
		fmt.Println(msg)
	}))
	time.Sleep(time.Second * 1)
}

var chainJsonFile = `
{
  "ruleChain": {
    "name": "测试数据共享规则链"
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "test/upper",
        "name": "test upper",
        "debugMode": false
      },
      {
        "id":"s2",
        "type": "test/time",
        "name": "add time",
        "debugMode": false
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "Success"
      }
    ],
    "ruleChainConnections": null
  }
}
`

//UpperNode A plugin that converts the message data to uppercase
type UpperNode struct{}

func (n *UpperNode) Type() string {
	return "test/upper"
}
func (n *UpperNode) New() types.Node {
	return &UpperNode{}
}
func (n *UpperNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *UpperNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	msg.Data = strings.ToUpper(msg.Data)
	v := ctx.GetContext().Value(shareKey)
	if v != nil {
		msg.Metadata.PutValue(shareKey, v.(string))
	}
	//增加新的共享数据
	modifyCtx := context.WithValue(ctx.GetContext(), addShareKey, addShareValue)
	ctx.SetContext(modifyCtx)
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
	return nil
}

func (n *UpperNode) Destroy() {
	// Do some cleanup work
}

//TimeNode A plugin that adds a timestamp to the message metadata
type TimeNode struct{}

func (n *TimeNode) Type() string {
	return "test/time"
}

func (n *TimeNode) New() types.Node {
	return &TimeNode{}
}

func (n *TimeNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *TimeNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	msg.Metadata.PutValue("timestamp", time.Now().Format(time.RFC3339))
	v1 := ctx.GetContext().Value(shareKey)
	if v1 != nil {
		msg.Metadata.PutValue(shareKey, v1.(string))
	}
	v2 := ctx.GetContext().Value(addShareKey)
	if v2 != nil {
		msg.Metadata.PutValue(addShareKey, v2.(string))
	}
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
	return nil
}

func (n *TimeNode) Destroy() {
	// Do some cleanup work
}
