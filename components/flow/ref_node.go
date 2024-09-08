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

//节点复用节点，示例：
//{
//        "id": "s1",
//        "type": "ref",
//        "name": "节点复用",
//        "configuration": {
//			"targetId": "chain_01:node",
//        }
//  }
import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"strings"
)

// 注册节点
func init() {
	Registry.Add(&RefNode{})
}

// RefNodeConfiguration 节点配置
type RefNodeConfiguration struct {
	//TargetId 节点ID，
	TargetId string
}

// RefNode 引用指定规则链或者当前规则链节点，用于节点复用
// 格式：[{chainId}]:{nodeId}，如果是引入本规则链，则格式为：{nodeId}
// 执行成功则使用该节点的输出关系发送到下一个节点
// 如果找不到节点，则把消息通过`Failure`关系发送到下一个节点
type RefNode struct {
	//节点配置
	Config  RefNodeConfiguration
	chainId string
	nodeId  string
}

// Type 组件类型
func (x *RefNode) Type() string {
	return "ref"
}

func (x *RefNode) New() types.Node {
	return &RefNode{}
}

// Init 初始化
func (x *RefNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	values := strings.Split(x.Config.TargetId, ":")
	if len(values) == 0 {
		return errors.New("targetId is empty")
	} else if len(values) == 1 {
		x.nodeId = values[0]
	} else {
		x.chainId = values[0]
		x.nodeId = values[1]
	}
	return err
}

// OnMsg 处理消息
func (x *RefNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellChainNode(ctx.GetContext(), x.chainId, x.nodeId, msg, true, func(newCtx types.RuleContext, newMsg types.RuleMsg, err error, relationType string) {
		if err != nil {
			ctx.TellFailure(msg, err)
		} else {
			ctx.TellNext(newMsg, relationType)
		}
	}, nil)
}

// Destroy 销毁
func (x *RefNode) Destroy() {
}
