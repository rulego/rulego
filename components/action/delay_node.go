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

//规则链节点配置示例：
//{
//        "id": "s1",
//        "type": "delay",
//        "name": "延迟节点",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "return 'Incoming message:\\n' + JSON.stringify(msg) + '\\nIncoming metadata:\\n' + JSON.stringify(metadata);"
//        }
//  }
import (
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strconv"
	"sync"
)

var DelayNodeMsgType = "DELAY_NODE_MSG_TYPE"

//注册节点
func init() {
	Registry.Add(&DelayNode{})
}

//DelayNodeConfiguration 节点配置
type DelayNodeConfiguration struct {
	//延迟时间，单位秒
	PeriodInSeconds int
	//最大允许挂起消息的数量
	MaxPendingMsgs int
	//通过${metadataKey}方式从metadata变量中获取，延迟时间，如果该值有值，优先取该值。
	PeriodInSecondsPattern string
}

//DelayNode
//当特定传入消息的延迟期达到后，该消息将从挂起队列中删除，并通过成功链路(`Success`)路由到下一个节点。
//如果已经达到了最大挂起消息限制，则每个下一条消息都会通过失败链路(`Failure`)路由。

type DelayNode struct {
	//节点配置
	Config DelayNodeConfiguration
	//消息队列
	PendingMsgs map[string]types.RuleMsg
	mu          sync.Mutex
}

//Type 组件类型
func (x *DelayNode) Type() string {
	return "delay"
}

func (x *DelayNode) New() types.Node {
	return &DelayNode{Config: DelayNodeConfiguration{PeriodInSeconds: 60, MaxPendingMsgs: 1000}}
}

//Init 初始化
func (x *DelayNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	x.PendingMsgs = make(map[string]types.RuleMsg)
	return maps.Map2Struct(configuration, &x.Config)
}

//OnMsg 处理消息
func (x *DelayNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {

	if msg.Type == DelayNodeMsgType {
		x.mu.Lock()
		defer x.mu.Unlock()
		pendingMsg, ok := x.PendingMsgs[msg.Id]
		if ok {
			delete(x.PendingMsgs, msg.Id)
			ctx.TellSuccess(pendingMsg)
		} else {
			ctx.TellFailure(msg, fmt.Errorf("msg not found"))
		}

	} else {
		//获取队列长度
		x.mu.Lock()
		length := len(x.PendingMsgs)
		x.mu.Unlock()

		if length < x.Config.MaxPendingMsgs {
			periodInSeconds := x.Config.PeriodInSeconds
			//从Metadata获取延迟时间
			if x.Config.PeriodInSecondsPattern != "" {
				if v, err := strconv.Atoi(str.SprintfDict(x.Config.PeriodInSecondsPattern, msg.Metadata.Values())); err != nil {
					ctx.TellFailure(msg, err)
					return err
				} else {
					periodInSeconds = v
				}
			}

			x.mu.Lock()
			x.PendingMsgs[msg.Id] = msg
			defer x.mu.Unlock()

			ackMsg := msg.Copy()
			ackMsg.Type = DelayNodeMsgType
			ctx.TellSelf(ackMsg, int64(periodInSeconds*1000))
		} else {
			ctx.TellFailure(msg, fmt.Errorf("max limit of pending messages"))
		}

	}

	return nil
}

//Destroy 销毁
func (x *DelayNode) Destroy() {
	x.PendingMsgs = make(map[string]types.RuleMsg)
}
