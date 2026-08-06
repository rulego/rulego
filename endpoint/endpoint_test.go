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
	endpointStr := strings.Replace(string(endpointBuf), "9090", "9081", -1)

	ruleDsl, err := os.ReadFile(testRulesFolder + "/filter_node.json")

	_, err = engine.New("test01", ruleDsl)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := NewFromDsl([]byte(endpointStr), endpoint.DynamicEndpointOptions.WithConfig(config),
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
	_ = json.Unmarshal([]byte(endpointStr), &def)
	v, _ := json.Marshal(def)
	dsl := strings.Replace(string(v), " ", "", -1)

	assert.Equal(t, dsl, strings.Replace(string(ep.DSL()), " ", "", -1))
	assert.True(t, reflect.DeepEqual(def, ep.Definition()))
	sendMsg(t, "http://127.0.0.1:9081/api/v1/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))
	time.Sleep(time.Millisecond * 2000)

	ep.Destroy()
}

// TestDynamicEndpointStartRetryDestroyRace guards the cancelStartRetry fix:
// while the background Start retry is active, Destroy must wait for the retry
// goroutine to exit before returning, so the endpoint instance is not
// destroyed out from under an in-flight Start().
func TestDynamicEndpointStartRetryDestroyRace(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	// Point at an unreachable port so Start() fails and arms the background retry.
	dsl := `{"id":"ep_retry_race","type":"endpoint/mqtt","configuration":{"server":"127.0.0.1:1","qos":0}}`
	ep, err := NewFromDsl([]byte(dsl), endpoint.DynamicEndpointOptions.WithConfig(config))
	if err != nil {
		t.Fatalf("NewFromDsl: %v", err)
	}

	// Wrapped Start swallows the failure and arms the background retry.
	if err := ep.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Confirm the retry is active.
	if got := ep.ConnectionStatus().Status; got != types.StatusReconnecting {
		t.Fatalf("status=%s, want reconnecting", got)
	}

	// Destroy during active retry must return promptly (not block on the retry).
	destroyDone := make(chan struct{})
	go func() { ep.Destroy(); close(destroyDone) }()
	select {
	case <-destroyDone:
		// returned without blocking
	case <-time.After(5 * time.Second):
		t.Fatal("Destroy blocked for >5s while retry was active")
	}

	// After Destroy, status must no longer report an active retry.
	if got := ep.ConnectionStatus().Status; got == types.StatusReconnecting {
		t.Fatalf("status still reconnecting after Destroy")
	}
}
