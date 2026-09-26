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

package types

import (
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

// TestNewConfigDefaults 验证默认配置值
func TestNewConfigDefaults(t *testing.T) {
	c := NewConfig()
	assert.Equal(t, 2*time.Second, c.ScriptMaxExecutionTime)
	assert.True(t, c.EndpointEnabled)
	assert.True(t, c.OnEndWithFailure)
	assert.NotNil(t, c.Logger)
	assert.NotNil(t, c.Properties)
	assert.Equal(t, 0, len(c.Properties))
}

// TestConfigUdf UDF 注册的脚本类型前缀约定与按类型检索
func TestConfigUdf(t *testing.T) {
	c := NewConfig()

	upper := func(s string) string { return s }
	c.RegisterUdf("upper", upper)
	c.RegisterUdf("count", 42)
	c.RegisterUdf("jsHelper", Script{Type: "js", Content: "function jsHelper(){}"})
	c.RegisterUdf("allHelper", Script{Type: AllScript, Content: "function allHelper(){}"})
	c.RegisterUdf("luaHelper", Script{Type: "lua", Content: "function luaHelper(){}"})

	// 无脚本类型：按原名取；Script 值解包返回 Content
	assert.Equal(t, 42, c.GetUdf("count", ""))
	assert.Equal(t, "function jsHelper(){}", c.GetUdf("jsHelper", "js"))
	assert.Equal(t, "function allHelper(){}", c.GetUdf("allHelper", ""))
	assert.Nil(t, c.GetUdf("missing", ""))
	assert.Nil(t, c.GetUdf("jsHelper", ""))

	// GetUdfs("") 返回全部注册项（Script 保持原值）
	all := c.GetUdfs("")
	assert.Equal(t, 5, len(all))
	assert.Equal(t, Script{Type: "js", Content: "function jsHelper(){}"}, all["js#jsHelper"])

	// GetUdfs("js") 仅返回该类型并剥掉前缀
	js := c.GetUdfs("js")
	assert.Equal(t, 1, len(js))
	assert.Equal(t, "function jsHelper(){}", js["jsHelper"])

	assert.Equal(t, 0, len(c.GetUdfs("python")))

	empty := NewConfig()
	assert.Nil(t, empty.GetUdf("x", ""))
	assert.Equal(t, 0, len(empty.GetUdfs("")))
}

// TestDefaultPool 包级默认协程池可用
func TestDefaultPool(t *testing.T) {
	p := DefaultPool()
	assert.NotNil(t, p)
	done := make(chan struct{})
	assert.Nil(t, p.Submit(func() { close(done) }))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("task not executed")
	}
	p.Release()
}
