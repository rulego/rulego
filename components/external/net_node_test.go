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

package external

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestNet(t *testing.T) {
	var node NetNode
	var configuration = make(types.Configuration)
	configuration["protocol"] = "tcp"
	configuration["server"] = "127.0.0.1:8888"

	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "{\"test\":\"AA\"}")
	node.OnMsg(ctx, msg1)
	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, "{\"test\":\"BB\"}")
	node.OnMsg(ctx, msg2)
	msg3 := ctx.NewMsg("TEST_MSG_TYPE_CC", metaData, "\"test\":\"CC\\n aa\"")
	node.OnMsg(ctx, msg3)

	time.Sleep(time.Second * 5)
	//销毁并 断开连接
	node.Destroy()
}
