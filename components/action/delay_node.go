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
//          "periodInSeconds": 1,
//          "maxPendingMsgs": 1000
//        }
//  }
import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

var DelayNodeMsgType = "DELAY_NODE_MSG_TYPE"

// 注册节点
func init() {
	Registry.Add(&DelayNode{})
}

// DelayNodeConfiguration 节点配置
type DelayNodeConfiguration struct {
	//最大允许挂起消息的数量
	MaxPendingMsgs int
	//延迟时间，单位秒
	PeriodInSeconds int
	//通过 ${metadata.key} 从元数据变量中获取或者通过 ${msg.key} 从消息负荷中获取，延迟时间，如果该值有值，优先取该值。
	PeriodInSecondsPattern string
	//是否覆盖周期内的消息
	//true：周期内只保留一条消息，新的消息会覆盖之前的消息。直到队列里的消息被处理后，才会再次进入延迟队列。
	//false：周期内保留所有消息，直到达到最大挂起消息限制后，才会进入失败链路。
	Overwrite bool
}

// DelayNode 提供消息延迟能力的组件，支持静态和动态延迟时间
// DelayNode provides message delay capabilities with configurable timing and queue management.
//
// 核心算法：
// Core Algorithm:
// 1. 消息进入挂起队列，启动延迟定时器 - Messages enter pending queue with delay timer
// 2. 定时器到期后从队列移除并发送到Success链 - Timer expires, remove from queue and send to Success
// 3. 覆盖模式：同一时间只保留一条消息 - Overwrite mode: only keep one message at a time
// 4. 队列溢出时发送到Failure链 - Send to Failure on queue overflow
//
// 延迟机制 - Delay mechanisms:
//   - 静态延迟：periodInSeconds - Static delay: periodInSeconds
//   - 动态延迟：periodInSecondsPattern变量替换 - Dynamic delay: periodInSecondsPattern variable substitution
//
// 消息覆盖模式 - Message overwrite modes:
//   - overwrite=false: 队列所有消息 - Queue all messages
//   - overwrite=true: 用新消息替换挂起的消息 - Replace pending message with new one
type DelayNode struct {
	//节点配置
	Config DelayNodeConfiguration
	//消息队列
	PendingMsgs map[string]types.RuleMsg
	//上一条pending msg id
	LastPendingMsgId atomic.Value
	//锁
	mu sync.Mutex
}

// Type 组件类型
func (x *DelayNode) Type() string {
	return "delay"
}

func (x *DelayNode) New() types.Node {
	return &DelayNode{Config: DelayNodeConfiguration{PeriodInSeconds: 60, MaxPendingMsgs: 1000}}
}

// Init 初始化
func (x *DelayNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	x.PendingMsgs = make(map[string]types.RuleMsg)
	err := maps.Map2Struct(configuration, &x.Config)
	if x.Config.MaxPendingMsgs <= 0 {
		x.Config.MaxPendingMsgs = 1000
	}
	x.LastPendingMsgId.Store("")
	return err
}

// OnMsg 处理消息，实现延迟队列逻辑
func (x *DelayNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {

	if msg.Type == DelayNodeMsgType {
		x.mu.Lock()
		defer x.mu.Unlock()
		pendingMsg, ok := x.PendingMsgs[msg.Id]
		if ok {
			//清除周期内的消息
			if x.Config.Overwrite {
				x.LastPendingMsgId.Store("")
			}

			delete(x.PendingMsgs, msg.Id)
			ctx.TellSuccess(pendingMsg)
		} else {
			ctx.TellFailure(msg, fmt.Errorf("msg not found"))
		}

	} else if oldMsgId := x.LastPendingMsgId.Load().(string); oldMsgId != "" {
		//如果是覆盖模式，替换队列里的消息
		x.mu.Lock()
		defer x.mu.Unlock()
		x.PendingMsgs[oldMsgId] = msg
	} else {
		//获取队列长度
		x.mu.Lock()
		length := len(x.PendingMsgs)
		x.mu.Unlock()

		if length < x.Config.MaxPendingMsgs {
			periodInSeconds := x.Config.PeriodInSeconds
			//从变量中获取延迟时间
			if x.Config.PeriodInSecondsPattern != "" {
				evn := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
				if v, err := strconv.Atoi(str.ExecuteTemplate(x.Config.PeriodInSecondsPattern, evn)); err != nil {
					ctx.TellFailure(msg, err)
					return
				} else {
					periodInSeconds = v
				}
			}
			//如果是覆盖模式
			if x.Config.Overwrite {
				x.LastPendingMsgId.Store(msg.Id)
			}
			x.mu.Lock()
			x.PendingMsgs[msg.Id] = msg
			x.mu.Unlock()

			ackMsg := msg.Copy()
			ackMsg.Type = DelayNodeMsgType
			ctx.TellSelf(ackMsg, int64(periodInSeconds*1000))
		} else {
			ctx.TellFailure(msg, fmt.Errorf("max limit of pending messages"))
		}
	}

}

// Destroy 销毁
func (x *DelayNode) Destroy() {
}
