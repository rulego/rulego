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

// Package js provides JavaScript execution capabilities for the RuleGo rule engine.
//
// This package implements a JavaScript engine using the goja library, allowing
// for the execution of JavaScript code within the rule engine. It includes
// functionality for creating and managing JavaScript virtual machines,
// compiling and caching JavaScript programs, and executing JavaScript code
// with access to global variables and user-defined functions.
//
// Key components:
// - GojaJsEngine: The main struct representing the JavaScript engine.
// - NewGojaJsEngine: Function to create a new instance of the JavaScript engine.
// - PreCompileJs: Method to precompile user-defined JavaScript functions.
//
// The package supports features such as:
// - Pooling of JavaScript VMs for efficient reuse
// - Precompilation of JavaScript code for improved performance
// - Integration with the RuleGo configuration system
// - Access to global variables and functions within JavaScript code
//
// This package is crucial for components that require JavaScript execution,
// such as the JsTransformNode and JsFilterNode in the action package.
package js

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/rulego/rulego/api/types"
)

const (
	//GlobalKey  global properties key,call them through the global.xx method
	GlobalKey = "global"
	CtxKey    = "$ctx"
)

// GojaJsEngine goja js engine
type GojaJsEngine struct {
	vmPool            sync.Pool
	config            types.Config
	jsScript          *goja.Program
	jsUdfProgramCache map[string]*goja.Program
}

// gojaVmSlot 池化的 VM 槽位；超时定时器不进槽位，避免迟到回调把中断锁进空闲 VM
type gojaVmSlot struct {
	vm *goja.Runtime
}

// NewGojaJsEngine Create a new instance of the JavaScript engine
func NewGojaJsEngine(config types.Config, jsScript string, fromVars map[string]interface{}) (*GojaJsEngine, error) {
	program, err := goja.Compile("", jsScript, true)
	if err != nil {
		return nil, err
	}
	jsEngine := &GojaJsEngine{
		config:   config,
		jsScript: program,
	}
	if err = jsEngine.PreCompileJs(config); err != nil {
		return nil, err
	}
	jsEngine.vmPool = sync.Pool{
		New: func() interface{} {
			vm := jsEngine.NewVm(config, fromVars)
			return &gojaVmSlot{vm: vm}
		},
	}
	return jsEngine, nil
}

// PreCompileJs Precompiled UDF JavaScript file
func (g *GojaJsEngine) PreCompileJs(config types.Config) error {
	var jsUdfProgramCache = make(map[string]*goja.Program)
	for k, v := range config.Udf {
		if jsFuncStr, ok := v.(string); ok {
			if p, err := goja.Compile(k, jsFuncStr, true); err != nil {
				return err
			} else {
				jsUdfProgramCache[k] = p
			}
		} else if script, scriptOk := v.(types.Script); scriptOk {
			if script.Type == types.Js || script.Type == "" {
				if c, ok := script.Content.(string); ok {
					if p, err := goja.Compile(k, c, true); err != nil {
						return err
					} else {
						jsUdfProgramCache[k] = p
					}
				} else if p, ok := script.Content.(*goja.Program); ok {
					jsUdfProgramCache[k] = p
				}
			}
		}
	}
	g.jsUdfProgramCache = jsUdfProgramCache

	return nil
}

// NewVm new a js VM
func (g *GojaJsEngine) NewVm(config types.Config, fromVars map[string]interface{}) *goja.Runtime {
	vm := goja.New()

	// Set fromVars directly
	if fromVars != nil {
		for k, v := range fromVars {
			if err := vm.Set(k, v); err != nil {
				config.Logger.Errorf("set fromVar %s error: %s", k, err.Error())
			}
		}
	}

	// Set global properties directly
	if len(config.Properties.Values()) != 0 {
		if err := vm.Set(GlobalKey, config.Properties.Values()); err != nil {
			config.Logger.Errorf("set global properties error: %s", err.Error())
		}
	}

	// Process UDF functions
	for k, v := range config.Udf {
		var err error
		if _, ok := v.(string); ok {
			// JS string - run precompiled program
			if p, exists := g.jsUdfProgramCache[k]; exists {
				_, err = vm.RunProgram(p)
			}
		} else if script, scriptOk := v.(types.Script); scriptOk {
			if script.Type == types.Js || script.Type == types.AllScript {
				if _, ok := script.Content.(string); ok {
					// JS string content - run precompiled program
					if p, exists := g.jsUdfProgramCache[k]; exists {
						_, err = vm.RunProgram(p)
					}
				} else if _, ok := script.Content.(*goja.Program); ok {
					// Precompiled program - run it
					if p, exists := g.jsUdfProgramCache[k]; exists {
						_, err = vm.RunProgram(p)
					}
				} else if script.Content != nil {
					// Go function in script wrapper
					funcName := strings.Replace(k, types.Js+types.ScriptFuncSeparator, "", 1)
					err = vm.Set(funcName, script.Content)
				}
			}
		} else {
			// Direct Go function
			err = vm.Set(k, v)
		}

		if err != nil {
			config.Logger.Errorf("parse js script=%s error: %s", k, err.Error())
		}
	}

	// Run main script with timeout
	timer := g.startTimeout(vm)
	_, err := vm.RunProgram(g.jsScript)
	g.stopTimeout(timer)

	if err != nil {
		config.Logger.Errorf("js vm error: %s", err.Error())
	}
	return vm
}

// Execute Execute JavaScript script
func (g *GojaJsEngine) Execute(ctx types.RuleContext, functionName string, argumentList ...interface{}) (out interface{}, err error) {
	defer func() {
		if caught := recover(); caught != nil {
			err = fmt.Errorf("%s", caught)
		}
	}()

	slot := g.vmPool.Get().(*gojaVmSlot)
	vm := slot.vm
	// 归还前清掉 $ctx：池化 VM 会长期持有上一条消息的 RuleContext 引用，
	// 阻碍大消息 GC；ctx 为 nil 的执行读到 null 报错优于读到上一条的脏值
	defer func() {
		_ = vm.Set(CtxKey, nil)
		g.vmPool.Put(slot)
	}()

	// Only set context if provided to avoid nil overhead
	if ctx != nil {
		vm.Set(CtxKey, ctx)
	}

	// 清除上一轮迟到定时器残留的中断，否则本次执行立刻超时失败
	vm.ClearInterrupt()

	timer := g.startTimeout(vm)
	defer g.stopTimeout(timer)

	// Get function
	f, ok := goja.AssertFunction(vm.Get(functionName))
	if !ok {
		return nil, errors.New(functionName + " is not a function")
	}

	// Optimized parameter conversion - pre-allocate slice
	var params []goja.Value
	if len(argumentList) > 0 {
		params = make([]goja.Value, len(argumentList))
		for i, v := range argumentList {
			params[i] = vm.ToValue(v)
		}
	}

	// Execute function
	res, err := f(goja.Undefined(), params...)
	if err != nil {
		return nil, err
	}
	return res.Export(), nil
}

func (g *GojaJsEngine) Stop() {
}

// startTimeout starts a timeout for JS script execution using time.AfterFunc
// Returns nil if timeout is not configured
func (g *GojaJsEngine) startTimeout(vm *goja.Runtime) *time.Timer {
	// Skip timeout if not configured
	if g.config.ScriptMaxExecutionTime <= 0 {
		return nil
	}

	// Use time.AfterFunc to avoid creating goroutines
	// This is more efficient and prevents goroutine leaks
	return time.AfterFunc(g.config.ScriptMaxExecutionTime, func() {
		vm.Interrupt("execution timeout")
	})
}

// stopTimeout stops the timeout timer
func (g *GojaJsEngine) stopTimeout(timer *time.Timer) {
	if timer != nil {
		timer.Stop()
	}
}

// gojaLinePosRe 匹配主脚本编译错误的行列号片段 "(anonymous): Line N:M"。
// 锚定 (anonymous) 是为了与 UDF 编译错误区分（后者带 UDF key 文件名，
// 用户脚本行号本就正确，不应平移）；运行期栈是 ":N:M" 后缀格式，同样不会命中
var gojaLinePosRe = regexp.MustCompile(`\(anonymous\): Line (\d+):(\d+)`)

// ShiftErrorLine 把编译错误中的行号平移 lines 行，用于脚本被包装进函数模板后
// 把 goja 报的行号换算回用户脚本坐标。平移后小于 1 或不含行列号时原样返回。
func ShiftErrorLine(err error, lines int) error {
	if err == nil || lines == 0 {
		return err
	}
	msg := err.Error()
	m := gojaLinePosRe.FindStringSubmatch(msg)
	if m == nil {
		return err
	}
	line, convErr := strconv.Atoi(m[1])
	if convErr != nil || line+lines < 1 {
		return err
	}
	return errors.New(gojaLinePosRe.ReplaceAllString(msg, fmt.Sprintf("Line %d:$2", line+lines)))
}
