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
	"strings"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
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
	var trueCount int32 // 新增：跟踪True结果数量
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
			// 直接使用原子操作获取当前计数，避免竞态窗口
			currentEndCount := atomic.AddInt32(&endCount, 1)
			var currentTrueCount int32
			if relationType == types.True {
				currentTrueCount = atomic.AddInt32(&trueCount, 1)
			} else {
				currentTrueCount = atomic.LoadInt32(&trueCount)
			}

			// 判断是否应该结束并发送结果
			var shouldComplete bool
			var result bool

			if x.Config.AllMatches {
				// AllMatches=true: 有任何False就立即返回False，所有都是True才返回True
				if relationType != types.True {
					shouldComplete = true
					result = false
				} else if currentEndCount >= x.Length && currentTrueCount >= x.Length {
					shouldComplete = true
					result = true
				}
			} else {
				// AllMatches=false: 有任何True就立即返回True，所有都完成且无True才返回False
				if relationType == types.True {
					shouldComplete = true
					result = true
				} else if currentEndCount >= x.Length && currentTrueCount == 0 {
					shouldComplete = true
					result = false
				}
			}

			// 使用CAS确保只有一个goroutine能发送结果
			if shouldComplete && atomic.CompareAndSwapInt32(&completed, 0, 1) {
				c <- result
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
