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
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/test/assert"
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
  	`
	start := time.Now()
	config := rulego.NewConfig()
	jsEngine := NewGojaJsEngine(config, jsScript, nil)
	jsEngine.config.Logger.Printf("用时1：%s", time.Since(start))
	var group sync.WaitGroup
	group.Add(10)
	i := 0
	for i < 10 {
		testExecuteJs(t, jsEngine, i, &group)
		i++
	}
	group.Wait()

	jsEngine.config.Logger.Printf("==========================")
	var group2 sync.WaitGroup
	group2.Add(10)
	i = 0
	for i < 10 {
		go testExecuteJs(t, jsEngine, i, &group2)
		i++
	}
	group2.Wait()
}
func testExecuteJs(t *testing.T, jsEngine *GojaJsEngine, index int, group *sync.WaitGroup) {
	metadata := map[string]interface{}{
		"aa": "test",
	}
	start := time.Now()
	var response interface{}
	var err error
	if index == 5 || index == 7 {
		response, err = jsEngine.Execute("Filter", "bb", metadata, "aa")
		assert.Nil(t, err)
		assert.Equal(t, false, response.(bool))
	} else {
		response, err = jsEngine.Execute("Filter", "aa", metadata, "aa")
		assert.Nil(t, err)
		assert.Equal(t, true, response.(bool))
	}

	group.Done()
	jsEngine.config.Logger.Printf("index:%d,响应:%s,用时：%s", index, response, time.Since(start))

}
