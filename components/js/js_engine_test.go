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
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

const (
	jsFuncFilter          = "Filter"
	jsFuncTransform       = "Transform"
	jsFuncGetValue        = "GetValue"
	jsFuncCallGolangFunc1 = "CallGolangFunc1"
	jsFuncCallGolangFunc2 = "CallGolangFunc2"
	jsFuncCallStructFunc  = "CallStructFunc"
	jsFuncAdd3            = "add3"
	jsFuncTimeout         = "timeoutFunc"
	jsFuncAdd2            = "add2"

	msgAa            = "aa"
	msgBb            = "bb"
	globalNameLala   = "lala"
	metaTestKey      = "test"
	returnFromGoVal  = "returnFromGo"
	executionTimeout = "execution timeout"
	user01           = "user01"
	resultUser01     = "result:user01"
)

func TestJsEngine(t *testing.T) {
	var jsScript = `
	function Filter(msg, metadata, msgType) {
	    function result(){
			return 'aa'
		}
		return msg==result()
	}
	function Transform(msg, metadata, msgType) {
		var msg ={}
		msg.isNumber=isNumber(5)
		msg.today=utilsFunc.dateFormat(new Date(),'yyyyMMdd')
		msg.add=add2(5,3)
		msg.add2=add2(5,3)
		return msg
	}
	function GetValue(msg, metadata, msgType) {
		return global.name 
	}
	function CallGolangFunc1(msg, metadata, msgType) {
			return add(1,5) 
	}
	function CallGolangFunc2(msg, metadata, msgType) {
			return handleMsg(msg, metadata, msgType) 
	}
	function CallStructFunc() {
		return tool.Query("user01")
	}
	`
	start := time.Now()
	config := types.NewConfig()
	//注册全局配置参数
	config.Properties.PutValue("name", "lala")
	//注册自定义函数
	config.RegisterUdf("add", func(a, b int) int {
		return a + b
	})
	config.RegisterUdf("handleMsg", func(msg map[string]string, metadata map[string]string, msgType string) map[string]string {
		msg["returnFromGo"] = "returnFromGo"
		return msg
	})

	//注册原生JS脚本
	//使用 isNumber(xx)
	config.RegisterUdf("isNumberScript", `function isNumber(value){
			return typeof value === "number";
		}
	`)
	// 使用：utilsFunc.dateFormat(new Date(), "yyyyMMddhh")
	config.RegisterUdf(
		"utilsFunScript", types.Script{
			Type: types.Js,
			Content: `var utilsFunc={
						dateFormat:function(date,fmt){
						   var o = {
							 "M+": date.getMonth() + 1,
							 /*月份*/ "d+": date.getDate(),
							 /*日*/ "h+": date.getHours(),
							 /*小时*/ "m+": date.getMinutes(),
							 /*分*/ "s+": date.getSeconds(),
							 /*秒*/ "q+": Math.floor((date.getMonth() + 3) / 3),
							 /*季度*/ S: date.getMilliseconds() /*毫秒*/,
						   };
						   fmt = fmt.replace(/(y+)/, function(match, group) {
							 return (date.getFullYear() + "").substr(4 - group.length); 
						   });
						   for (var k in o) {
							 fmt = fmt.replace(new RegExp("(" + k + ")"), function(match, group) { 
							   return group.length == 1 ? o[k] : ("00" + o[k]).substr(("" + o[k]).length); 
							 });
						   }
						   return fmt;
						},
						isArray:function(arg){
						  if (typeof Array.isArray === 'undefined') {
							return Object.prototype.toString.call(arg) === '[object Array]'
							}
							return Array.isArray(arg)
						},
						isObject: function(value){
							if (!data || this.isArray(data)) {
							  return false;
							}
							return data instanceof Object;
						},
						isNumber: function(value){
							return typeof value === "number";
						},
					}
				`,
		},
	)
	config.RegisterUdf(
		"add2", types.Script{
			Type: types.Js,
			Content: func(a, b int) int {
				return a + b
			},
		})
	config.RegisterUdf(
		"timeoutFunc", types.Script{
			Type: types.Js,
			Content: `
			function sleep(ms) {
				var start = Date.now();
				while (Date.now() < start + ms);
			}
			function timeoutFunc(value){
			  sleep(3000); // 休眠3秒
			}
		`,
		},
	)
	p, _ := goja.Compile("add3", `function add3(){return 9}`, true)
	config.RegisterUdf(
		"add3", types.Script{
			Type:    types.Js,
			Content: p,
		},
	)
	//注册结构体所有导出函数
	var tool = &ToolTest{}
	config.RegisterUdf(
		"tool", tool,
	)
	baseJsEngine, err := NewGojaJsEngine(config, jsScript, nil)
	assert.NotNil(t, baseJsEngine)
	assert.Nil(t, err)
	baseJsEngine.Stop() // Stop the engine created just for validation

	// Recreate engine with params for actual tests
	jsEngine, err := NewGojaJsEngine(config, jsScript, map[string]interface{}{"username": globalNameLala})
	assert.NotNil(t, jsEngine)
	assert.Nil(t, err)
	defer jsEngine.Stop()

	jsEngine.config.Logger.Printf("Setup time: %s", time.Since(start))

	metadata := map[string]interface{}{
		metaTestKey: "test", // Using const for key "aa"
	}

	t.Run("Filter_MsgBb_ReturnsFalse_And_Transform_And_Add3", func(t *testing.T) {
		response, err := jsEngine.Execute(nil, jsFuncFilter, msgBb, metadata, msgAa)
		assert.Nil(t, err)
		assert.Equal(t, false, response.(bool))

		response, err = jsEngine.Execute(nil, jsFuncTransform, msgBb, metadata, msgAa)
		assert.Nil(t, err)
		r, ok := response.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, int64(8), r["add"])
		assert.Equal(t, int64(8), r[jsFuncAdd2])
		assert.Equal(t, true, r["isNumber"])
		assert.Equal(t, time.Now().Format("20060102"), r["today"])

		response, err = jsEngine.Execute(nil, jsFuncAdd3, msgBb, metadata, msgAa)
		assert.Nil(t, err)
		assert.Equal(t, int64(9), response)
	})

	t.Run("GetValue_ReturnsGlobalName", func(t *testing.T) {
		response, err := jsEngine.Execute(nil, jsFuncGetValue, msgBb, metadata, msgAa)
		assert.Nil(t, err)
		assert.Equal(t, globalNameLala, response.(string))
	})

	t.Run("CallGolangFunc1_ReturnsSum", func(t *testing.T) {
		response, err := jsEngine.Execute(nil, jsFuncCallGolangFunc1, msgBb, metadata, msgAa)
		assert.Nil(t, err)
		assert.Equal(t, int64(6), response.(int64))
	})

	t.Run("CallGolangFunc2_And_Timeout_And_UnknownFunc", func(t *testing.T) {
		response, err := jsEngine.Execute(nil, jsFuncCallGolangFunc2, metadata, metadata, "testMsgType")
		assert.Nil(t, err)
		assert.Equal(t, returnFromGoVal, response.(map[string]string)["returnFromGo"])

		response, err = jsEngine.Execute(nil, jsFuncTimeout, metadata, metadata, "testMsgType")
		assert.NotNil(t, err) // Expecting an error
		assert.Equal(t, true, strings.HasPrefix(err.Error(), executionTimeout))

		response, err = jsEngine.Execute(nil, msgAa, metadata, metadata, "testMsgType") // Calling undefined function 'aa'
		assert.NotNil(t, err)
	})

	t.Run("Filter_MsgAa_ReturnsTrue_And_CallStructFunc", func(t *testing.T) {
		response, err := jsEngine.Execute(nil, jsFuncFilter, msgAa, metadata, msgAa)
		assert.Nil(t, err)
		assert.Equal(t, true, response.(bool))

		response, err = jsEngine.Execute(nil, jsFuncCallStructFunc, user01)
		assert.Nil(t, err)
		assert.Equal(t, resultUser01, response.(string))
	})

	t.Run("Add2_DirectCall", func(t *testing.T) {
		response, err := jsEngine.Execute(nil, jsFuncAdd2, 5, 4)
		assert.Nil(t, err)
		assert.Equal(t, int64(9), response.(int64))
	})

	t.Run("MultipleCalls_JsVmPool", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			response, err := jsEngine.Execute(nil, jsFuncAdd2, i, i+1)
			assert.Nil(t, err)
			assert.Equal(t, int64(i*2+1), response.(int64))
		}
	})

	// Final check for NewGojaJsEngine with nil params after all tests
	_, err = NewGojaJsEngine(config, jsScript, nil)
	assert.Nil(t, err) // Expecting this to succeed as per original logic
}

type ToolTest struct {
}

func (t *ToolTest) Query(id string) string {
	return "result:" + id
}
func (t *ToolTest) Delete(id string) bool {
	return true
}
