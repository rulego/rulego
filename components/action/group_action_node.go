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
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

func init() {
	Registry.Add(&GroupActionNode{})
}

// GroupActionNodeConfiguration 节点配置
type GroupActionNodeConfiguration struct {
	//MatchRelationType 匹配组内节点关系类型，支持`Success`、`Failure`、`True`、`False`和自定义等关系,默认`Success`
	MatchRelationType string
	//MatchNum 匹配满足节点数量
	//默认0：代表组内所有节点都是 MatchRelationType 指定类型，发送到`Success`链，否则发送到`Failure`链。
	//MatchNum>0，则表示任意匹配 MatchNum 个节点是 MatchRelationType 指定类型，发送到`Success`链，否则发送到`Failure`链。
	//MatchNum>=len(NodeIds)则等价于MatchNum=0
	MatchNum int
	//NodeIds 组内节点ID列表，多个ID与`,`隔开，也可以使用[]string格式指定节点列表
	NodeIds interface{}
	//Timeout 执行超时，单位秒，默认0：代表不限制。
	Timeout int
}

// GroupActionNode 把多个节点组成一个分组,异步执行所有节点，等待所有节点执行完成后，把所有节点结果合并，发送到下一个节点
// 如果匹配到 Config.MatchNum 个节点是 Config.MatchRelationType 类型，则把数据到`Success`链, 否则发到`Failure`链。
// nodeIds为空或者执行超时，发送到`Failure`链
type GroupActionNode struct {
	//节点配置
	Config GroupActionNodeConfiguration
	//节点ID列表
	NodeIdList []string
	//节点列表长度
	Length int32
}

// Type 组件类型
func (x *GroupActionNode) Type() string {
	return "groupAction"
}

func (x *GroupActionNode) New() types.Node {
	return &GroupActionNode{Config: GroupActionNodeConfiguration{MatchRelationType: types.Success, MatchNum: 0}}
}

// Init 初始化
func (x *GroupActionNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
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
		if v := strings.TrimSpace(nodeId); v != "" {
			x.NodeIdList = append(x.NodeIdList, v)
		}
	}
	x.Config.MatchRelationType = strings.TrimSpace(x.Config.MatchRelationType)

	if x.Config.MatchRelationType == "" {
		x.Config.MatchRelationType = types.Success
	}
	if x.Config.MatchNum <= 0 || x.Config.MatchNum > len(x.NodeIdList) {
		x.Config.MatchNum = len(x.NodeIdList)
	}
	x.Length = int32(len(x.NodeIdList))
	return err
}

// OnMsg 处理消息
func (x *GroupActionNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.Length == 0 {
		ctx.TellFailure(msg, errors.New("nodeIds is empty"))
		return
	}
	//完成执行节点数量
	var endCount int32
	//匹配节点数量
	var currentMatchedCount int32
	//是否已经完成
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

	var wrapperMsg = msg.Copy()
	//每个节点执行结果列表
	var msgs = make([]types.WrapperMsg, len(x.NodeIdList))
	//保护msgs数组的互斥锁
	var msgsMutex sync.Mutex

	//执行节点列表逻辑
	for i, nodeId := range x.NodeIdList {
		//创建一个局部变量，避免闭包引用问题
		index := i
		ctx.TellNode(chanCtx, nodeId, msg, true, func(callbackCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			// 安全地写入msgs数组
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			selfId := callbackCtx.GetSelfId()

			msgsMutex.Lock()
			msgs[index] = types.WrapperMsg{
				Msg:    onEndMsg,
				Err:    errStr,
				NodeId: selfId,
			}
			msgsMutex.Unlock()

			// 直接使用原子操作获取当前计数，避免竞态窗口
			currentEndCount := atomic.AddInt32(&endCount, 1)
			var currentMatchCount int32
			if x.Config.MatchRelationType == relationType {
				currentMatchCount = atomic.AddInt32(&currentMatchedCount, 1)
			} else {
				currentMatchCount = atomic.LoadInt32(&currentMatchedCount)
			}

			// 判断是否应该结束并发送结果
			var shouldComplete bool
			var result bool

			// 如果已经达到匹配数量，立即返回成功
			if currentMatchCount >= int32(x.Config.MatchNum) {
				shouldComplete = true
				result = true
			} else if currentEndCount >= x.Length {
				// 所有节点都完成，但没有达到匹配数量，返回失败
				shouldComplete = true
				result = false
			}

			// 使用CAS确保只有一个goroutine能发送结果
			if shouldComplete && atomic.CompareAndSwapInt32(&completed, 0, 1) {
				// 安全地读取msgs数组进行处理
				msgsMutex.Lock()
				msgsCopy := make([]types.WrapperMsg, len(msgs))
				copy(msgsCopy, msgs)
				msgsMutex.Unlock()

				wrapperMsg.SetData(str.ToString(filterEmptyAndRemoveMeta(msgsCopy)))
				mergeMetadata(msgsCopy, &wrapperMsg)
				c <- result
			}
		}, nil)
	}

	// 等待执行结束或者超时
	select {
	case <-chanCtx.Done():
		ctx.TellFailure(wrapperMsg, chanCtx.Err())
	case r := <-c:
		if r {
			ctx.TellSuccess(wrapperMsg)
		} else {
			ctx.TellNext(wrapperMsg, types.Failure)
		}
	}
}

// Destroy 销毁
func (x *GroupActionNode) Destroy() {
}

// 过滤空值，返回新的数组
func filterEmptyAndRemoveMeta(msgs []types.WrapperMsg) []types.WrapperMsg {
	var result []types.WrapperMsg
	for _, msg := range msgs {
		if msg.NodeId != "" {
			if msg.Msg.Metadata != nil {
				msg.Msg.Metadata.Clear()
			}
			result = append(result, msg)
		}
	}
	return result
}

// 合并metadata - use zero-copy ForEach for better performance
func mergeMetadata(msgs []types.WrapperMsg, wrapperMsg *types.RuleMsg) {
	for _, msg := range msgs {
		if msg.NodeId != "" && msg.Err == "" {
			msg.Msg.Metadata.ForEach(func(k, v string) bool {
				wrapperMsg.Metadata.PutValue(k, v)
				return true // continue iteration
			})
		}
	}
}
