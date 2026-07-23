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
//        "id": "s2",
//        "type": "functions",
//        "name": "函数调用",
//        "debugMode": false,
//        "configuration": {
//          "functionName": "test"
//        }
//  }
import (
	"fmt"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
)

// Functions: A global function registry for registering and finding custom handler functions
// Functions is the global registry for custom functions that can be called by FunctionsNode.
var Functions = &FunctionsRegistry{}

// init registers the FunctionsNode component
// init registers the FunctionsNode component with the default registry.
func init() {
	Registry.Add(&FunctionsNode{})
}

// FunctionDef function definition
// FunctionDef defines the structure for a registered function, including metadata and implementation.
type FunctionDef struct {
	// Name function name
	// Name is the unique identifier for the function.
	Name string `json:"name"`
	// The Label function displays the name/label
	// Label is the display name or label for the function.
	Label string `json:"label"`
	// Desc function description
	// Desc provides a description of what the function does.
	Desc string `json:"desc"`
	// F function implementation
	// F is the actual function implementation to be executed.
	F func(ctx types.RuleContext, msg types.RuleMsg) `json:"-"`
}

// FunctionsRegistry: A custom handler function registry thread-safe for thread safety
// FunctionsRegistry is a thread-safe registry for custom processing functions.
//
// Function signature:
//   - func(ctx types.RuleContext, msg types.RuleMsg)
//   - Functions must call ctx.TellSuccess/TellNext/TellFailure for routing - Functions must call ctx. Tell* methods for routing
type FunctionsRegistry struct {
	// functions: stores the mapping from function names to definitions
	// functions stores the mapping from function names to their definitions
	functions map[string]FunctionDef
	// functionNames stores a list of function names to maintain the registration order
	// functionNames stores the list of function names to maintain registration order
	functionNames []string
	sync.RWMutex
}

// Register: Register the function to the registry
// Register adds a new function to the registry with the specified name.
// params[0] label
// params[1] desc
func (x *FunctionsRegistry) Register(functionName string, f func(ctx types.RuleContext, msg types.RuleMsg), params ...string) {
	def := FunctionDef{
		Name: functionName,
		F:    f,
	}
	if len(params) > 0 {
		def.Label = params[0]
	}
	if len(params) > 1 {
		def.Desc = params[1]
	}
	x.RegisterDef(def)
}

// RegisterDef The registration function is defined in the registry
// RegisterDef adds a new function definition to the registry.
func (x *FunctionsRegistry) RegisterDef(def FunctionDef) {
	x.Lock()
	defer x.Unlock()
	if x.functions == nil {
		x.functions = make(map[string]FunctionDef)
		x.functionNames = make([]string, 0)
	}
	if _, ok := x.functions[def.Name]; !ok {
		x.functionNames = append(x.functionNames, def.Name)
	}
	x.functions[def.Name] = def
}

// UnRegister removes the function from the registry
// UnRegister removes a function from the registry by name.
func (x *FunctionsRegistry) UnRegister(functionName string) {
	x.Lock()
	defer x.Unlock()
	if x.functions != nil {
		if _, ok := x.functions[functionName]; ok {
			delete(x.functions, functionName)
			// remove from slice
			for i, name := range x.functionNames {
				if name == functionName {
					x.functionNames = append(x.functionNames[:i], x.functionNames[i+1:]...)
					break
				}
			}
		}
	}
}

// Get the function from the registry
// Get retrieves a function from the registry by name.
func (x *FunctionsRegistry) Get(functionName string) (func(ctx types.RuleContext, msg types.RuleMsg), bool) {
	x.RLock()
	defer x.RUnlock()
	if x.functions == nil {
		return nil, false
	}
	f, ok := x.functions[functionName]
	if ok {
		return f.F, true
	}
	return nil, false
}

// List returns a list of all registered function definitions
// List returns a list of all registered function definitions.
func (x *FunctionsRegistry) List() []FunctionDef {
	x.RLock()
	defer x.RUnlock()
	var defs = make([]FunctionDef, 0, len(x.functions))
	for _, name := range x.functionNames {
		if v, ok := x.functions[name]; ok {
			defs = append(defs, v)
		}
	}
	return defs
}

// Names returns a list of all registered function names
// Names returns a list of all registered function names.
func (x *FunctionsRegistry) Names() []string {
	x.RLock()
	defer x.RUnlock()
	var keys = make([]string, len(x.functionNames))
	copy(keys, x.functionNames)
	return keys
}

// FunctionsNodeConfiguration FunctionsNode configuration structure
// FunctionsNodeConfiguration defines the configuration structure for the FunctionsNode component.
type FunctionsNodeConfiguration struct {
	// FunctionName is the name of the registered function to call.
	// Supports ${metadata.key} and ${msg.key} substitution.
	FunctionName string `json:"functionName" label:"Function Name" desc:"Registered function name. Supports ${metadata.key} and ${msg.key} substitution" required:"true"`
	// Param is the input parameter for the function. If empty, message payload is used.
	Param string `json:"param" label:"Parameter" desc:"Function input parameter. Supports ${metadata.key} and ${msg.key}. If empty, uses message payload"`
}

// FunctionsNode calls the action component of a registered custom function via the function name
// FunctionsNode is an action component that invokes registered custom functions by name.
//
// Core algorithm:
// Core Algorithm:
// 1. Resolve function name (static or dynamic variable substitution)
// 2. Look up function in the global registry
// 3. Call functions and let functions handle routing - Invoke function and let function handle routing
//
// Function name resolution - Function name resolution:
//   - Static names: used directly
//   - Dynamic names: supports variable substitution including nodeId.metadata.key msg.key metadata.key cross-node access
type FunctionsNode struct {
	// Config defines the node configuration
	// Config holds the node configuration including function name specification
	Config FunctionsNodeConfiguration

	// functionNameTemplate is a template used to parse dynamic function names
	// functionNameTemplate template for resolving dynamic function names
	functionNameTemplate el.Template
	// paramTemplate parameter template
	// paramTemplate template for resolving function parameters
	paramTemplate el.Template
}

// Type returns the component type
// Type returns the component type identifier.
func (x *FunctionsNode) Type() string {
	return "functions"
}

// New creates an instance
// New creates a new instance.
func (x *FunctionsNode) New() types.Node {
	return &FunctionsNode{Config: FunctionsNodeConfiguration{
		FunctionName: "test",
	}}
}

// Init initializes the component
// Init initializes the component.
func (x *FunctionsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	// Initialize the function-name template
	// Initialize function name template
	x.functionNameTemplate, err = el.NewTemplate(x.Config.FunctionName)
	if err != nil {
		return fmt.Errorf("failed to create function name template: %w", err)
	}
	// Initialize parameter templates
	// Initialize parameter template
	if x.Config.Param != "" {
		x.paramTemplate, err = el.NewTemplate(x.Config.Param)
		if err != nil {
			return fmt.Errorf("failed to create param template: %w", err)
		}
	}
	return nil
}

// OnMsg processes messages and calls specified functions
// OnMsg processes incoming messages by invoking the specified function from the registry.
func (x *FunctionsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	funcName := x.getFunctionName(ctx, msg)
	if f, ok := Functions.Get(funcName); ok {
		// Handle parameter
		if x.paramTemplate != nil {
			evn := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
			param := x.paramTemplate.ExecuteAsString(evn)
			msg.SetData(param)
		}
		// Calling the function
		f(ctx, msg)
	} else {
		ctx.TellFailure(msg, fmt.Errorf("can not found the function=%s", funcName))
	}
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *FunctionsNode) Destroy() {
	// No resources to clean
	// No resources to clean up
}

// getFunctionName parses function names, handles static and dynamic situations, and supports cross-node values
// getFunctionName resolves the function name, handling both static and dynamic cases with cross-node access support.
func (x *FunctionsNode) getFunctionName(ctx types.RuleContext, msg types.RuleMsg) string {
	if x.functionNameTemplate != nil {
		// Execute template
		return x.functionNameTemplate.ExecuteAsString(base.NodeUtils.GetEvnAndMetadata(ctx, msg))
	}
	return x.Config.FunctionName
}

// Desc returns the component description
func (x *FunctionsNode) Desc() string {
	return "Invoke a registered custom function by name. functionName supports ${metadata.key} and ${msg.key} substitution. Routes to Success/Failure"
}
