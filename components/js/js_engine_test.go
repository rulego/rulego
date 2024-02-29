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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"strings"
	"sync"
	"testing"
	"time"
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

	jsEngine, err := NewGojaJsEngine(config, jsScript, nil)
	assert.NotNil(t, jsEngine)
	assert.Nil(t, err)

	jsEngine, err = NewGojaJsEngine(config, jsScript, map[string]interface{}{"username": "lala"})
	defer jsEngine.Stop()
	jsEngine.config.Logger.Printf("用时1：%s", time.Since(start))
	var group sync.WaitGroup
	group.Add(10)
	i := 0
	for i < 10 {
		testExecuteJs(t, jsEngine, i, &group)
		i++
	}
	group.Wait()

	_, err = NewGojaJsEngine(config, jsScript, nil)
}

func testExecuteJs(t *testing.T, jsEngine *GojaJsEngine, index int, group *sync.WaitGroup) {
	metadata := map[string]interface{}{
		"aa": "test",
	}
	start := time.Now()
	var response interface{}
	var err error
	if index == 3 || index == 5 {
		response, err = jsEngine.Execute("Filter", "bb", metadata, "aa")
		assert.Nil(t, err)
		assert.Equal(t, false, response.(bool))
		response, err = jsEngine.Execute("Transform", "bb", metadata, "aa")
		r, ok := response.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, int64(8), r["add"])
		assert.Equal(t, int64(8), r["add2"])
		assert.Equal(t, true, r["isNumber"])
		assert.Equal(t, time.Now().Format("20060102"), r["today"])
	} else if index == 6 {
		response, err = jsEngine.Execute("GetValue", "bb", metadata, "aa")
		assert.Nil(t, err)
		assert.Equal(t, "lala", response.(string))
	} else if index == 7 {
		response, err = jsEngine.Execute("CallGolangFunc1", "bb", metadata, "aa")
		assert.Nil(t, err)
		assert.Equal(t, int64(6), response.(int64))
	} else if index == 8 {
		response, err = jsEngine.Execute("CallGolangFunc2", metadata, metadata, "testMsgType")
		assert.Nil(t, err)
		assert.Equal(t, "returnFromGo", response.(map[string]string)["returnFromGo"])

		response, err = jsEngine.Execute("timeoutFunc", metadata, metadata, "testMsgType")
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "execution timeout"))

		response, err = jsEngine.Execute("aa", metadata, metadata, "testMsgType")
		assert.NotNil(t, err)

	} else {
		response, err = jsEngine.Execute("Filter", "aa", metadata, "aa")
		assert.Nil(t, err)
		assert.Equal(t, true, response.(bool))
	}
	response, err = jsEngine.Execute("add2", 5, 4)
	assert.Nil(t, err)
	assert.Equal(t, int64(9), response.(int64))

	group.Done()
	jsEngine.config.Logger.Printf("index:%d,响应:%s,用时：%s", index, response, time.Since(start))

}
