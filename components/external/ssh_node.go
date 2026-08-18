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
//
// 公钥认证示例（密码与私钥至少配置其一）：
// Public key auth example (password and private key are alternatives):
//
//	{
//	  "type": "ssh",
//	  "config": {
//	    "host": "192.168.1.1",
//	    "port": 22,
//	    "username": "root",
//	    "privateKeyPath": "/root/.ssh/id_rsa",
//	    "privateKeyPassphrase": "key-passphrase",
//	    "cmd": "ls /tmp"
//	  }
//	}

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"golang.org/x/crypto/ssh"
)

var (
	SshConfigEmptyErr           = errors.New("ssh config can not empty")
	SshClientNotInitErr         = errors.New("ssh client not initialized")
	SshCmdEmptyErr              = errors.New("cmd can not empty")
	SshConfigPassphraseNoKeyErr = errors.New("ssh privateKeyPassphrase configured but no private key")
)

func init() {
	Registry.Add(&SshNode{})
}

// SshConfiguration SSH节点配置
// SshConfiguration defines SSH node configuration.
type SshConfiguration struct {
	// Host is the SSH server address.
	Host string `json:"host" label:"Host" desc:"SSH server host address" required:"true"`
	// Port is the SSH server port.
	Port int `json:"port" label:"Port" desc:"SSH server port, default 22" required:"true"`
	// Username is the SSH login username.
	Username string `json:"username" label:"Username" desc:"SSH login username" required:"true"`
	// Password is the SSH login password.
	Password string `json:"password" label:"Password" desc:"SSH login password" required:"true"`
	// PrivateKey is the SSH private key PEM content (e.g. -----BEGIN OPENSSH PRIVATE KEY-----).
	// Mutually exclusive with PrivateKeyPath; PrivateKey takes precedence when both are set.
	PrivateKey string `json:"privateKey,omitempty" label:"Private Key" desc:"SSH private key PEM content, e.g. -----BEGIN OPENSSH PRIVATE KEY-----"`
	// PrivateKeyPath is the path to the SSH private key file (e.g. /root/.ssh/id_rsa).
	// Mutually exclusive with PrivateKey; PrivateKey takes precedence when both are set.
	PrivateKeyPath string `json:"privateKeyPath,omitempty" label:"Private Key File" desc:"SSH private key file path, e.g. /root/.ssh/id_rsa"`
	// PrivateKeyPassphrase is the passphrase of the encrypted private key.
	// Required only when the configured private key is encrypted.
	PrivateKeyPassphrase string `json:"privateKeyPassphrase,omitempty" label:"Private Key Passphrase" desc:"Passphrase of the encrypted private key, required only when the private key is encrypted"`
	// Cmd is the shell command. Supports ${metadata.key} and ${msg.key} substitution.
	Cmd string `json:"cmd" label:"Command" desc:"Shell command to execute. Supports ${metadata.key} and ${msg.key} substitution" required:"true"`
}

// SshNode SSH远程命令执行组件，建立SSH连接到远程主机并执行shell命令
// SshNode provides SSH-based remote command execution capabilities.
//
// 核心算法：
// Core Algorithm:
// 1. 懒建立SSH连接（首次使用或配置 NodeClientInitNow 时）- Establish SSH connection lazily (on first use or when NodeClientInitNow is set)
// 2. 解析命令模板，支持变量替换 - Parse command template with variable substitution
// 3. 创建SSH会话执行命令 - Create SSH session to execute command
// 4. 捕获命令输出（stdout+stderr）- Capture command output (stdout+stderr)
// 5. 连接断开时自动重连 - Reconnect automatically after connection loss
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
//		"password": "secret123",        // 密码 - Password (密码与私钥至少配置其一 - password and private key are alternatives)
//		"privateKeyPath": "/root/.ssh/id_rsa",            // 私钥文件路径 - Private key file path
//		"privateKeyPassphrase": "key-passphrase",         // 私钥口令（私钥加密时必填）- Passphrase of encrypted private key
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
	base.SharedNode[*ssh.Client]
	//节点配置
	Config SshConfiguration
	// cmdTemplate 命令模板，用于解析动态命令
	// cmdTemplate template for resolving dynamic commands
	cmdTemplate el.Template
	// hasVar 标识模板是否包含变量
	// hasVar indicates whether the template contains variables
	hasVar bool
	// connHealthy 缓存连接健康标记，避免每条消息都调 SetStatus
	// connHealthy caches connection health to avoid SetStatus on every message
	connHealthy int32
	// signer 私钥签名器，Init 时解析并缓存，用于公钥认证；未配置私钥时为 nil
	// signer is parsed and cached at Init time for public key auth; nil when no private key is configured
	signer ssh.Signer
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
	if err != nil {
		return err
	}
	if x.Config.Host == "" || x.Config.Port == 0 || x.Config.Username == "" {
		return SshConfigEmptyErr
	}
	// 密码与私钥至少配置其一 - require password or private key
	if x.Config.Password == "" && x.Config.PrivateKey == "" && x.Config.PrivateKeyPath == "" {
		return SshConfigEmptyErr
	}
	// 配置了私钥口令但没有私钥 - passphrase without private key
	if x.Config.PrivateKeyPassphrase != "" && x.Config.PrivateKey == "" && x.Config.PrivateKeyPath == "" {
		return SshConfigPassphraseNoKeyErr
	}
	if x.Config.Cmd == "" {
		return SshCmdEmptyErr
	}
	// 解析并缓存私钥签名器，提前暴露密钥格式/口令错误 - parse the private key early (fail-fast)
	x.signer, err = x.parseSigner()
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", x.Config.Host, x.Config.Port)
	err = x.SharedNode.InitWithClose(ruleConfig, x.Type(), addr, ruleConfig.NodeClientInitNow, func() (*ssh.Client, error) {
		return x.initClient()
	}, func(client *ssh.Client) error {
		return client.Close()
	})
	if err != nil {
		return err
	}
	// 启用同链连接池：本地模式 *ssh.Client 按节点ID注册到链目录，供链内 ref:// 借用复用
	x.SharedNode.BindChain(configuration)
	x.cmdTemplate, err = el.NewTemplate(x.Config.Cmd)
	if err != nil {
		return err
	}
	x.hasVar = x.cmdTemplate.HasVar()
	return nil
}

// OnMsg 方法用来处理消息，每条流入组件的数据会经过该函数处理
func (x *SshNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	client, err := x.SharedNode.GetSafely()
	if err != nil {
		if errors.Is(err, base.ErrClientNotInit) {
			err = SshClientNotInitErr
		}
		ctx.TellFailure(msg, err)
		return
	}
	var evn map[string]interface{}
	if x.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	cmd := x.cmdTemplate.ExecuteAsString(evn)
	session, err := client.NewSession()
	if err != nil {
		x.onConnFailure(err)
		ctx.TellFailure(msg, err)
		return
	}
	defer session.Close()
	output, err := session.CombinedOutput(cmd)

	msg.SetData(string(output))
	msg.DataType = types.TEXT

	if err != nil {
		// 命令非零退出/无退出码属于命令失败，连接本身可能还是好的
		if !isSshCmdError(err) {
			x.onConnFailure(err)
		}
		ctx.TellFailure(msg, err)
	} else {
		if atomic.CompareAndSwapInt32(&x.connHealthy, 0, 1) {
			x.SharedNode.SetStatus(types.StatusConnected, "")
		}
		ctx.TellSuccess(msg)
	}
}

// onConnFailure 连接级失败：标记断连并重置客户端，下次消息触发重连
// onConnFailure marks the connection lost and resets the client for lazy re-dial.
func (x *SshNode) onConnFailure(err error) {
	if atomic.CompareAndSwapInt32(&x.connHealthy, 1, 0) {
		_ = x.SharedNode.Close()
		x.SharedNode.SetStatus(types.StatusReconnecting, err.Error())
	}
}

// isSshCmdError 命令退出类错误不算断连
// isSshCmdError reports command exit errors, which don't imply a broken connection.
func isSshCmdError(err error) bool {
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return true
	}
	var missingErr *ssh.ExitMissingError
	return errors.As(err, &missingErr)
}

// Destroy 方法用来销毁组件，做一些资源释放操作
func (x *SshNode) Destroy() {
	_ = x.SharedNode.Close()
}

// clientConfig 构建 SSH 客户端配置。
// clientConfig builds the SSH client configuration.
func (x *SshNode) clientConfig() *ssh.ClientConfig {
	// 密码与公钥认证可共存，由 SSH 服务器协商选择 - password and public key auth can coexist
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if x.Config.Password != "" {
		authMethods = append(authMethods, ssh.Password(x.Config.Password))
	}
	if x.signer != nil {
		authMethods = append(authMethods, ssh.PublicKeys(x.signer))
	}
	return &ssh.ClientConfig{
		User:            x.Config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

// initClient 拨号建立 SSH 连接
// initClient dials the SSH server.
func (x *SshNode) initClient() (*ssh.Client, error) {
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", x.Config.Host, x.Config.Port), x.clientConfig())
}

// parseSigner 解析私钥并返回签名器；未配置私钥时返回 nil。
// parseSigner parses the configured private key into a signer; returns nil when no private key is configured.
// 私钥内容 (PrivateKey) 优先于文件路径 (PrivateKeyPath)。
// PrivateKey content takes precedence over PrivateKeyPath.
func (x *SshNode) parseSigner() (ssh.Signer, error) {
	if x.Config.PrivateKey == "" && x.Config.PrivateKeyPath == "" {
		return nil, nil
	}
	var pemBytes []byte
	var err error
	if x.Config.PrivateKey != "" {
		pemBytes = []byte(x.Config.PrivateKey)
	} else {
		pemBytes, err = os.ReadFile(x.Config.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("ssh: read private key file %s: %w", x.Config.PrivateKeyPath, err)
		}
	}
	if x.Config.PrivateKeyPassphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(x.Config.PrivateKeyPassphrase))
		if err != nil {
			return nil, fmt.Errorf("ssh: parse encrypted private key: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, errors.New("ssh: private key is encrypted, please configure privateKeyPassphrase")
		}
		return nil, fmt.Errorf("ssh: parse private key: %w", err)
	}
	return signer, nil
}

// Desc returns the component description
func (x *SshNode) Desc() string {
	return "SSH remote command execution. Connects to remote host and executes shell commands. cmd supports ${metadata.key} and ${msg.key} substitution. Routes to Success/Failure"
}
