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
//		"timeout": 10
//	}
//}
import (
	"context"
	"sync"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

func init() {
	Registry.Add(&JoinNode{})
}

type JoinNodeConfiguration struct {
	//Timeout 执行超时，单位秒，默认0：代表不限制。
	Timeout int
}

// JoinNode 合并多个异步节点执行结果
// 内存优化版本：减少不必要的拷贝、使用对象池复用slice
type JoinNode struct {
	//节点配置
	Config JoinNodeConfiguration
	//消息缓冲池，复用slice减少GC压力
	msgBufferPool sync.Pool
}

// Type 组件类型
func (x *JoinNode) Type() string {
	return "join"
}

func (x *JoinNode) New() types.Node {
	node := &JoinNode{
		Config: JoinNodeConfiguration{},
	}
	// 初始化对象池
	node.msgBufferPool = sync.Pool{
		New: func() interface{} {
			// 预分配合理大小的slice，减少扩容开销
			return make([]types.WrapperMsg, 0, 8)
		},
	}
	return node
}

// Init 初始化
func (x *JoinNode) Init(_ types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)

	// 确保对象池已初始化
	if x.msgBufferPool.New == nil {
		x.msgBufferPool = sync.Pool{
			New: func() interface{} {
				return make([]types.WrapperMsg, 0, 8)
			},
		}
	}

	return err
}

// OnMsg processes the message with memory optimization.
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

	// 延迟拷贝：只有在需要修改消息时才进行拷贝
	var wrapperMsg types.RuleMsg

	ok := ctx.TellCollect(msg, func(msgList []types.WrapperMsg) {
		// 现在才进行消息拷贝
		wrapperMsg = msg.Copy()

		// 内存优化的消息处理
		x.processMessagesOptimized(msgList, &wrapperMsg)
		c <- true
	})

	if ok {
		// 等待执行结束或者超时
		select {
		case <-chanCtx.Done():
			// 超时时才创建拷贝
			if wrapperMsg.Id == "" {
				wrapperMsg = msg.Copy()
			}
			ctx.TellFailure(wrapperMsg, chanCtx.Err())
		case _ = <-c:
			ctx.TellSuccess(wrapperMsg)
		}
	}
}

// processMessagesOptimized 内存优化的消息处理方法
func (x *JoinNode) processMessagesOptimized(msgList []types.WrapperMsg, wrapperMsg *types.RuleMsg) {
	// 从对象池获取缓冲区
	filteredMsgs := x.msgBufferPool.Get().([]types.WrapperMsg)
	filteredMsgs = filteredMsgs[:0] // 重置长度但保留容量

	defer func() {
		// 归还到对象池前清理引用，防止内存泄漏
		for i := range filteredMsgs {
			filteredMsgs[i] = types.WrapperMsg{}
		}
		x.msgBufferPool.Put(filteredMsgs)
	}()

	// 内存优化的过滤和元数据合并
	x.filterAndMergeOptimized(msgList, &filteredMsgs, wrapperMsg)

	// 设置处理后的数据
	wrapperMsg.DataType = types.JSON
	wrapperMsg.SetData(str.ToString(filteredMsgs))
}

// filterAndMergeOptimized 内存优化的过滤和元数据合并
func (x *JoinNode) filterAndMergeOptimized(msgList []types.WrapperMsg, filteredMsgs *[]types.WrapperMsg, wrapperMsg *types.RuleMsg) {
	// 预分配元数据映射的合理大小
	metadataMap := make(map[string]string, len(msgList)*2)

	for _, msg := range msgList {
		if msg.NodeId != "" {
			// 创建消息副本，但清理元数据以减少内存占用
			msgCopy := msg
			if msgCopy.Msg.Metadata != nil {
				msgCopy.Msg.Metadata.Clear()
			}
			*filteredMsgs = append(*filteredMsgs, msgCopy)

			// 高效的元数据合并：只合并成功的消息
			if msg.Err == "" && msg.Msg.Metadata != nil {
				for k, v := range msg.Msg.Metadata.Values() {
					metadataMap[k] = v
				}
			}
		}
	}

	// 批量设置元数据，减少锁开销
	if len(metadataMap) > 0 {
		wrapperMsg.Metadata.ReplaceAll(metadataMap)
	}
}

func (x *JoinNode) Destroy() {
	// 清理对象池（如果需要）
}
