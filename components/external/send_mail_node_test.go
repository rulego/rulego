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
	"github.com/rulego/rulego/utils/maps"
	"sync"
	"testing"
)

func TestSendEmailNode(t *testing.T) {
	var sendEmailNode SendEmailNode
	node := sendEmailNode.New()
	email := Email{
		From:    "xxx@163.com",
		To:      "xxx@163.com,aaa@hotmail.com",
		Cc:      "bbb@hotmail.com",
		Subject: "测试邮件3",
		Body:    "<b>测试内容3</b>",
	}
	mailConfig := SendEmailConfiguration{
		SmtpHost:  "smtp.163.com",
		SmtpPort:  25,
		Username:  "xxx@163.com",
		Password:  "authorize code",
		EnableTls: false,
		Email:     email,
	}
	var configuration = make(types.Configuration)

	err := maps.Map2Struct(&mailConfig, &configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	config := types.NewConfig()
	err = node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	var group sync.WaitGroup
	group.Add(1)
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {

	})
	ctx.SetEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		group.Done()
		if err != nil {
			t.Errorf("err=%s", err)
		}
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE", metaData, "aa")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	group.Wait()
}
func TestSendEmailNodeWithTls(t *testing.T) {
	var sendEmailNode SendEmailNode
	node := sendEmailNode.New()
	email := Email{
		From:    "xxx@163.com",
		To:      "xxx@163.com,yyy@hotmail.com",
		Cc:      "bbb@hotmail.com",
		Subject: "测试邮件3",
		Body:    "<b>测试内容3</b>",
	}
	mailConfig := SendEmailConfiguration{
		SmtpHost:  "smtp.163.com",
		SmtpPort:  465,
		Username:  "xx@163.com",
		Password:  "authorize code",
		EnableTls: true,
		Email:     email,
	}
	var configuration = make(types.Configuration)

	err := maps.Map2Struct(&mailConfig, &configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	config := types.NewConfig()
	err = node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	var group sync.WaitGroup
	group.Add(1)
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {

	})
	ctx.SetEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		group.Done()
		if err != nil {
			t.Errorf("err=%s", err)
		}
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE", metaData, "aa")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	group.Wait()
}
