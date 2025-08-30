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
	"os"
	"strconv"
	"testing"
	"time"
)

func TestSendEmailNode(t *testing.T) {
	var targetNodeType = "sendEmail"
	smtpHost := os.Getenv("TEST_SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "smtp.163.com"
	}
	smtpTlsPortEnv := os.Getenv("TEST_SMTP_TLS_PORT")
	if smtpTlsPortEnv == "" {
		smtpTlsPortEnv = "465"
	}
	smtpTlsPort, err := strconv.Atoi(smtpTlsPortEnv)
	if err != nil {
		smtpTlsPort = 465
	}
	smtpPortEnv := os.Getenv("TEST_SMTP_TLS_PORT")
	if smtpPortEnv == "" {
		smtpPortEnv = "25"
	}

	smtpPort, err := strconv.Atoi(smtpPortEnv)
	if err != nil {
		smtpPort = 22
	}
	username := os.Getenv("TEST_SMTP_USERNAME")
	if username == "" {
		username = "xx@163.com"
	}
	password := os.Getenv("TEST_SMTP_PASSWORD")
	if password == "" {
		password = "xx"
	}

	emailWithTls := types.Configuration{
		"from":    username,
		"to":      username,
		"cc":      username,
		"bcc":     username,
		"subject": "测试邮件WithTls",
		"body":    "<b>测试内容WithTls，productType:${metadata.productType}</b>",
	}
	mailConfigWithTls := types.Configuration{
		"smtpHost":  smtpHost,
		"smtpPort":  smtpTlsPort,
		"username":  username,
		"password":  password,
		"enableTls": true,
		"email":     emailWithTls,
	}
	emailWithNotTls := types.Configuration{
		"from":    username,
		"to":      username,
		"cc":      username,
		"bcc":     username,
		"subject": "测试邮件WithNotTls",
		"body":    "<b>测试内容WithNotTls，productType:${metadata.productType}</b>",
	}
	mailConfigWithNotTls := types.Configuration{
		"smtpHost":  smtpHost,
		"smtpPort":  smtpPort,
		"username":  username,
		"password":  password,
		"enableTls": false,
		"email":     emailWithNotTls,
	}

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &SendEmailNode{}, types.Configuration{
			"connectTimeout": 10,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, mailConfigWithTls, types.Configuration{
			"smtpHost":  smtpHost,
			"smtpPort":  smtpTlsPort,
			"username":  username,
			"password":  password,
			"enableTls": true,
			"email": Email{
				From:    username,
				To:      username,
				Cc:      username,
				Bcc:     username,
				Subject: "测试邮件WithTls",
				Body:    "<b>测试内容WithTls，productType:${metadata.productType}</b>",
			},
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"connectTimeout": 10,
			"email":          emailWithTls,
		}, types.Configuration{
			"connectTimeout": 10,
			"email": Email{
				From:    username,
				To:      username,
				Cc:      username,
				Bcc:     username,
				Subject: "测试邮件WithTls",
				Body:    "<b>测试内容WithTls，productType:${metadata.productType}</b>",
			},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, mailConfigWithTls, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, mailConfigWithNotTls, Registry)
		assert.Nil(t, err)
		mailConfigHostErr := types.Configuration{
			"smtpHost":  "smtp.xx.com",
			"smtpPort":  25,
			"username":  "xx@163.com",
			"password":  "xx",
			"enableTls": true,
			"email":     emailWithNotTls,
		}
		node3, err := test.CreateAndInitNode(targetNodeType, mailConfigHostErr, Registry)

		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"smtpHost":  "smtp.163.com",
			"smtpPort":  25,
			"username":  "xx@163.com",
			"password":  "xx",
			"enableTls": false,
		}, Registry)
		assert.NotNil(t, err)

		node4 := SendEmailNode{
			Config: SendEmailConfiguration{ConnectTimeout: -1},
		}
		node4.Init(types.NewConfig(), mailConfigWithTls)
		assert.Equal(t, 10, node4.Config.ConnectTimeout)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if username == "xx@163.com" {
						assert.Equal(t, types.Failure, relationType)
					} else {
						assert.Equal(t, types.Success, relationType)
					}
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if username == "xx@163.com" {
						assert.Equal(t, types.Failure, relationType)
					} else {
						assert.Equal(t, types.Success, relationType)
					}
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Second * 1)
	})

}
