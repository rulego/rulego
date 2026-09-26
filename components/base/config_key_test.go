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

package base

import (
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// TestConfigKey verifies the key is deterministic for identical configs and
// distinct for different ones; unmarshalable values degrade to "config".
func TestConfigKey(t *testing.T) {
	a := ConfigKey(map[string]interface{}{"host": "127.0.0.1", "port": 9090})
	b := ConfigKey(map[string]interface{}{"port": 9090, "host": "127.0.0.1"})
	assert.Equal(t, a, b)
	assert.Equal(t, a, ConfigKey(map[string]interface{}{"host": "127.0.0.1", "port": 9090}))
	assert.NotEqual(t, a, ConfigKey(map[string]interface{}{"host": "127.0.0.2", "port": 9090}))

	// sha256 truncated to 8 bytes -> 16 hex chars
	assert.Equal(t, 16, len(a))

	// chan cannot be marshaled: constant fallback, still stable
	assert.Equal(t, "config", ConfigKey(make(chan int)))

	// nil and scalar inputs must not error
	assert.NotEqual(t, "", ConfigKey(nil))
	assert.NotEqual(t, "", ConfigKey(42))
}

// TestNodeIdOf reads the node Id injected via NodeConfigurationKeySelfDefinition;
// absent or wrongly-typed entries yield "".
func TestNodeIdOf(t *testing.T) {
	cfg := types.Configuration{
		types.NodeConfigurationKeySelfDefinition: types.RuleNode{Id: "node1"},
	}
	assert.Equal(t, "node1", NodeIdOf(cfg))
	assert.Equal(t, "", NodeIdOf(types.Configuration{}))
	assert.Equal(t, "", NodeIdOf(types.Configuration{
		types.NodeConfigurationKeySelfDefinition: 42,
	}))
}
