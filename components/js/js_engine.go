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

package js

import (
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"github.com/rulego/rulego/api/types"
	"strings"
	"sync"
	"time"
)

const (
	//GlobalKey  global properties key,call them through the global.xx method
	GlobalKey = "global"
)

// GojaJsEngine goja js engine
type GojaJsEngine struct {
	vmPool            sync.Pool
	config            types.Config
	jsScript          *goja.Program
	jsUdfProgramCache map[string]*goja.Program
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
			return jsEngine.NewVm(config, fromVars)
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
	vars := make(map[string]interface{})
	if fromVars != nil {
		for k, v := range fromVars {
			vars[k] = v
		}
	}
	if len(config.Properties.Values()) != 0 {
		////Add global properties to the JavaScript runtime and call them through the global.xx method
		vars[GlobalKey] = config.Properties.Values()
	}
	//Add global custom functions to the JavaScript runtime
	for k, v := range config.Udf {
		var err error
		if _, ok := v.(string); ok {
			if p, ok := g.jsUdfProgramCache[k]; ok {
				_, err = vm.RunProgram(p)
			}
		} else if script, scriptOk := v.(types.Script); scriptOk {
			if script.Type == types.Js || script.Type == "" {
				// parse  JS script
				if _, ok := script.Content.(string); ok {
					if p, ok := g.jsUdfProgramCache[k]; ok {
						_, err = vm.RunProgram(p)
					}
				} else if _, ok := script.Content.(*goja.Program); ok {
					if p, ok := g.jsUdfProgramCache[k]; ok {
						_, err = vm.RunProgram(p)
					}
				} else {
					funcName := strings.Replace(k, types.Js+types.ScriptFuncSeparator, "", 1)
					vars[funcName] = vm.ToValue(script.Content)
				}
			}
		} else {
			// parse go func
			vars[k] = vm.ToValue(v)
		}
		if err != nil {
			config.Logger.Printf("parse js script=" + k + " error,err:" + err.Error())
		}
	}
	for k, v := range vars {
		if err := vm.Set(k, v); err != nil {
			config.Logger.Printf("set variable error,err:" + err.Error())
			panic(errors.New("set variable error,err:" + err.Error()))
		}
	}

	state := g.setTimeout(vm)

	_, err := vm.RunProgram(g.jsScript)
	//If there is no timeout, state=0; otherwise, state=-2
	closeStateChan(state)

	if err != nil {
		config.Logger.Printf("js vm error,err:" + err.Error())
		panic(errors.New("js vm error,err:" + err.Error()))
	}
	return vm
}

// Execute Execute JavaScript script
func (g *GojaJsEngine) Execute(functionName string, argumentList ...interface{}) (out interface{}, err error) {
	defer func() {
		if caught := recover(); caught != nil {
			err = fmt.Errorf("%s", caught)
		}
	}()

	vm := g.vmPool.Get().(*goja.Runtime)

	state := g.setTimeout(vm)

	f, ok := goja.AssertFunction(vm.Get(functionName))
	if !ok {
		return nil, errors.New(functionName + " is not a function")
	}
	var params []goja.Value
	for _, v := range argumentList {
		params = append(params, vm.ToValue(v))
	}
	res, err := f(goja.Undefined(), params...)
	//If there is no timeout, state=0; otherwise, state=-2
	closeStateChan(state)
	//Put back to the pool
	g.vmPool.Put(vm)
	if err != nil {
		return nil, err
	}
	return res.Export(), err
}

func (g *GojaJsEngine) Stop() {
}

// setTimeout if timeout interrupt the js script execution
func (g *GojaJsEngine) setTimeout(vm *goja.Runtime) chan int {
	state := make(chan int, 1)
	state <- 0
	time.AfterFunc(g.config.ScriptMaxExecutionTime, func() {
		if <-state == 0 {
			state <- 2
			vm.Interrupt("execution timeout")
		}
	})
	return state
}

func closeStateChan(state chan int) {
	if <-state == 0 {
		state <- 1
	}
	close(state)
}
