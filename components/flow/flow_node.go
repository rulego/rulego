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

package flow

//子规则链节点，示例：
//{
//        "id": "s1",
//        "type": "flow",
//        "name": "子规则链",
//        "configuration": {
//			"targetId": "sub_chain_01",
//        }
//  }
import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
)

//注册节点
func init() {
	Registry.Add(&ChainNode{})
}

//ChainNodeConfiguration 节点配置
type ChainNodeConfiguration struct {
	//TargetId 子规则链ID
	TargetId string
}

//ChainNode 子规则链
//如果找不到规则链，并把消息通过`Failure`关系发送到下一个节点
//子规则链分支执行完后，如果成功把该分支链最后执行结果消息通过`Success`关系发送到下一个节点，否则通过`Failure`关系发送到下一个节点，如果子规则链触发多个分支，则会执行多次
type ChainNode struct {
	//节点配置
	Config ChainNodeConfiguration
}

//Type 组件类型
func (x *ChainNode) Type() string {
	return "flow"
}

func (x *ChainNode) New() types.Node {
	return &ChainNode{}
}

//Init 初始化
func (x *ChainNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, &x.Config)
}

//OnMsg 处理消息
func (x *ChainNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	ctx.TellFlow(msg, x.Config.TargetId, func(onEndMsg types.RuleMsg, err error) {
		if err == nil {
			ctx.TellSuccess(onEndMsg)
		} else {
			ctx.TellFailure(onEndMsg, err)
		}

	}, nil)
	return nil
}

//Destroy 销毁
func (x *ChainNode) Destroy() {
}
