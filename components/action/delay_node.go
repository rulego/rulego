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

//Example of rule chain node configuration:
//{
//        "id": "s1",
//        "type": "delay",
//        "name": "延迟节点",
//        "debugMode": false,
//        "configuration": {
//          "periodInSeconds": 1,
//          "maxPendingMsgs": 1000
//        }
//  }
import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

var DelayNodeMsgType = "DELAY_NODE_MSG_TYPE"

// KeyDelayOffsetMs Internal special metadata key: Delay offset time (milliseconds), used for component execution to resume from the offset point after recovery
// KeyDelayOffsetMs internal special metadata key: delay offset time in milliseconds
const KeyDelayOffsetMs = "_delayOffsetMs"

// Register the node
func init() {
	Registry.Add(&DelayNode{})
}

// DelayNodeConfiguration node configuration
type DelayNodeConfiguration struct {
	// MaxPendingMsgs is the maximum number of pending messages allowed in the delay queue.
	MaxPendingMsgs int `json:"maxPendingMsgs" label:"Max Pending Messages" desc:"Maximum pending messages in delay queue, default 1000"`
	// DelayMs is the delay duration in milliseconds. Supports numbers or expressions like ${metadata.delay}.
	DelayMs string `json:"delayMs" label:"Delay (ms)" desc:"Delay duration in ms. Supports numbers or ${metadata.key} expressions" required:"true"`
	// Overwrite: true keeps only one message during the period, new messages overwrite previous ones.
	Overwrite bool `json:"overwrite" label:"Overwrite" desc:"true=keep only one pending message (new overwrites old), false=queue all messages"`

	// Deprecated: Use DelayMs instead
	PeriodInSeconds int `json:"periodInSeconds" deprecated:"true"`
	// Deprecated: Use DelayMs instead
	PeriodInSecondsPattern string `json:"periodInSecondsPattern" deprecated:"true"`
}

// DelayNode provides components with message delay capabilities, supporting both static and dynamic delay times
// DelayNode provides message delay capabilities with configurable timing and queue management.
//
// Core algorithm:
// Core Algorithm:
// 1. Messages enter pending queue with delay timer
// 2. After the timer expires, remove from queue and send to Success - Timer expires, remove from queue and send to Success
// 3. Overwrite mode: Only keep one message at a time
// 4. Send to Failure on queue overflow when the queue overflows
//
// Delay mechanisms:
//   - Static delay: periodInSeconds - Static delay: periodInSeconds
//   - Dynamic delay: periodInSecondsPattern variable substitution
//
// Message overwrite modes:
//   - overwrite=false: Queue all messages
//   - overwrite=true: Replace pending message with new one
type DelayNode struct {
	//Node configuration
	Config DelayNodeConfiguration
	//Message queue
	PendingMsgs map[string]types.RuleMsg
	//Previous pending msg id
	LastPendingMsgId atomic.Value
	//Lock
	mu sync.Mutex
	// delayMsTemplate: A latency time template used to parse dynamic delay times
	// delayMsTemplate template for resolving dynamic delay time
	delayMsTemplate el.Template
	// delayMsValue The delay time value (in milliseconds) for preparsing, used when DelayMs is a pure number
	// delayMsValue pre-parsed delay time value in milliseconds, used when DelayMs is a pure number
	delayMsValue int64
}

// Type returns the component type
func (x *DelayNode) Type() string {
	return "delay"
}

func (x *DelayNode) New() types.Node {
	return &DelayNode{Config: DelayNodeConfiguration{MaxPendingMsgs: 1000, DelayMs: "60000"}}
}

// Init initializes the component
func (x *DelayNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	x.PendingMsgs = make(map[string]types.RuleMsg)
	x.Config = DelayNodeConfiguration{} //Clear the configuration; otherwise, the default value will be retained
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	if x.Config.MaxPendingMsgs <= 0 {
		x.Config.MaxPendingMsgs = 1000
	}
	x.LastPendingMsgId.Store("")

	// Initialization delay time analysis
	// Initialize delay time parsing
	x.Config.DelayMs = strings.TrimSpace(x.Config.DelayMs)
	if x.Config.DelayMs != "" {
		// Try to parse directly into a numerical value
		if value, err := strconv.ParseInt(x.Config.DelayMs, 10, 64); err == nil {
			// It is a pure number and stores preparative values
			x.delayMsValue = value
		} else {
			// Not just numbers, creating templates
			x.delayMsTemplate, err = el.NewTemplate(x.Config.DelayMs)
			if err != nil {
				return fmt.Errorf("failed to create delay time template: %w", err)
			}
		}
	}

	return nil
}

// getDelayMilliseconds gets the delay time (in milliseconds), supporting both numerical and template methods
// getDelayMilliseconds gets the delay time in milliseconds, supporting both numeric and template modes
func (x *DelayNode) getDelayMilliseconds(ctx types.RuleContext, msg types.RuleMsg) (int64, error) {
	// Prioritize the use of the new DelayMs parameters
	if x.Config.DelayMs != "" {
		// If there is a pre-parsed value, it returns directly
		if x.delayMsValue > 0 {
			return x.delayMsValue, nil
		}
		// If there is a template, use template parsing
		if x.delayMsTemplate != nil {
			evn := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
			delayStr := x.delayMsTemplate.ExecuteAsString(evn)
			if v, err := strconv.ParseInt(delayStr, 10, 64); err != nil {
				return 0, fmt.Errorf("failed to parse delay time from template result '%s': %w", delayStr, err)
			} else {
				return v, nil
			}
		}
		return 0, fmt.Errorf("no delay time configured")
	}

	// Compatible with older second-level parameters
	periodInSeconds := x.Config.PeriodInSeconds
	//Obtain latency from variables
	if x.Config.PeriodInSecondsPattern != "" {
		evn := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
		if v, err := strconv.Atoi(str.ExecuteTemplate(x.Config.PeriodInSecondsPattern, evn)); err != nil {
			return 0, err
		} else {
			periodInSeconds = v
		}
	}
	return int64(periodInSeconds * 1000), nil
}

// getOffsetMilliseconds Gets the delay offset time (in milliseconds) from metadata
// getOffsetMilliseconds reads delay offset time in milliseconds from message metadata
func (x *DelayNode) getOffsetMilliseconds(msg types.RuleMsg) (int64, error) {
	if msg.Metadata == nil {
		return 0, nil
	}
	v := msg.Metadata.GetValue(KeyDelayOffsetMs)
	if v == "" {
		return 0, nil
	}
	if offset, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err != nil {
		return 0, fmt.Errorf("failed to parse offset ms from metadata '%s': %w", v, err)
	} else {
		return offset, nil
	}
}

// OnMsg processes messages and implements delayed queue logic
func (x *DelayNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {

	if msg.Type == DelayNodeMsgType {
		x.mu.Lock()
		defer x.mu.Unlock()
		pendingMsg, ok := x.PendingMsgs[msg.Id]
		if ok {
			//Clear messages within the cycle
			if x.Config.Overwrite {
				x.LastPendingMsgId.Store("")
			}

			delete(x.PendingMsgs, msg.Id)
			ctx.TellSuccess(pendingMsg)
		} else {
			ctx.TellFailure(msg, fmt.Errorf("msg not found"))
		}

	} else if oldMsgId := x.LastPendingMsgId.Load().(string); oldMsgId != "" {
		//If you are in overwrite mode, replace the message in the queue
		x.mu.Lock()
		defer x.mu.Unlock()
		x.PendingMsgs[oldMsgId] = msg
	} else {
		//Get queue length
		x.mu.Lock()
		length := len(x.PendingMsgs)
		x.mu.Unlock()

		if length < x.Config.MaxPendingMsgs {
			// Get the delay time
			periodInMilliseconds, err := x.getDelayMilliseconds(ctx, msg)
			if err != nil {
				ctx.TellFailure(msg, err)
				return
			}
			// Read offset time from metadata
			offsetMs, err := x.getOffsetMilliseconds(msg)
			if err != nil {
				ctx.TellFailure(msg, err)
				return
			}
			// Calculate actual delay
			adjustedDelay := periodInMilliseconds - offsetMs
			if adjustedDelay <= 0 {
				// If less than or equal to 0, execute immediately and no longer enter the delay queue
				ctx.TellSuccess(msg)
				return
			}

			//If it is override mode,
			if x.Config.Overwrite {
				x.LastPendingMsgId.Store(msg.Id)
			}
			x.mu.Lock()
			x.PendingMsgs[msg.Id] = msg
			x.mu.Unlock()

			ackMsg := msg.Copy()
			ackMsg.Type = DelayNodeMsgType
			ctx.TellSelf(ackMsg, adjustedDelay)
		} else {
			ctx.TellFailure(msg, fmt.Errorf("max limit of pending messages"))
		}
	}

}

// Destroy releases resources
func (x *DelayNode) Destroy() {
}

// Desc returns the component description
func (x *DelayNode) Desc() string {
	return "Delay message delivery. delayMs supports static values or ${metadata.key} expressions. overwrite=true keeps only one pending message. Routes to Success/Failure"
}
