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
	"strconv"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
)

func init() {
	// Register the WhileNode with the component registry on initialization.
	Registry.Add(&WhileNode{})
}

// WhileNodeConfiguration defines the configuration for the WhileNode.
type WhileNodeConfiguration struct {
	// Condition is the expression to check before each iteration.
	// If the expression evaluates to true, the loop continues.
	// Uses 'el' expression language (e.g., "${msg.count} < 10").
	Condition string
	// Do specifies the node or sub-rule chain to process in each iteration,
	// e.g., "s1" or "chain:rule01".
	Do string
}

// WhileNode provides a while-loop structure.
// It executes the 'Do' node/chain repeatedly as long as the 'Condition' evaluates to true.
//
// WhileNode 提供 while 循环结构。
// 只要 'Condition' 评估为真，它就会重复执行 'Do' 节点/链。
//
// Configuration:
// 配置说明：
//
//	{
//		"condition": "${msg.count} < 5", // Expression to check  检查表达式
//		"do": "s3",                      // Target node ID or sub-chain  目标节点ID或子链
//	}
type WhileNode struct {
	// Config contains the node configuration.
	Config WhileNodeConfiguration
	// ruleNodeId is the parsed target for the 'Do' action.
	ruleNodeId types.RuleNodeId
	// conditionTemplate is the compiled template for the condition.
	conditionTemplate el.Template
}

// Type returns the component type.
func (x *WhileNode) Type() string {
	return "while"
}

func (x *WhileNode) New() types.Node {
	return &WhileNode{Config: WhileNodeConfiguration{
		Condition: "msg.count==nil || msg.count < 3",
	}}
}

// Init initializes the node.
func (x *WhileNode) Init(_ types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	x.Config.Condition = strings.TrimSpace(x.Config.Condition)
	if x.Config.Condition != "" {
		if template, err := el.NewExprTemplate(x.Config.Condition); err != nil {
			return fmt.Errorf("failed to create condition template: %w", err)
		} else {
			x.conditionTemplate = template
		}
	} else {
		return errors.New("condition is empty")
	}

	x.Config.Do = strings.TrimSpace(x.Config.Do)
	if x.Config.Do == "" {
		return errors.New("do is empty")
	}
	return x.formDoVar()
}

// OnMsg processes the message.
func (x *WhileNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error

	// Create a context with cancel for the loop execution
	ctxWithCancel, cancelFunc := context.WithCancel(ctx.GetContext())
	defer cancelFunc()

	var lastMsg = msg
	var index = 0

	for {
		// Update loop index in metadata
		msg.Metadata.PutValue(KeyLoopIndex, strconv.Itoa(index))

		// Check condition
		var conditionMet bool
		if x.conditionTemplate != nil {
			evn := base.NodeUtils.GetEvn(ctx, msg)
			if out, err := x.conditionTemplate.Execute(evn); err != nil {
				ctx.TellFailure(msg, err)
				return
			} else {
				// Parse result to boolean
				conditionMet = castToBool(out)
			}
		} else {
			conditionMet = false
		}

		if !conditionMet {
			break
		}

		// Execute the iteration
		var loopErr error
		if lastMsg, _, loopErr = x.executeItem(ctxWithCancel, ctx, msg); loopErr != nil {
			err = loopErr
			break
		}

		msg = lastMsg

		// Check for break signal
		if msg.Metadata.GetValue(MdKeyBreak) == MdValueBreak {
			msg.Metadata.Delete(MdKeyBreak)
			break
		}

		index++
	}

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		// msg is already lastMsg (final state).
		ctx.TellSuccess(msg)
	}
}

// Destroy cleans up resources.
func (x *WhileNode) Destroy() {
}

// executeItem processes the 'Do' node/chain.
func (x *WhileNode) executeItem(ctxWithCancel context.Context, ctx types.RuleContext, fromMsg types.RuleMsg) (types.RuleMsg, []string, error) {
	var wg sync.WaitGroup
	wg.Add(1)
	var returnErr error
	var lock sync.Mutex
	var msgData []string
	var lastMsg = fromMsg

	// Prepare callback
	onEnd := func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		if err != nil {
			returnErr = err
		} else {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
			msgData = append(msgData, msg.GetData())
		}
	}

	onAllCompleted := func() {
		wg.Done()
	}

	if x.ruleNodeId.Type == types.CHAIN {
		ctx.TellFlow(x.ruleNodeId.Id, fromMsg, types.WithContext(ctx.GetContext()), types.WithOnEnd(onEnd), types.WithOnAllNodeCompleted(onAllCompleted))
	} else {
		ctx.TellNode(ctx.GetContext(), x.ruleNodeId.Id, fromMsg, false, onEnd, onAllCompleted)
	}

	wg.Wait()

	if returnErr != nil {
		return lastMsg, msgData, returnErr
	}
	return lastMsg, msgData, ctxWithCancel.Err()
}

func (x *WhileNode) formDoVar() error {
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

// castToBool converts interface{} to bool.
func castToBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return strings.ToLower(v) == "true"
	case int, int8, int16, int32, int64:
		return v != 0
	case float32, float64:
		return v != 0
	default:
		return false
	}
}
