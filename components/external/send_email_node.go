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

// Separator
const splitUserSep = ","

func init() {
	Registry.Add(&SendEmailNode{})
}

// Email
type Email struct {
	From    string `json:"from" label:"From" desc:"Sender email address" required:"true"`
	To      string `json:"to" label:"To" desc:"Recipient email addresses, comma-separated for multiple" required:"true"`
	Cc      string `json:"cc" label:"CC" desc:"CC email addresses, comma-separated for multiple"`
	Bcc     string `json:"bcc" label:"BCC" desc:"BCC email addresses, comma-separated for multiple"`
	Subject string `json:"subject" label:"Subject" desc:"Email subject, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
	Body    string `json:"body" label:"Body" desc:"Email body content, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
}

// EmailTemplates: An email template structure that centrally manages templates for all email fields
type EmailTemplates struct {
	// fromTemplate sender template
	fromTemplate el.Template
	// toTemplate recipient template
	toTemplate el.Template
	// ccTemplate CC Person Template
	ccTemplate el.Template
	// bccTemplate: Secret sending person
	bccTemplate el.Template
	// subjectTemplate theme template
	subjectTemplate el.Template
	// bodyTemplate body template
	bodyTemplate el.Template
	// hasVar identifies whether the template contains variables
	hasVar bool
}

// initTemplates initializes the email template
// Initialize email templates
// initTemplates initializes templates for all mail fields
func (x *SendEmailNode) initTemplates() error {
	var err error

	// Create a sender template
	if x.templates.fromTemplate, err = el.NewTemplate(x.Config.Email.From); err != nil {
		return err
	}

	// Create recipient templates
	if x.templates.toTemplate, err = el.NewTemplate(x.Config.Email.To); err != nil {
		return err
	}

	// Create a CC person-to-copy template
	if x.templates.ccTemplate, err = el.NewTemplate(x.Config.Email.Cc); err != nil {
		return err
	}

	// Create a secret sender template
	if x.templates.bccTemplate, err = el.NewTemplate(x.Config.Email.Bcc); err != nil {
		return err
	}

	// Create a theme template
	if x.templates.subjectTemplate, err = el.NewTemplate(x.Config.Email.Subject); err != nil {
		return err
	}

	// Create a body template for the main text
	if x.templates.bodyTemplate, err = el.NewTemplate(x.Config.Email.Body); err != nil {
		return err
	}

	// Check if variables are included
	x.templates.hasVar = x.templates.fromTemplate.HasVar() || x.templates.toTemplate.HasVar() ||
		x.templates.ccTemplate.HasVar() || x.templates.bccTemplate.HasVar() ||
		x.templates.subjectTemplate.HasVar() || x.templates.bodyTemplate.HasVar()
	return nil
}

// createEmailMsg creates email message content
func (x *SendEmailNode) createEmailMsg(ctx types.RuleContext, ruleMsg types.RuleMsg) ([]byte, []string) {
	var from, to, cc, bcc, subject, body string
	var evn map[string]interface{}
	if x.templates.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, ruleMsg)
	}

	// Perform template rendering
	from = x.templates.fromTemplate.ExecuteAsString(evn)
	to = x.templates.toTemplate.ExecuteAsString(evn)
	cc = x.templates.ccTemplate.ExecuteAsString(evn)
	bcc = x.templates.bccTemplate.ExecuteAsString(evn)
	subject = x.templates.subjectTemplate.ExecuteAsString(evn)
	body = x.templates.bodyTemplate.ExecuteAsString(evn)

	toList := strings.Split(to, splitUserSep)
	// Merge all recipients, cc, and secret mail into a single slice
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

	// Create an email message that complies with RFC 822 standards
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
	// Get the sender address after rendering
	var evn map[string]interface{}
	if x.templates.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, ruleMsg)
	}
	from := x.templates.fromTemplate.ExecuteAsString(evn)
	// Call the SendMail function to send an email
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
	// Get the sender address after rendering
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

// SendEmailConfiguration configuration
type SendEmailConfiguration struct {
	SmtpHost       string `json:"smtpHost" label:"SMTP Host" desc:"SMTP server address" required:"true"`
	SmtpPort       int    `json:"smtpPort" label:"SMTP Port" desc:"SMTP server port, default 25"`
	Username       string `json:"username" label:"Username" desc:"SMTP authentication username"`
	Password       string `json:"password" label:"Password" desc:"SMTP authentication password"`
	EnableTls      bool   `json:"enableTls" label:"Enable TLS" desc:"Enable TLS encryption"`
	Email          Email  `json:"email" label:"Email" desc:"Email content configuration"`
	ConnectTimeout int    `json:"connectTimeout" label:"Connect Timeout (s)" desc:"SMTP connection timeout in seconds"`
}

// SendEmailNode sends mail messages through the SMTP server
// If the request succeeds, send the message to the `Success` chain; otherwise, send it to the `Failure` chain.
type SendEmailNode struct {
	//Node configuration
	Config                 SendEmailConfiguration
	ConnectTimeoutDuration time.Duration
	smtpAddr               string
	smtpAuth               smtp.Auth
	// templates: Email template manager
	templates EmailTemplates
}

// Type returns the component type
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

// Init initializes the component
func (x *SendEmailNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if x.Config.Email.To == "" {
			return errors.New("to address can not empty")
		}
		// Initialize email templates
		err = x.initTemplates()
		if err != nil {
			return err
		}
		x.smtpAddr = fmt.Sprintf("%s:%d", x.Config.SmtpHost, x.Config.SmtpPort)
		// Create a PLAIN certification
		x.smtpAuth = smtp.PlainAuth("", x.Config.Username, x.Config.Password, x.Config.SmtpHost)
		if x.Config.ConnectTimeout <= 0 {
			x.Config.ConnectTimeout = 10
		}
		x.ConnectTimeoutDuration = time.Duration(x.Config.ConnectTimeout) * time.Second
	}
	return err
}

// OnMsg processes a message
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

// Destroy releases resources
func (x *SendEmailNode) Destroy() {
}

// Desc returns the component description
func (x *SendEmailNode) Desc() string {
	return "Send email via SMTP with TLS support. Subject and body support ${metadata.key} and ${msg.key} substitution. Multiple recipients via comma separation. Routes to Success/Failure"
}
