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

package filter

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestFieldFilterOnMsg1(t *testing.T) {

	var node FieldFilterNode
	var configuration = make(types.Configuration)
	configuration["checkAllKeys"] = true
	configuration["dataNames"] = "temperature"
	configuration["metadataNames"] = "productType,name"
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		assert.Equal(t, types.True, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "A001")
	metaData.PutValue("name", "A001")
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, `{"temperature":56}`)
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}
func TestFieldFilterOnMsg2(t *testing.T) {

	var node FieldFilterNode
	var configuration = make(types.Configuration)
	configuration["checkAllKeys"] = true
	configuration["dataNames"] = "temperature"
	configuration["metadataNames"] = "productType,name,location"
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		assert.Equal(t, types.False, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "A001")
	metaData.PutValue("name", "A001")
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, `{"temperature":56}`)
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}

func TestFieldFilterOnMsg3(t *testing.T) {

	var node FieldFilterNode
	var configuration = make(types.Configuration)
	configuration["checkAllKeys"] = false
	configuration["dataNames"] = "temperature"
	configuration["metadataNames"] = "productType,name,location"
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		assert.Equal(t, types.True, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "A001")
	metaData.PutValue("name", "A001")
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, `{"temperature":56}`)
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}

func TestFieldFilterOnMsg4(t *testing.T) {

	var node FieldFilterNode
	var configuration = make(types.Configuration)
	configuration["checkAllKeys"] = false
	configuration["dataNames"] = "temperature"
	configuration["metadataNames"] = "productType,name,location"
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		assert.Equal(t, types.True, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, `{"temperature":56}`)
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}
