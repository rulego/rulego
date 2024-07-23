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
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strconv"
	"strings"
	"sync"
)

const (
	// KeyLoopIndex is the current index during iteration.
	KeyLoopIndex = "_loopIndex"
	// KeyLoopKey is the current key during iteration.
	KeyLoopKey = "_loopKey"
	// KeyLoopItem is the current item during iteration.
	KeyLoopItem = "_loopItem"
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
}

// ForNode iterates over msg or a specified field item value in msg to the next node.
// The iterated field value must be of array, slice, or struct type, and supports extracting iteration values through expr expressions.
// After iteration ends, the original msg is sent to the next node through the Success chain.
// If the context is canceled or fails, the original msg is sent to the next node through the `Failure` chain.
// Use metadata._loopIndex to get the current index of the iteration.
// Use metadata._loopItem to get the current item of the iteration.
// Use metadata._loopKey to get the current key of the iteration; this only has a value when iterating over a struct.
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

// OnMsg processes the message.
func (x *ForNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error
	evn := base.NodeUtils.GetEvn(ctx, msg)
	var inData = msg.Data
	var data interface{}
	var exprVm = vm.VM{}
	if x.program != nil {
		if out, err := exprVm.Run(x.program, evn); err != nil {
			ctx.TellFailure(msg, err)
			return
		} else {
			data = out
		}
	}
	ctxWithCancel, cancelFunc := context.WithCancel(ctx.GetContext())
	defer cancelFunc()

	switch v := data.(type) {
	case []interface{}:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Data = str.ToString(item)
			msg.Metadata.PutValue(KeyLoopItem, msg.Data)
			// 执行并，检查是否有取消请求
			if err = x.executeItem(ctxWithCancel, ctx, msg); err != nil {
				break
			}
		}
	case []int:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if err = x.executeItem(ctxWithCancel, ctx, msg); err != nil {
				break
			}
		}
	case []int64:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if err = x.executeItem(ctxWithCancel, ctx, msg); err != nil {
				break
			}
		}
	case []float64:
		for index, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if err = x.executeItem(ctxWithCancel, ctx, msg); err != nil {
				break
			}
		}
	case map[string]interface{}:
		index := 0
		for k, item := range v {
			msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))
			msg.Metadata.PutValue(KeyLoopKey, k)
			msg.Metadata.PutValue(KeyLoopItem, str.ToString(item))
			// 执行并，检查是否有取消请求
			if err = x.executeItem(ctxWithCancel, ctx, msg); err != nil {
				break
			}
			index++
		}
	default:
		err = errors.New("value is not a supported. must array slice or struct type")
	}
	//不修改in data
	msg.Data = inData
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		ctx.TellSuccess(msg)
	}
}

// Destroy cleans up resources used by the ForNode.
func (x *ForNode) Destroy() {
}

// executeItem processes each item during iteration.
func (x *ForNode) executeItem(ctxWithCancel context.Context, ctx types.RuleContext, fromMsg types.RuleMsg) error {
	var wg sync.WaitGroup
	wg.Add(1)
	var returnErr error
	var lock sync.Mutex
	if x.ruleNodeId.Type == types.CHAIN {
		ctx.TellFlow(ctx.GetContext(), x.ruleNodeId.Id, fromMsg, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			if err != nil {
				returnErr = err
			} else {
				lock.Lock()
				defer lock.Unlock()
				// copy metadata
				for k, v := range msg.Metadata {
					fromMsg.Metadata.PutValue(k, v)
				}
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
				// copy metadata
				for k, v := range msg.Metadata {
					fromMsg.Metadata.PutValue(k, v)
				}
			}
		}, func() {
			wg.Done()
		})
	}
	wg.Wait()

	if returnErr != nil {
		return returnErr
	} else {
		return ctxWithCancel.Err()
	}
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
