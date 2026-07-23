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
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
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

// SshConfiguration: SSH node configuration
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
	// Cmd is the shell command. Supports ${metadata.key} and ${msg.key} substitution.
	Cmd string `json:"cmd" label:"Command" desc:"Shell command to execute. Supports ${metadata.key} and ${msg.key} substitution" required:"true"`
}

// SshNode SSH remote command execution component establishes an SSH connection to a remote host and executes shell commands
// SshNode provides SSH-based remote command execution capabilities.
//
// Core algorithm:
// Core Algorithm:
// 1. Establish SSH connection during initialization - Establish SSH connection during initialization
// 2. Parse command templates with variable substitution - Parse command template with variable substitution
// 3. Create SSH session to execute command - Create SSH session to execute command
// 4. Capture command output (stdout+stderr) - Capture command output (stdout+stderr)
// 5. Close session and return results - Close session and return results
//
// Variable substitution:
//   - ${metadata.key}: Access message metadata variables
//   - ${msg.key}: Access message payload variables
//
// Configuration example:
//
//	{
//		"host": "192.168.1.100", // SSH server address - SSH server address
//		"port": 22, // SSH port - SSH port
//		"username": "admin", // username - username
//		"password": "secret123", // Password - password
//		"cmd": "ls -la /tmp/${metadata.path}" // Command supporting variable substitution - Command with variables
//	}
//
// Usage examples:
//
//	Execute system monitoring command
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
//	Execute command with dynamic parameters
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
// Use cases:
//   - Remote system monitoring and maintenance
//   - Batch server management operations
//   - Automated operations script execution
type SshNode struct {
	//Node configuration
	Config SshConfiguration
	// client is a field of type ssh.Client used to store SSH client objects
	client *ssh.Client
	// cmdTemplate command template, used to parse dynamic commands
	// cmdTemplate template for resolving dynamic commands
	cmdTemplate el.Template
	// hasVar identifies whether the template contains variables
	// hasVar indicates whether the template contains variables
	hasVar bool
	// Protects concurrent access to the client field
	clientMutex sync.RWMutex
}

// The Type method is used to return the type of component
func (x *SshNode) Type() string {
	return "ssh"
}

// The New method is used to create a new instance of SshNode
func (x *SshNode) New() types.Node {
	return &SshNode{Config: SshConfiguration{
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		Password: "password",
	}}
}

// The Init method is used to initialize components, usually for component parameter configuration or client-side initialization
func (x *SshNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		// Obtain the SSH connection parameters from the configuration
		sshConfig := x.Config
		// If the argument is not null, create an SSH client object
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
		if x.Config.Cmd == "" {
			return SshCmdEmptyErr
		}
		x.cmdTemplate, err = el.NewTemplate(x.Config.Cmd)
		if err != nil {
			return err
		}
		x.hasVar = x.cmdTemplate.HasVar()
	}
	return err
}

// The OnMsg method is used to process messages, and every piece of data entering the component is processed by this function
func (x *SshNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error

	// Securely retrieve client references
	x.clientMutex.RLock()
	client := x.client
	x.clientMutex.RUnlock()

	if client == nil {
		ctx.TellFailure(msg, SshClientNotInitErr)
		return
	}

	// Command to get shell
	var evn map[string]interface{}
	if x.hasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	cmd := x.cmdTemplate.ExecuteAsString(evn)
	var output []byte
	var session *ssh.Session
	// If there is an SSH client object, create an SSH session and execute the remote shell command, retrieving its output or error message
	if session, err = client.NewSession(); err == nil {
		defer session.Close()
		output, err = session.CombinedOutput(cmd)

		msg.SetData(string(output))
		msg.DataType = types.TEXT

		if err != nil {
			ctx.TellFailure(msg, err)
		} else {
			// Send the output results as new messages to the next component
			ctx.TellSuccess(msg)
		}
	} else {
		ctx.TellFailure(msg, err)
	}
}

// The Destroy method is used to destroy components and perform some resource release operations
func (x *SshNode) Destroy() {
	x.clientMutex.Lock()
	defer x.clientMutex.Unlock()

	if x.client != nil {
		_ = x.client.Close()
		x.client = nil
	}
}

// Desc returns the component description
func (x *SshNode) Desc() string {
	return "SSH remote command execution. Connects to remote host and executes shell commands. cmd supports ${metadata.key} and ${msg.key} substitution. Routes to Success/Failure"
}
