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
	"github.com/rulego/rulego/utils/maps"
	string2 "github.com/rulego/rulego/utils/str"
	"log"
	"net"
	"net/smtp"
	"strings"
)

// 分隔符
const splitUserSep = ","

func init() {
	Registry.Add(&SendEmailNode{})
}

// Email 邮件消息体
type Email struct {
	//From 发件人邮箱
	From string
	//To 收件人邮箱，多个与`,`隔开
	To string
	//Cc 抄送人邮箱，多个与`,`隔开
	Cc string
	//Bcc 密送人邮箱，多个与`,`隔开
	Bcc string
	//Subject 邮件主题，可以使用 ${metaKeyName} 替换元数据中的变量
	Subject string
	//Body 邮件模板，可以使用 ${metaKeyName} 替换元数据中的变量
	Body string
}

func (e *Email) createEmailMsg(metadata map[string]string) ([]byte, []string) {
	// 设置邮件主题
	subject := string2.SprintfDict(e.Subject, metadata)
	// 设置邮件正文，使用HTML格式
	body := string2.SprintfDict(e.Body, metadata)

	to := strings.Split(e.To, splitUserSep)
	// 将所有的收件人、抄送和密送合并为一个切片
	sendTo := to

	var cc, bcc []string
	if e.Cc != "" {
		cc = strings.Split(e.Cc, splitUserSep)
		sendTo = append(sendTo, cc...)
	}
	if e.Bcc != "" {
		bcc = strings.Split(e.Bcc, splitUserSep)
		sendTo = append(sendTo, bcc...)
	}

	// 创建一个邮件消息，符合RFC 822标准
	msg := []byte("To: " + e.To + "\r\n" +
		"From: " + e.From + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Cc: " + e.Cc + "\r\n" +
		"Bcc: " + e.Bcc + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" +
		body)
	return msg, sendTo
}
func (e *Email) SendEmail(addr string, auth smtp.Auth, metadata map[string]string) error {
	msg, sendTo := e.createEmailMsg(metadata)
	// 调用SendMail函数发送邮件
	return smtp.SendMail(addr, auth, e.From, sendTo, msg)
}

func (e *Email) SendEmailWithTls(addr string, auth smtp.Auth, metadata map[string]string) error {

	msg, sendTo := e.createEmailMsg(metadata)

	host, _, _ := net.SplitHostPort(addr)
	// TLS
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	// Here is the key, you need to call tls.Dial instead of smtp.Dial
	// for smtp servers running on 465 that require an ssl connection
	// from the very beginning (no starttls)
	conn, err := tls.Dial("tcp", addr, tlsconfig)
	if err != nil {
		log.Panic(err)
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
	if err = c.Mail(e.From); err != nil {
		return err
	}

	for _, addr := range sendTo {
		if err = c.Rcpt(addr); err != nil {
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
	SmtpHost string
	//SmtpPort Smtp端口
	SmtpPort int
	//Username 用户名
	Username string
	//Password 授权码
	Password string
	//EnableTls 是否是使用tls方式
	EnableTls bool
	//Email 邮件内容配置
	Email Email
}

// SendEmailNode 通过SMTP服务器发送邮消息
// 如果请求成功，发送消息到`Success`链, 否则发到`Failure`链，
type SendEmailNode struct {
	//节点配置
	Config   SendEmailConfiguration
	smtpAddr string
	smtpAuth smtp.Auth
}

// Type 组件类型
func (x *SendEmailNode) Type() string {
	return "sendEmail"
}

func (x *SendEmailNode) New() types.Node {
	return &SendEmailNode{}
}

// Init 初始化
func (x *SendEmailNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if x.Config.Email.To == "" {
			return errors.New("to address can not empty")
		}
		x.smtpAddr = fmt.Sprintf("%s:%d", x.Config.SmtpHost, x.Config.SmtpPort)
		// 创建一个PLAIN认证
		x.smtpAuth = smtp.PlainAuth("", x.Config.Username, x.Config.Password, x.Config.SmtpHost)
	}
	return err
}

// OnMsg 处理消息
func (x *SendEmailNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	metaData := msg.Metadata.Values()
	emailPojo := x.Config.Email
	var err error
	if x.Config.EnableTls {
		err = emailPojo.SendEmailWithTls(x.smtpAddr, x.smtpAuth, metaData)
	} else {
		err = emailPojo.SendEmail(x.smtpAddr, x.smtpAuth, metaData)
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
