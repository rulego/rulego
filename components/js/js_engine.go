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
	"sync"
	"time"
)

func closeStateChan(state chan int) {
	// 超过时间也会执行到这里
	//如果没有超过时间，那么取出的是0，否则取出的是2
	if <-state == 0 {
		state <- 1
	}
	close(state)
}

//GojaJsEngine goja js引擎
type GojaJsEngine struct {
	vmPool   sync.Pool
	jsScript string
	config   types.Config
}

//NewGojaJsEngine 创建一个新的js引擎实例
func NewGojaJsEngine(config types.Config, jsScript string, vars map[string]interface{}) *GojaJsEngine {
	//defaultAntsPool, _ := ants.NewPool(config.MaxTaskPool)
	jsEngine := &GojaJsEngine{
		vmPool: sync.Pool{
			New: func() interface{} {
				//atomic.AddInt64(&vmNum, 1)
				//config.Logger.Printf("create new js vm%d", vmNum)
				vm := goja.New()
				if vars == nil {
					vars = make(map[string]interface{})
				}
				if len(config.Properties.Values()) != 0 {
					//增加全局Properties 到js运行时
					vars["global"] = config.Properties.Values()
				}
				//增加全局自定义函数到js运行时
				for k, v := range config.Udf {
					var err error
					if jsFuncStr, ok := v.(string); ok {
						// parse  JS script
						_, err = vm.RunString(jsFuncStr)
					} else if script, scriptOk := v.(types.Script); scriptOk {
						if script.Type == types.Js || script.Type == "" {
							// parse  JS script
							_, err = vm.RunString(script.Content)
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
						//return nil, errors.New("set variable error,err:" + err.Error())
						panic(errors.New("set variable error,err:" + err.Error()))
					}
				}

				state := make(chan int, 1)
				state <- 0
				time.AfterFunc(config.ScriptMaxExecutionTime, func() {
					if <-state == 0 {
						state <- 2
						vm.Interrupt("execution timeout")
					}
				})

				_, err := vm.RunString(jsScript)
				// 超过时间也会执行到这里，如果没有超过时间，那么取出的是0，否则取出的是
				closeStateChan(state)

				if err != nil {
					//return nil, errors.New("js vm error,err:" + err.Error())
					config.Logger.Printf("js vm error,err:" + err.Error())
					panic(errors.New("js vm error,err:" + err.Error()))
				}

				return vm
			},
		},
		jsScript: jsScript,
		config:   config,
	}

	return jsEngine
}

func (g *GojaJsEngine) Execute(functionName string, argumentList ...interface{}) (out interface{}, err error) {
	defer func() {
		if caught := recover(); caught != nil {
			err = fmt.Errorf("%s", caught)
		}
	}()

	vm := g.vmPool.Get().(*goja.Runtime)
	state := make(chan int, 1)
	state <- 0
	time.AfterFunc(g.config.ScriptMaxExecutionTime, func() {
		if <-state == 0 {
			state <- 2
			vm.Interrupt("execution timeout")
		}
	})

	f, ok := goja.AssertFunction(vm.Get(functionName))
	if !ok {
		return nil, errors.New(functionName + "is not a function")
	}
	var params []goja.Value
	for _, v := range argumentList {
		params = append(params, vm.ToValue(v))
	}
	res, err := f(goja.Undefined(), params...)

	// 超过时间也会执行到这里，如果没有超过时间，那么取出的是0，否则取出的是2
	closeStateChan(state)
	//放回对象池
	g.vmPool.Put(vm)
	if err != nil {
		return nil, err
	}
	return res.Export(), err
}

func (g *GojaJsEngine) Stop() {
}
