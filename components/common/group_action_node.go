/*
 * Copyright 2025 The RuleGo Authors.
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

package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init 注册GroupActionNode组件
// init registers the GroupActionNode component with the default registry.
func init() {
	Registry.Add(&GroupActionNode{})
}

// GroupActionNodeConfiguration GroupActionNode配置结构
// GroupActionNodeConfiguration defines the configuration structure for the GroupActionNode component.
type GroupActionNodeConfiguration struct {
	// MatchRelationType 指定组内要匹配的关系类型，支持Success、Failure、True、False和自定义关系
	// MatchRelationType specifies the relation type to match within the group.
	// Supports 'Success', 'Failure', 'True', 'False', and custom relations.
	MatchRelationType string

	// MatchNum 指定必须匹配MatchRelationType的节点数量
	// MatchNum specifies the number of nodes that must match the MatchRelationType.
	// Default 0: All nodes in the group must be of MatchRelationType to send to Success chain, otherwise to Failure chain.
	// MatchNum > 0: Any MatchNum nodes must be of MatchRelationType to send to Success chain, otherwise to Failure chain.
	MatchNum int

	// NodeIds 指定组内节点ID列表，可以是逗号分隔的字符串或[]string格式
	// NodeIds specifies the list of node IDs in the group.
	// Can be a comma-separated string or a []string format to specify the node list.
	NodeIds interface{}

	// Timeout 指定执行超时时间（秒），默认0表示无时间限制
	// Timeout specifies the execution timeout in seconds.
	// Default 0 means no time limit.
	Timeout int

	// MergeToMap 如果true，如果是json类型 则把所有节点的输出data合并到同一个map
	// MergeToMap if true, and if the data type is JSON, merges the output data of all nodes into the same map.
	MergeToMap bool
}

// GroupActionNode 将多个节点分组并异步执行的动作组件
// GroupActionNode is an action component that groups multiple nodes and executes them asynchronously.
//
// 核心算法：
// Core Algorithm:
// 1. 并发执行组内所有节点 - Execute all nodes in group concurrently
// 2. 收集执行结果并计数匹配的关系类型 - Collect results and count matching relation types
// 3. 根据MatchNum判断成功条件 - Determine success based on MatchNum criteria
// 4. 合并结果并路由到Success或Failure链 - Merge results and route to Success or Failure
//
// 匹配逻辑 - Matching logic:
//   - MatchNum=0: 所有节点都必须匹配 - All nodes must match
//   - MatchNum>0: 至少MatchNum个节点匹配 - At least MatchNum nodes must match
//
// 超时保护 - Timeout protection:
//   - 配置超时防止无限等待 - Configured timeout prevents indefinite waiting
//   - 早期终止当满足匹配条件时 - Early termination when match criteria satisfied
type GroupActionNode struct {
	// Config 节点配置
	// Config holds the node configuration including matching criteria and timeout
	Config GroupActionNodeConfiguration

	// NodeIdList 要执行的节点ID列表
	// NodeIdList contains the parsed list of node IDs to execute
	NodeIdList []string

	// Length 组中节点数量
	// Length stores the number of nodes in the group for efficient access
	Length int32
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *GroupActionNode) Type() string {
	return "groupAction"
}

// New 创建新实例
// New creates a new instance.
func (x *GroupActionNode) New() types.Node {
	return &GroupActionNode{Config: GroupActionNodeConfiguration{MatchRelationType: types.Success, MatchNum: 0}}
}

// Init 初始化组件
// Init initializes the component.
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

// OnMsg 处理消息，并发执行节点组并根据匹配条件确定成功
// OnMsg processes incoming messages by executing the configured group of nodes in parallel.
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
		index := i
		ctx.TellNode(chanCtx, nodeId, msg.Copy(), true, func(callbackCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			// 检查context是否已被取消，避免无意义的计算
			select {
			case <-chanCtx.Done():
				return // 提前退出，避免资源浪费
			default:
			}

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

				if x.Config.MergeToMap {
					wrapperMsg.SetDataType(types.JSON)
					mergedMap := make(map[string]interface{})
					for _, val := range msgsCopy {
						if val.NodeId != "" {
							// 根据数据类型进行不同的处理
							switch val.Msg.DataType {
							case types.JSON:
								if dataMap, err := val.Msg.GetJsonData(); err == nil {
									if m, ok := dataMap.(map[string]interface{}); ok {
										for k, v := range m {
											mergedMap[k] = v
										}
									} else {
										mergedMap[val.NodeId] = dataMap
									}
								} else {
									mergedMap[val.NodeId] = val.Msg.GetData()
								}
							default:
								mergedMap[val.NodeId] = val.Msg.GetData()
							}
						}
					}
					wrapperMsg.SetData(str.ToString(mergedMap))
				} else {
					wrapperMsg.SetData(str.ToString(filterEmptyAndRemoveMeta(msgsCopy)))
				}
				_ = mergeMetadata(msgsCopy, &wrapperMsg)

				// 使用非阻塞发送，防止在超时情况下channel阻塞
				select {
				case c <- result:
					// 发送成功
				default:
					// Channel已满或无接收者（可能主函数已超时退出），放弃发送
				}
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

// Destroy 清理资源
// Destroy cleans up resources.
func (x *GroupActionNode) Destroy() {
	// 无资源需要清理
	// No resources to clean up
}

// filterEmptyAndRemoveMeta 过滤空消息并清除元数据
// filterEmptyAndRemoveMeta filters out empty messages and removes metadata for cleaner output.
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

// mergeMetadata 合并成功执行的元数据到包装消息中
// mergeMetadata merges metadata from successful group executions into the wrapper message.
func mergeMetadata(msgs []types.WrapperMsg, wrapperMsg *types.RuleMsg) error {
	var errStr string
	for _, msg := range msgs {
		if msg.NodeId != "" && msg.Err == "" {
			msg.Msg.Metadata.ForEach(func(k, v string) bool {
				wrapperMsg.Metadata.PutValue(k, v)
				return true // continue iteration
			})
		} else if msg.Err != "" {
			errStr += fmt.Sprintf("NodeId=%s,Err=%s ", msg.NodeId, msg.Err)
		}
	}
	if errStr != "" {
		return errors.New(errStr)
	} else {
		return nil
	}
}
