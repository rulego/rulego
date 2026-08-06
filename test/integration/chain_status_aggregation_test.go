/*
 * Copyright 2024 The RuleGo Authors.
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

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
)

// waitFor polls cond until it returns true or the timeout expires; calls t.Fatal on timeout.
func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s not reached within %s", what, timeout)
}

// TestChainEndpointStatusAggregation verifies chain-level connection-status aggregation:
// an mqtt endpoint declared in metadata.endpoints is registered into the chain resource registry via EndpointAspect,
// and RuleChainCtx.Statuses() aggregates its status and flips live on broker failure.
func TestChainEndpointStatusAggregation(t *testing.T) {
	broker, err := test.NewMqttBroker("127.0.0.1:0")
	if err != nil {
		t.Fatalf("start broker: %v", err)
	}
	defer broker.Close()
	addr := broker.Addr()

	chain := `{"ruleChain":{"id":"chain_status_agg","name":"status aggregation","root":true},
"metadata":{
  "endpoints":[{"id":"ep1","type":"endpoint/mqtt","configuration":{"server":"__ADDR__","qos":0},
    "routers":[{"from":{"path":"#"},"to":{"path":"chain:chain_status_agg"}}]}],
  "nodes":[{"id":"n1","type":"log","configuration":{"jsScript":"return msg;"}}]
}}`
	chain = strings.ReplaceAll(chain, "__ADDR__", addr)

	config := rulego.NewConfig(types.WithEndpointEnabled(true))
	eng, err := rulego.New("chain_status_agg", []byte(chain), engine.WithConfig(config))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("chain_status_agg")

	ruleEng, ok := eng.(*engine.RuleEngine)
	if !ok {
		t.Fatalf("engine type %T not *engine.RuleEngine", eng)
	}
	chainCtx := ruleEng.RootRuleChainCtx().(*engine.RuleChainCtx)

	// 1. aggregated status appears as Connected after the endpoint connects
	waitFor(t, "chain Statuses ep1=Connected", 6*time.Second, func() bool {
		info, ok := chainCtx.Statuses()["ep1"]
		return ok && info.Status == types.StatusConnected
	})

	// 2. broker outage (drop connections + reject new ones for a while) -> stably Reconnecting
	broker.SimulateOutage(3 * time.Second)
	waitFor(t, "chain Statuses ep1=Reconnecting", 6*time.Second, func() bool {
		info, ok := chainCtx.Statuses()["ep1"]
		return ok && info.Status == types.StatusReconnecting
	})

	// 3. outage window ends, broker recovers -> aggregated status back to Connected
	waitFor(t, "chain Statuses ep1 recovers Connected", 12*time.Second, func() bool {
		info, ok := chainCtx.Statuses()["ep1"]
		return ok && info.Status == types.StatusConnected
	})
}
