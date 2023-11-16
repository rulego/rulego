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

func TestJsFilterNodeOnMsg(t *testing.T) {
	var node JsFilterNode
	var configuration = make(types.Configuration)
	configuration["jsScript"] = `
		//测试注释
		return msg=='AA';
  	`
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		if err2 != nil {
			t.Errorf("err=%s", err)
		}
		if msg.Type == "TEST_MSG_TYPE_AA" {
			assert.Equal(t, "True", relationType)
		} else if msg.Type == "TEST_MSG_TYPE_BB" {
			assert.Equal(t, "False", relationType)
		}

	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "AA")
	node.OnMsg(ctx, msg)

	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, "BB")
	node.OnMsg(ctx, msg2)
}
