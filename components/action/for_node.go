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
//	"type": "for",
//	"name": "Iteration",
//	"debugMode": false,
//		"configuration": {
//			"range": "msg.items",
//			"do":        "s3"
//		}
//	}
//}
import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

const (
	// KeyLoopIndex is the current index during iteration.
	KeyLoopIndex = "_loopIndex"
	// KeyLoopKey is the current key during iteration.
	KeyLoopKey = "_loopKey"
	// KeyLoopItem is the current item during iteration.
	KeyLoopItem = "_loopItem"
)
const (
	// DoNotProcess indicates that the iterated values should not be processed.
	DoNotProcess = 0
	// MergeValues indicates that the iterated values should be merged.
	MergeValues = 1
	// ReplaceValues indicates that the iterated values should be replaced and passed to the next iteration.
	ReplaceValues = 2
	// AsyncProcess 异步处理每一项item，不关注结果
	AsyncProcess = 3
)

func init() {
	// Register the ForNode with the component registry on initialization.
	Registry.Add(&ForNode{})
}

// ForNodeConfiguration defines the configuration for the ForNode.
type ForNodeConfiguration struct {
	// Range is the target expression to iterate over, supporting arrays, slices, and structs.
	// Expr expressions are allowed, e.g., msg.items, 1..5 to iterate over []int{1,2,3,4,5}.
	// If empty, it iterates over the msg payload.
	Range string
	// Do specifies the node or sub-rule chain to process the iterated elements,
	//e.g., "s1", where the item will start executing from the branch chain of s1 until the chain completes,
	//then returns to the starting point of the iteration.
	// The item can also be processed by a sub-rule chain, using the syntax: chain:chainId,
	//e.g., chain:rule01, where the item will start executing from the sub-chain rule01 until the chain completes,
	//then returns to the starting point of the iteration.
	Do string
	// Mode 0:不处理msg，1：合并遍历msg.Data，2：替换msg,3:异步处理每一项
	Mode int
}

// ForNode provides iteration capabilities for processing collections, arrays, and data structures.
// It supports various iteration patterns including synchronous/asynchronous processing,
// result merging, value replacement, and integration with sub-rule chains or individual nodes.
//
// ForNode 为处理集合、数组和数据结构提供迭代能力。
// 支持各种迭代模式，包括同步/异步处理、结果合并、值替换和与子规则链或单个节点的集成。
//
// Configuration:
// 配置说明：
//
//	{
//		"range": "msg.items",           // Target to iterate: msg field, metadata, or expression  迭代目标：消息字段、元数据或表达式
//		"do": "s3",                     // Target node ID or sub-chain: "nodeId" or "chain:chainId"  目标节点ID或子链
//		"mode": 1                       // Processing mode: 0=no processing, 1=merge, 2=replace, 3=async  处理模式
//	}
//
// Range Expressions:
// Range 表达式：
//
// The range field supports various data sources and expressions:
// Range 字段支持各种数据源和表达式：
//   - Message fields: "msg.items", "msg.users"  消息字段
//   - Metadata: "metadata.list"  元数据
//   - Numeric ranges: "1..5" creates [1,2,3,4,5]  数值范围
//   - Complex expressions: "msg.data.products"  复杂表达式
//   - Empty: Iterates over entire message payload  空值：遍历整个消息负荷
//
// Processing Modes:
// 处理模式：
//
//   - 0 (DoNotProcess): Execute target without processing results  执行目标但不处理结果
//   - 1 (MergeValues): Merge all iteration results into array  将所有迭代结果合并为数组
//   - 2 (ReplaceValues): Replace message with each iteration result  用每次迭代结果替换消息
//   - 3 (AsyncProcess): Process each item asynchronously without waiting  异步处理每个项目而不等待
//
// Target Execution:
// 目标执行：
//
//   - Node ID: "s3" - Execute specific node in current rule chain  节点ID：在当前规则链中执行特定节点
//   - Sub-chain: "chain:rule01" - Execute sub-rule chain  子链：执行子规则链
//
// Iteration Context Variables:
// 迭代上下文变量：
//
// During iteration, the component sets metadata variables:
// 迭代期间，组件设置元数据变量：
//   - _loopIndex: Current iteration index (0-based)  当前迭代索引（从0开始）
//   - _loopItem: Current item value being processed  正在处理的当前项目值
//   - _loopKey: Current key (only for map/object iteration)  当前键（仅用于映射/对象迭代）
//
// Supported Data Types:
// 支持的数据类型：
//
//   - []interface{}: Generic arrays and slices  通用数组和切片
//   - []int, []int64, []float64: Typed numeric arrays  类型化数值数组
//   - map[string]interface{}: Objects and maps  对象和映射
//   - Automatically handles JSON parsing for complex data  自动处理复杂数据的 JSON 解析
//
// Synchronous vs Asynchronous Processing:
// 同步与异步处理：
//
//   - Modes 0-2: Synchronous processing with result collection  模式 0-2：同步处理并收集结果
//   - Mode 3: Asynchronous processing for high-throughput scenarios  模式 3：高吞吐量场景的异步处理
//   - Synchronous modes wait for all iterations to complete  同步模式等待所有迭代完成
//   - Asynchronous mode fires and forgets each iteration  异步模式发送后即忘记每次迭代
//
// Error Handling:
// 错误处理：
//
//   - Invalid range expressions result in Failure chain execution  无效的 range 表达式导致 Failure 链执行
//   - Unsupported data types are rejected  不支持的数据类型被拒绝
//   - Individual iteration errors are aggregated  单个迭代错误会被聚合
//   - Context cancellation stops iteration  上下文取消会停止迭代
//
// Output Relations:
// 输出关系：
//
//   - Success: Iteration completed successfully  迭代成功完成
//   - Failure: Range evaluation error, unsupported data type, or iteration error  Range 评估错误、不支持的数据类型或迭代错误
//
// Usage Examples:
// 使用示例：
//
//	// Process array items and merge results
//	// 处理数组项目并合并结果
//	{
//		"id": "processItems",
//		"type": "for",
//		"configuration": {
//			"range": "msg.orderItems",
//			"do": "processOrderItem",
//			"mode": 1
//		}
//	}
//
//	// Execute sub-chain for each user
//	// 为每个用户执行子链
//	{
//		"id": "processUsers",
//		"type": "for",
//		"configuration": {
//			"range": "msg.users",
//			"do": "chain:userProcessingChain",
//			"mode": 2
//		}
//	}
//
//	// Async processing for high-throughput
//	// 高吞吐量异步处理
//	{
//		"id": "asyncNotify",
//		"type": "for",
//		"configuration": {
//			"range": "1..1000",
//			"do": "sendNotification",
//			"mode": 3
//		}
//	}
type ForNode struct {
	//节点配置
	Config ForNodeConfiguration
	// range variable
	program *vm.Program
	// do variable nodeId or chainId
	ruleNodeId types.RuleNodeId
}

// Type 组件类型
func (x *ForNode) Type() string {
	return "for"
}

func (x *ForNode) New() types.Node {
	return &ForNode{Config: ForNodeConfiguration{
		Range: "1..3",
		Do:    "s3",
	}}
}

// Init 初始化
func (x *ForNode) Init(_ types.Config, configuration types.Configuration) error {
	// Map the configuration to the ForNodeConfiguration struct.
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	// Trim whitespace from the Range configuration.
	x.Config.Range = strings.TrimSpace(x.Config.Range)
	// Compile the Range expression if it's not empty.
	if x.Config.Range != "" {
		if program, err := expr.Compile(x.Config.Range, expr.AllowUndefinedVariables()); err != nil {
			return err
		} else {
			x.program = program
		}
	}
	// Trim whitespace from the Do configuration and validate it's not empty.
	x.Config.Do = strings.TrimSpace(x.Config.Do)
	if x.Config.Do == "" {
		return errors.New("do is empty")
	}
	return x.formDoVar()
}

func (x *ForNode) toMap(data string) interface{} {
	var dataMap interface{}
	if err := json.Unmarshal([]byte(data), &dataMap); err == nil {
		return dataMap
	} else {
		return data
	}
}

func (x *ForNode) toList(dataType types.DataType, itemDataList []string) []interface{} {
	var resultData []interface{}
	for _, itemData := range itemDataList {
		if dataType == types.JSON {
			resultData = append(resultData, x.toMap(itemData))
		} else {
			resultData = append(resultData, itemData)
		}
	}
	return resultData
}

// OnMsg processes the message.
func (x *ForNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error
	evn := base.NodeUtils.GetEvn(ctx, msg)
	var inData = msg.GetData()
	var data interface{}
	var exprVm = vm.VM{}
	if x.program != nil {
		if out, err := exprVm.Run(x.program, evn); err != nil {
			ctx.TellFailure(msg, err)
			return
		} else {
			data = out
		}
	} else {
		data = x.toMap(inData)
	}
	ctxWithCancel, cancelFunc := context.WithCancel(ctx.GetContext())
	defer cancelFunc()

	var resultData []interface{}
	var itemDataList []string
	var lastMsg types.RuleMsg
	switch v := data.(type) {
	case []interface{}:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			if x.Config.Mode != ReplaceValues || index == 0 {
				msg.SetData(str.ToString(item))
				msg.Metadata.PutValue(KeyLoopItem, msg.GetData())
			} else {
				msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			}

			// 执行并，检查是否有取消请求
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}
		}
	case []int:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}
		}
	case []int64:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}
		}
	case []float64:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}
		}
	case map[string]interface{}:
		index := 0
		for k, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopKey, k)
			if x.Config.Mode != ReplaceValues || index == 0 {
				msg.SetData(str.ToString(item))
				msg.Metadata.PutValue(KeyLoopItem, msg.GetData())
			} else {
				msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			}
			// 执行并，检查是否有取消请求
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}
			index++
		}
	default:
		err = errors.New("must array slice or struct type")
	}

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if x.Config.Mode == DoNotProcess || x.Config.Mode == AsyncProcess {
			//不修改in data
			msg.SetData(inData)
		} else if x.Config.Mode == MergeValues {
			msg.SetData(str.ToString(resultData))
		}
		ctx.TellSuccess(msg)
	}
}

// Destroy cleans up resources used by the ForNode.
func (x *ForNode) Destroy() {
}

// executeItem processes each item during iteration.
func (x *ForNode) executeItem(ctxWithCancel context.Context, ctx types.RuleContext, fromMsg types.RuleMsg, mode int) (types.RuleMsg, []string, error) {
	if mode == AsyncProcess {
		//异步
		return fromMsg, nil, x.asyncExecuteItem(ctxWithCancel, ctx, fromMsg)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var returnErr error
	var lock sync.Mutex
	var msgData []string
	var lastMsg types.RuleMsg
	if x.ruleNodeId.Type == types.CHAIN {
		ctx.TellFlow(ctx.GetContext(), x.ruleNodeId.Id, fromMsg, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			if err != nil {
				returnErr = err
			} else {
				lock.Lock()
				defer lock.Unlock()
				lastMsg = msg
				// copy metadata
				for k, v := range msg.Metadata.Values() {
					fromMsg.Metadata.PutValue(k, v)
				}
				msgData = append(msgData, msg.GetData())
			}
		}, func() {
			wg.Done()
		})
	} else {
		ctx.TellNode(ctx.GetContext(), x.ruleNodeId.Id, fromMsg, false, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			if err != nil {
				returnErr = err
			} else {
				lock.Lock()
				defer lock.Unlock()
				lastMsg = msg
				// copy metadata
				for k, v := range msg.Metadata.Values() {
					fromMsg.Metadata.PutValue(k, v)
				}
				msgData = append(msgData, msg.GetData())
			}
		}, func() {
			wg.Done()
		})
	}
	wg.Wait()

	if returnErr != nil {
		return lastMsg, msgData, returnErr
	} else {
		return lastMsg, msgData, ctxWithCancel.Err()
	}
}

// 异步执行每一项
func (x *ForNode) asyncExecuteItem(ctxWithCancel context.Context, ctx types.RuleContext, fromMsg types.RuleMsg) error {
	fromMsg = fromMsg.Copy()
	if x.ruleNodeId.Type == types.CHAIN {
		ctx.TellFlow(ctx.GetContext(), x.ruleNodeId.Id, fromMsg, nil, nil)
	} else {
		ctx.TellNode(ctx.GetContext(), x.ruleNodeId.Id, fromMsg, false, nil, nil)
	}
	return ctxWithCancel.Err()
}

// formDoVar forms the Do variable from the configuration.
func (x *ForNode) formDoVar() error {
	values := strings.Split(x.Config.Do, ":")
	length := len(values)
	if length == 1 {
		x.ruleNodeId = types.RuleNodeId{
			Id:   strings.TrimSpace(values[0]),
			Type: types.NODE,
		}
	} else if length == 2 {
		if strings.TrimSpace(values[0]) == "chain" {
			x.ruleNodeId = types.RuleNodeId{
				Id:   strings.TrimSpace(values[1]),
				Type: types.CHAIN,
			}
		} else {
			x.ruleNodeId = types.RuleNodeId{
				Id:   strings.TrimSpace(values[1]),
				Type: types.NODE,
			}
		}
	} else {
		return fmt.Errorf("do variable should be nodeId or chain:chainId style")
	}
	return nil
}
