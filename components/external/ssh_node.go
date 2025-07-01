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

//{
//"type": "ssh",
//"config": {
//"host": "192.168.1.1",
//"port": 22,
//"username": "root",
//"password": "password",
//"cmd": "sh count.sh test.txt hello"
//}
//}

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"golang.org/x/crypto/ssh"
)

var (
	SshConfigEmptyErr   = errors.New("ssh config can not empty")
	SshClientNotInitErr = errors.New("ssh client not initialized")
	SshCmdEmptyErr      = errors.New("cmd can not empty")
)

func init() {
	Registry.Add(&SshNode{})
}

// SshConfiguration SSH节点配置
// SshConfiguration defines SSH node configuration.
type SshConfiguration struct {
	//Host ssh 主机地址
	Host string
	//Port ssh 主机端口
	Port int
	//Username ssh登录用户名
	Username string
	//Password ssh登录密码
	Password string
	//Cmd shell命令,可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Cmd string
}

// SshNode SSH远程命令执行组件，建立SSH连接到远程主机并执行shell命令
// SshNode provides SSH-based remote command execution capabilities.
//
// 核心算法：
// Core Algorithm:
// 1. 初始化时建立SSH连接 - Establish SSH connection during initialization
// 2. 解析命令模板，支持变量替换 - Parse command template with variable substitution
// 3. 创建SSH会话执行命令 - Create SSH session to execute command
// 4. 捕获命令输出（stdout+stderr）- Capture command output (stdout+stderr)
// 5. 关闭会话并返回结果 - Close session and return results
//
// 变量替换 - Variable substitution:
//   - ${metadata.key}: 访问消息元数据变量 - Access message metadata variables
//   - ${msg.key}: 访问消息负荷变量 - Access message payload variables
//
// 配置示例 - Configuration example:
//
//	{
//		"host": "192.168.1.100",        // SSH服务器地址 - SSH server address
//		"port": 22,                     // SSH端口 - SSH port
//		"username": "admin",            // 用户名 - Username
//		"password": "secret123",        // 密码 - Password
//		"cmd": "ls -la /tmp/${metadata.path}"  // 支持变量替换的命令 - Command with variables
//	}
//
// 使用示例 - Usage examples:
//
//	// 执行系统监控命令 - Execute system monitoring command
//	{
//		"id": "sshMonitor",
//		"type": "ssh",
//		"configuration": {
//			"host": "server.example.com",
//			"port": 22,
//			"username": "monitor",
//			"password": "mon123",
//			"cmd": "df -h && free -m"
//		}
//	}
//
//	// 执行带动态参数的命令 - Execute command with dynamic parameters
//	{
//		"id": "sshDynamic",
//		"type": "ssh",
//		"configuration": {
//			"host": "${metadata.targetHost}",
//			"port": 22,
//			"username": "admin",
//			"password": "pass",
//			"cmd": "cat /var/log/${msg.logFile} | tail -${metadata.lines}"
//		}
//	}
//
// 使用场景 - Use cases:
//   - 远程系统监控和维护 - Remote system monitoring and maintenance
//   - 批量服务器管理操作 - Batch server management operations
//   - 自动化运维脚本执行 - Automated operations script execution
type SshNode struct {
	//节点配置
	Config SshConfiguration
	// client 是一个 ssh.Client 类型的字段，用来保存 ssh 客户端对象
	client      *ssh.Client
	cmdTemplate str.Template
	// 保护client字段的并发访问
	clientMutex sync.RWMutex
}

// Type 方法用来返回组件的类型
func (x *SshNode) Type() string {
	return "ssh"
}

// New 方法用来创建一个 SshNode 的新实例
func (x *SshNode) New() types.Node {
	return &SshNode{Config: SshConfiguration{
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		Password: "password",
	}}
}

// Init 方法用来初始化组件，一般做一些组件参数配置或者客户端初始化操作
func (x *SshNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		// 从配置中获取 ssh 连接的参数
		sshConfig := x.Config
		// 如果参数不为空，则创建一个 ssh 客户端对象
		if sshConfig.Host != "" && sshConfig.Port != 0 && sshConfig.Username != "" && sshConfig.Password != "" {
			config := &ssh.ClientConfig{
				User: sshConfig.Username,
				Auth: []ssh.AuthMethod{
					ssh.Password(sshConfig.Password),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
			x.client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshConfig.Host, sshConfig.Port), config)
		} else {
			return SshConfigEmptyErr
		}
		x.cmdTemplate = str.NewTemplate(x.Config.Cmd)
	}
	return err
}

// OnMsg 方法用来处理消息，每条流入组件的数据会经过该函数处理
func (x *SshNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error

	// 安全获取client引用
	x.clientMutex.RLock()
	client := x.client
	x.clientMutex.RUnlock()

	if client == nil {
		ctx.TellFailure(msg, SshClientNotInitErr)
		return
	}

	// 获取shell 命令
	cmd := x.Config.Cmd
	if cmd == "" {
		ctx.TellFailure(msg, SshCmdEmptyErr)
		return
	}
	cmd = x.cmdTemplate.ExecuteFn(func() map[string]any {
		return base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	})
	var output []byte
	var session *ssh.Session
	// 如果有 ssh 客户端对象，则创建一个 ssh 会话，并执行远程 shell 命令，并获取其输出或错误信息
	if session, err = client.NewSession(); err == nil {
		defer session.Close()
		output, err = session.CombinedOutput(cmd)

		msg.SetData(string(output))
		msg.DataType = types.TEXT

		if err != nil {
			ctx.TellFailure(msg, err)
		} else {
			// 将输出结果作为新的消息发送到下一个组件
			ctx.TellSuccess(msg)
		}
	} else {
		ctx.TellFailure(msg, err)
	}
}

// Destroy 方法用来销毁组件，做一些资源释放操作
func (x *SshNode) Destroy() {
	x.clientMutex.Lock()
	defer x.clientMutex.Unlock()

	if x.client != nil {
		_ = x.client.Close()
		x.client = nil
	}
}
