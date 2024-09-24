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

// Example of rule chain node configuration:
//{
//	"id": "s1",
//	"type": "join",
//	"name": "join",
//	"configuration": {
//	 }
//	}
//}
import (
	"context"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"time"
)

func init() {
	Registry.Add(&JoinNode{})
}

type JoinNodeConfiguration struct {
	//Timeout 执行超时，单位秒，默认0：代表不限制。
	Timeout int
}

// JoinNode 合并多个异步节点执行结果
type JoinNode struct {
	//节点配置
	Config JoinNodeConfiguration
}

// Type 组件类型
func (x *JoinNode) Type() string {
	return "join"
}

func (x *JoinNode) New() types.Node {
	return &JoinNode{Config: JoinNodeConfiguration{}}
}

// Init 初始化
func (x *JoinNode) Init(_ types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, &x.Config)
}

// OnMsg processes the message.
func (x *JoinNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	c := make(chan bool, 1)
	var chanCtx context.Context
	var cancel context.CancelFunc
	if x.Config.Timeout > 0 {
		chanCtx, cancel = context.WithTimeout(ctx.GetContext(), time.Duration(x.Config.Timeout)*time.Second)
	} else {
		chanCtx, cancel = context.WithCancel(ctx.GetContext())
	}

	defer cancel()

	var wrapperMsg = msg.Copy()

	ok := ctx.TellCollect(msg, func(msgList []types.WrapperMsg) {
		wrapperMsg.DataType = types.JSON
		wrapperMsg.Data = str.ToString(filterEmptyAndRemoveMeta(msgList))
		mergeMetadata(msgList, &wrapperMsg)
		c <- true
	})
	if ok {
		// 等待执行结束或者超时
		select {
		case <-chanCtx.Done():
			ctx.TellFailure(wrapperMsg, chanCtx.Err())
		case _ = <-c:
			ctx.TellSuccess(wrapperMsg)
		}
	}
}

func (x *JoinNode) Destroy() {
}
