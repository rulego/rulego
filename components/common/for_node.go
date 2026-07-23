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
	"github.com/rulego/rulego/utils/el"
	"strconv"
	"strings"
	"sync"

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
	// AsyncProcess processes each item asynchronously, without focusing on the results
	AsyncProcess = 3
)

func init() {
	// Register the ForNode with the component registry on initialization.
	Registry.Add(&ForNode{})
}

// ForNodeConfiguration defines the configuration for the ForNode.
type ForNodeConfiguration struct {
	// Range is the target expression to iterate over.
	// Supports msg fields, metadata, numeric ranges (1..5), and expressions.
	// If empty, iterates over the msg payload.
	Range string `json:"range" label:"Range" desc:"Target to iterate: msg.items, metadata.list, 1..5, or expression. Empty=msg payload" required:"true"`
	// Do is the node ID or sub-rule chain to process each element.
	// Format: {nodeId} or chain:{chainId}
	Do string `json:"do" label:"Do" desc:"Node ID or sub-chain to process each item. Format: {nodeId} or chain:{chainId}" required:"true"`
	// Mode: 0=do not process, 1=merge results, 2=replace msg, 3=async
	Mode int `json:"mode" label:"Mode" desc:"0=ignore results, 1=merge into array, 2=replace msg each iteration, 3=async fire-and-forget"`
}

// ForNode provides iteration capabilities for processing collections, arrays, and data structures.
// It supports various iteration patterns including synchronous/asynchronous processing,
// result merging, value replacement, and integration with sub-rule chains or individual nodes.
//
// ForNode provides iterative capabilities for handling collections, arrays, and data structures.
// Supports various iteration modes, including synchronous/asynchronous processing, result merging, value substitution, and integration with sub-rule chains or individual nodes.
//
// Configuration:
// Configuration:
//
//	{
//		"range": "msg.items",           // Target to iterate: msg field, metadata, or expression
//		"do": "s3",                     // Target node ID or sub-chain: "nodeId" or "chain:chainId"
//		"mode": 1                       // Processing mode: 0=no processing, 1=merge, 2=replace, 3=async
//	}
//
// Range Expressions:
// Range expression:
//
// The range field supports various data sources and expressions:
// The Range field supports various data sources and expressions:
//   - Message fields: "msg.items", "msg.users"
//   - Metadata: "metadata.list"
//   - Numeric ranges: "1..5" creates [1,2,3,4,5]
//   - Complex expressions: "msg.data.products"
//   - Empty: Iterates over entire message payload
//
// Processing Modes:
// Processing Mode:
//
//   - 0 (DoNotProcess): Execute target without processing results
//   - 1 (MergeValues): Merge all iteration results into array
//   - 2 (ReplaceValues): Replace message with each iteration result
//   - 3 (AsyncProcess): Process each item asynchronously without waiting
//
// Target Execution:
// Objective Execution:
//
//   - Node ID: "s3" - Execute specific node in current rule chain
//   - Sub-chain: "chain:rule01" - Execute sub-rule chain
//
// Iteration Context Variables:
// Iterative context variables:
//
// During iteration, the component sets metadata variables:
// During iteration, components set metadata variables:
//   - _loopIndex: Current iteration index (0-based)
//   - _loopItem: Current item value being processed
//   - _loopKey: Current key (only for map/object iteration)
//
// Supported Data Types:
// Supported data types:
//
//   - []interface{}: Generic arrays and slices
//   - []int, []int64, []float64: Typed numeric arrays
//   - map[string]interface{}: Objects and maps
//   - Automatically handles JSON parsing for complex data
//
// Synchronous vs Asynchronous Processing:
// Synchronous and asynchronous processing:
//
//   - Modes 0-2: Synchronous processing with result collection
//   - Mode 3: Asynchronous processing for high-throughput scenarios
//   - Synchronous modes wait for all iterations to complete
//   - Asynchronous mode fires and forgets each iteration
//
// Error Handling:
// Error handling:
//
//   - Invalid range expressions result in Failure chain execution
//   - Unsupported data types are rejected
//   - Individual iteration errors are aggregated
//   - Context cancellation stops iteration
//
// Output Relations:
// Output relationships:
//
//   - Success: Iteration completed successfully
//   - Failure: Range evaluation error, unsupported data type, or iteration error. Range evaluation error, unsupported data type, or iteration error
//
// Usage Examples:
// Example:
//
//	// Process array items and merge results
//	Handle array items and merge results
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
//	Execute subchains for each user
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
//	High-throughput asynchronous processing
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
	//Node configuration
	Config ForNodeConfiguration
	// do variable nodeId or chainId
	ruleNodeId types.RuleNodeId
	// range template
	rangeTemplate el.Template
}

// Type returns the component type
func (x *ForNode) Type() string {
	return "for"
}

func (x *ForNode) New() types.Node {
	return &ForNode{Config: ForNodeConfiguration{
		Range: "1..3",
		Do:    "s3",
	}}
}

// Init initializes the component
func (x *ForNode) Init(_ types.Config, configuration types.Configuration) error {
	// Map the configuration to the ForNodeConfiguration struct.
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	// Trim whitespace from the Range configuration.
	x.Config.Range = strings.TrimSpace(x.Config.Range)
	// Compile the Range expression if it's not empty.
	if x.Config.Range != "" {

		if template, err := el.NewExprTemplate(x.Config.Range); err != nil {
			return fmt.Errorf("failed to create range template: %w", err)
		} else {
			x.rangeTemplate = template
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

	var inData = msg.GetData()
	var data interface{}
	if x.rangeTemplate != nil {
		evn := base.NodeUtils.GetEvn(ctx, msg)
		if out, err := x.rangeTemplate.Execute(evn); err != nil {
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

			// Execute and check if there are any cancellation requests
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}

			// Detects whether an interrupt has been triggered
			if msg.Metadata.GetValue(MdKeyBreak) == MdValueBreak {
				msg.Metadata.Delete(MdKeyBreak)
				break
			}
		}
	case []int:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// Execute and check if there are any cancellation requests
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}

			// Detects whether an interrupt has been triggered
			if msg.Metadata.GetValue(MdKeyBreak) == MdValueBreak {
				msg.Metadata.Delete(MdKeyBreak)
				break
			}
		}
	case []int64:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// Execute and check if there are any cancellation requests
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}

			// Detects whether an interrupt has been triggered
			if msg.Metadata.GetValue(MdKeyBreak) == MdValueBreak {
				msg.Metadata.Delete(MdKeyBreak)
				break
			}
		}
	case []float64:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// Execute and check if there are any cancellation requests
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}

			// Detects whether an interrupt has been triggered
			if msg.Metadata.GetValue(MdKeyBreak) == MdValueBreak {
				msg.Metadata.Delete(MdKeyBreak)
				break
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
			// Execute and check if there are any cancellation requests
			if lastMsg, itemDataList, err = x.executeItem(ctxWithCancel, ctx, msg, x.Config.Mode); err != nil {
				break
			} else if x.Config.Mode == MergeValues {
				resultData = append(resultData, x.toList(msg.DataType, itemDataList)...)
			} else if x.Config.Mode == ReplaceValues {
				msg = lastMsg
			}

			// Detects whether an interrupt has been triggered
			if msg.Metadata.GetValue(MdKeyBreak) == MdValueBreak {
				msg.Metadata.Delete(MdKeyBreak)
				break
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
			//Do not modify the data in the data
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
		//Asynchronous
		return fromMsg, nil, x.asyncExecuteItem(ctxWithCancel, ctx, fromMsg)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var returnErr error
	var lock sync.Mutex
	var msgData []string
	var lastMsg types.RuleMsg
	if x.ruleNodeId.Type == types.CHAIN {
		ctx.TellFlow(x.ruleNodeId.Id, fromMsg, types.WithContext(ctx.GetContext()), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
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
		}), types.WithOnAllNodeCompleted(func() {
			wg.Done()
		}))
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

// Execute each item asynchronously
func (x *ForNode) asyncExecuteItem(ctxWithCancel context.Context, ctx types.RuleContext, fromMsg types.RuleMsg) error {
	fromMsg = fromMsg.Copy()
	if x.ruleNodeId.Type == types.CHAIN {
		ctx.TellFlow(x.ruleNodeId.Id, fromMsg, types.WithContext(ctx.GetContext()))
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

// Desc returns the component description
func (x *ForNode) Desc() string {
	return "Iterate over collections with range expression. do specifies processing node/chain. mode: 0=ignore, 1=merge, 2=replace, 3=async. Routes to Success/Failure"
}
