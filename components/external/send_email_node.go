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
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// 分隔符
const splitUserSep = ","

func init() {
	Registry.Add(&SendEmailNode{})
}

// Email 邮件消息体
type Email struct {
	//From 发件人邮箱
	From string `json:"from"`
	//To 收件人邮箱，多个与`,`隔开
	To string `json:"to"`
	//Cc 抄送人邮箱，多个与`,`隔开
	Cc string `json:"cc"`
	//Bcc 密送人邮箱，多个与`,`隔开
	Bcc string `json:"bcc"`
	//Subject 邮件主题，可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Subject string `json:"subject"`
	//Body 邮件模板，可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Body string `json:"body"`
}

// EmailTemplates 邮件模板结构体，统一管理所有邮件字段的模板
type EmailTemplates struct {
	// fromTemplate 发件人模板
	fromTemplate    el.Template
	// toTemplate 收件人模板
	toTemplate      el.Template
	// ccTemplate 抄送人模板
	ccTemplate      el.Template
	// bccTemplate 密送人模板
	bccTemplate     el.Template
	// subjectTemplate 主题模板
	subjectTemplate el.Template
	// bodyTemplate 正文模板
	bodyTemplate    el.Template
	// hasVar 标识模板是否包含变量
	hasVar          bool
}

// initTemplates 初始化邮件模板
// Initialize email templates
// initTemplates 初始化所有邮件字段的模板
func (x *SendEmailNode) initTemplates() error {
	var err error
	
	// 创建发件人模板
	if x.templates.fromTemplate, err = el.NewTemplate(x.Config.Email.From); err != nil {
		return err
	}
	
	// 创建收件人模板
	if x.templates.toTemplate, err = el.NewTemplate(x.Config.Email.To); err != nil {
		return err
	}
	
	// 创建抄送人模板
	if x.templates.ccTemplate, err = el.NewTemplate(x.Config.Email.Cc); err != nil {
		return err
	}
	
	// 创建密送人模板
	if x.templates.bccTemplate, err = el.NewTemplate(x.Config.Email.Bcc); err != nil {
		return err
	}
	
	// 创建主题模板
	if x.templates.subjectTemplate, err = el.NewTemplate(x.Config.Email.Subject); err != nil {
		return err
	}

	// 创建正文模板
	if x.templates.bodyTemplate, err = el.NewTemplate(x.Config.Email.Body); err != nil {
		return err
	}

	// 检查是否包含变量
	x.templates.hasVar = x.templates.fromTemplate.HasVar() || x.templates.toTemplate.HasVar() || 
						 x.templates.ccTemplate.HasVar() || x.templates.bccTemplate.HasVar() ||
						 x.templates.subjectTemplate.HasVar() || x.templates.bodyTemplate.HasVar()
	return nil
}

// createEmailMsg 创建邮件消息内容
func (x *SendEmailNode) createEmailMsg(ctx types.RuleContext, ruleMsg types.RuleMsg) ([]byte, []string) {
	var from, to, cc, bcc, subject, body string
	var evn map[string]interface{}
	if x.templates.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, ruleMsg)
	}
	
	// 执行模板渲染
	from = x.templates.fromTemplate.ExecuteAsString(evn)
	to = x.templates.toTemplate.ExecuteAsString(evn)
	cc = x.templates.ccTemplate.ExecuteAsString(evn)
	bcc = x.templates.bccTemplate.ExecuteAsString(evn)
	subject = x.templates.subjectTemplate.ExecuteAsString(evn)
	body = x.templates.bodyTemplate.ExecuteAsString(evn)

	toList := strings.Split(to, splitUserSep)
	// 将所有的收件人、抄送和密送合并为一个切片
	sendTo := toList

	var ccList, bccList []string
	if cc != "" {
		ccList = strings.Split(cc, splitUserSep)
		sendTo = append(sendTo, ccList...)
	}
	if bcc != "" {
		bccList = strings.Split(bcc, splitUserSep)
		sendTo = append(sendTo, bccList...)
	}

	// 创建一个邮件消息，符合RFC 822标准
	msg := []byte("To: " + to + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Cc: " + cc + "\r\n" +
		"Bcc: " + bcc + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" +
		body)
	return msg, sendTo
}

func (x *SendEmailNode) SendEmail(ctx types.RuleContext, ruleMsg types.RuleMsg, addr string, auth smtp.Auth, connectTimeout time.Duration) error {
	msg, sendTo := x.createEmailMsg(ctx, ruleMsg)
	// 获取渲染后的发件人地址
	var evn map[string]interface{}
	if x.templates.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, ruleMsg)
	}
	from := x.templates.fromTemplate.ExecuteAsString(evn)
	// 调用SendMail函数发送邮件
	return smtp.SendMail(addr, auth, from, sendTo, msg)
}

func (x *SendEmailNode) SendEmailWithTls(ctx types.RuleContext, ruleMsg types.RuleMsg, addr string, auth smtp.Auth, connectTimeout time.Duration) error {

	msg, sendTo := x.createEmailMsg(ctx, ruleMsg)

	host, _, _ := net.SplitHostPort(addr)

	conn, err := net.DialTimeout("tcp", addr, connectTimeout)
	if err != nil {
		return err
	}
	// TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	conn = tls.Client(conn, tlsConfig)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	// Auth
	if err = c.Auth(auth); err != nil {
		return err
	}

	// To && From
	// 获取渲染后的发件人地址
	var evn map[string]interface{}
	if x.templates.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, ruleMsg)
	}
	from := x.templates.fromTemplate.ExecuteAsString(evn)
	if err = c.Mail(from); err != nil {
		return err
	}

	for _, item := range sendTo {
		if err = c.Rcpt(item); err != nil {
			return err
		}
	}

	// Data
	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	if err = w.Close(); err != nil {
		return err
	}

	return c.Quit()
}

// SendEmailConfiguration 配置
type SendEmailConfiguration struct {
	//SmtpHost Smtp主机地址
	SmtpHost string `json:"smtpHost"`
	//SmtpPort Smtp端口
	SmtpPort int `json:"smtpPort"`
	//Username 用户名
	Username string `json:"username"`
	//Password 授权码
	Password string `json:"password"`
	//EnableTls 是否是使用tls方式
	EnableTls bool `json:"enableTls"`
	//Email 邮件内容配置
	Email Email `json:"email"`
	//ConnectTimeout 连接超时，单位秒
	ConnectTimeout int
}

// SendEmailNode 通过SMTP服务器发送邮消息
// 如果请求成功，发送消息到`Success`链, 否则发到`Failure`链，
type SendEmailNode struct {
	//节点配置
	Config                 SendEmailConfiguration
	ConnectTimeoutDuration time.Duration
	smtpAddr               string
	smtpAuth               smtp.Auth
	// templates 邮件模板管理器
	templates              EmailTemplates
}

// Type 组件类型
func (x *SendEmailNode) Type() string {
	return "sendEmail"
}

func (x *SendEmailNode) New() types.Node {
	return &SendEmailNode{
		Config: SendEmailConfiguration{
			ConnectTimeout: 10,
		},
	}
}

// Init 初始化
func (x *SendEmailNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if x.Config.Email.To == "" {
			return errors.New("to address can not empty")
		}
		// 初始化邮件模板
		err = x.initTemplates()
		if err != nil {
			return err
		}
		x.smtpAddr = fmt.Sprintf("%s:%d", x.Config.SmtpHost, x.Config.SmtpPort)
		// 创建一个PLAIN认证
		x.smtpAuth = smtp.PlainAuth("", x.Config.Username, x.Config.Password, x.Config.SmtpHost)
		if x.Config.ConnectTimeout <= 0 {
			x.Config.ConnectTimeout = 10
		}
		x.ConnectTimeoutDuration = time.Duration(x.Config.ConnectTimeout) * time.Second
	}
	return err
}

// OnMsg 处理消息
func (x *SendEmailNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error
	if x.Config.EnableTls {
		err = x.SendEmailWithTls(ctx, msg, x.smtpAddr, x.smtpAuth, x.ConnectTimeoutDuration)
	} else {
		err = x.SendEmail(ctx, msg, x.smtpAddr, x.smtpAuth, x.ConnectTimeoutDuration)

	}
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		ctx.TellSuccess(msg)
	}
}

// Destroy 销毁
func (x *SendEmailNode) Destroy() {
}
