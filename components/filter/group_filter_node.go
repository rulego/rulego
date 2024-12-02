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

package filter

import (
	"context"
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strings"
	"sync/atomic"
	"time"
)

func init() {
	Registry.Add(&GroupFilterNode{})
}

// GroupFilterNodeConfiguration 节点配置
type GroupFilterNodeConfiguration struct {
	//AllMatches 是否要求所有节点都匹配才发送到True链，如果为false，则只要有任何一个节点匹配就发送到True链
	AllMatches bool
	//NodeIds 组内节点ID列表，多个ID与`,`隔开，也可以使用[]string格式指定节点列表
	NodeIds interface{}
	//Timeout 执行超时，单位秒，默认0：代表不限制。
	Timeout int
}

// GroupFilterNode 把多个filter节点组成一个分组，
// 如果所有节点都是True，则把数据到`True`链, 否则发到`False`链。
// AllMatches=false，则只要有任何一个节点返回是True，则发送到`True`链
// nodeIds为空或者执行超时，发送到`Failure`链
type GroupFilterNode struct {
	//节点配置
	Config     GroupFilterNodeConfiguration
	NodeIdList []string
	Length     int32
}

// Type 组件类型
func (x *GroupFilterNode) Type() string {
	return "groupFilter"
}

func (x *GroupFilterNode) New() types.Node {
	return &GroupFilterNode{Config: GroupFilterNodeConfiguration{AllMatches: false}}
}

// Init 初始化
func (x *GroupFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	var nodeIds []string
	if v, ok := x.Config.NodeIds.(string); ok {
		nodeIds = strings.Split(v, ",")
	} else if v, ok := x.Config.NodeIds.([]string); ok {
		nodeIds = v
	} else if v, ok := x.Config.NodeIds.([]interface{}); ok {
		for _, item := range v {
			nodeIds = append(nodeIds, str.ToString(item))
		}
	}
	for _, nodeId := range nodeIds {
		if v := strings.Trim(nodeId, ""); v != "" {
			x.NodeIdList = append(x.NodeIdList, v)
		}
	}
	x.Length = int32(len(x.NodeIdList))
	return err
}

// OnMsg 处理消息
func (x *GroupFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.Length == 0 {
		ctx.TellFailure(msg, errors.New("nodeIds is empty"))
		return
	}
	var endCount int32
	var completed int32
	c := make(chan bool, 1)
	var chanCtx context.Context
	var cancel context.CancelFunc
	if x.Config.Timeout > 0 {
		chanCtx, cancel = context.WithTimeout(ctx.GetContext(), time.Duration(x.Config.Timeout)*time.Second)
	} else {
		chanCtx, cancel = context.WithCancel(ctx.GetContext())
	}

	defer cancel()

	//执行节点列表逻辑
	for _, nodeId := range x.NodeIdList {
		ctx.TellNode(chanCtx, nodeId, msg, true, func(callbackCtx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			if atomic.LoadInt32(&completed) == 0 {
				firstRelationType := relationType
				atomic.AddInt32(&endCount, 1)

				if x.Config.AllMatches {
					if firstRelationType != types.True && atomic.CompareAndSwapInt32(&completed, 0, 1) {
						c <- false
					} else if atomic.LoadInt32(&endCount) >= x.Length && atomic.CompareAndSwapInt32(&completed, 0, 1) {
						c <- true
					}
				} else if !x.Config.AllMatches {
					if firstRelationType == types.True && atomic.CompareAndSwapInt32(&completed, 0, 1) {
						c <- true
					} else if atomic.LoadInt32(&endCount) >= x.Length && atomic.CompareAndSwapInt32(&completed, 0, 1) {
						c <- false
					}
				}
			}

		}, nil)
	}

	// 等待执行结束或者超时
	select {
	case <-chanCtx.Done():
		ctx.TellFailure(msg, chanCtx.Err())
	case r := <-c:
		if r {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Destroy 销毁
func (x *GroupFilterNode) Destroy() {
}
