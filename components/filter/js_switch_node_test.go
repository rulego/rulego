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

func TestJsSwitchNodeOnMsg(t *testing.T) {
	var node JsSwitchNode
	var configuration = make(types.Configuration)
	configuration["jsScript"] = `
		//测试注释
		return ['one','two'];
  	`
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	var i = 0
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		if err2 != nil {
			t.Errorf("err=%s", err)
		}
		if i == 0 {
			assert.Equal(t, "one", relationType)
		} else if i == 1 {
			assert.Equal(t, "two", relationType)
		}
		i++

	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("ACTIVITY_EVENT", metaData, "AA")
	node.OnMsg(ctx, msg)
}
