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

package endpoint

import (
	"context"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testEndpointsFolder = "../testdata/endpoint"
var testRulesFolder = "../testdata/rule"

func TestDynamicEndpoint(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		//assert.Equal(t, "ok", msg.Data)
	})
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", types.NewMetadata(), "{\"name\":\"lala\"}")

	endpointBuf, err := os.ReadFile(testEndpointsFolder + "/http_01.json")
	if err != nil {
		t.Fatal(err)
	}
	ruleDsl, err := os.ReadFile(testRulesFolder + "/filter_node.json")

	_, err = engine.New("test01", ruleDsl)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := NewFromDsl(endpointBuf, endpoint.DynamicEndpointOptions.WithConfig(config),
		endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithContextFunc(func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
			return context.Background()
		})))

	if err != nil {
		t.Fatal(err)
	}

	err = ep.Start()
	time.Sleep(time.Millisecond * 200)

	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		assert.Equal(t, "aa", router.Definition().AdditionalInfo["aa"])
		return true
	})

	var def types.EndpointDsl
	_ = json.Unmarshal(endpointBuf, &def)
	v, _ := json.Marshal(def)
	dsl := strings.Replace(string(v), " ", "", -1)

	assert.Equal(t, dsl, strings.Replace(string(ep.DSL()), " ", "", -1))
	assert.True(t, reflect.DeepEqual(def, ep.Definition()))
	sendMsg(t, "http://127.0.0.1:9090/api/v1/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))
	time.Sleep(time.Millisecond * 2000)
	
	ep.Destroy()
}
