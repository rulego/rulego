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

package action

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
)

// ErrCmdNotAllowed 命令不在白名单中的错误
// ErrCmdNotAllowed is returned when attempting to execute a command not in the whitelist.
var ErrCmdNotAllowed = errors.New("cmd not allowed error")

const (
	// KeyExecNodeWhitelist 命令白名单的配置键
	// KeyExecNodeWhitelist is the configuration key for the ExecCommandNode command whitelist.
	KeyExecNodeWhitelist = "execNodeWhitelist"

	// KeyWorkDir 工作目录的元数据键
	// KeyWorkDir is the metadata key for specifying the command working directory.
	KeyWorkDir = "workDir"
)

// init 注册ExecCommandNode组件
// init registers the ExecCommandNode component with the default registry.
func init() {
	Registry.Add(&ExecCommandNode{})
}

// ExecCommandNodeConfiguration ExecCommandNode配置结构
// ExecCommandNodeConfiguration defines the configuration structure for the ExecCommandNode component.
type ExecCommandNodeConfiguration struct {
	// Cmd 要执行的命令，支持${metadata.key}和${msg.key}变量替换
	// Cmd specifies the command to execute.
	// Supports variable substitution using ${metadata.key} and ${msg.key} syntax.
	Cmd string

	// Args 命令参数，每个参数支持变量替换
	// Args specifies the command arguments.
	// Each argument supports variable substitution using ${metadata.key} and ${msg.key} syntax.
	Args []string

	// Log 是否将命令标准输出到调试日志
	// Log controls whether to output command stdout to the debug log.
	Log bool

	// ReplaceData 是否用命令输出替换消息数据
	// ReplaceData controls whether to replace the message data with command output.
	ReplaceData bool
}

// ExecCommandTemplate 执行命令模板结构
// ExecCommandTemplate defines the template structure for command execution.
type ExecCommandTemplate struct {
	// CmdTemplate 命令模板
	// CmdTemplate holds the command template for variable substitution
	CmdTemplate el.Template

	// ArgsTemplate 参数模板列表
	// ArgsTemplate holds the argument templates for variable substitution
	ArgsTemplate []el.Template

	// HasVar 是否包含变量
	// HasVar indicates whether the template contains variables
	HasVar bool
}

// ExecCommandNode 执行本地系统命令的动作组件，具有安全控制
// ExecCommandNode is an action component that executes local system commands with security controls.
//
// 核心算法：
// Core Algorithm:
// 1. 变量替换：解析命令和参数中的${metadata.key}和${msg.key} - Variable substitution in command and arguments
// 2. 白名单验证：检查命令是否在允许列表中 - Whitelist validation for security
// 3. 命令执行：设置工作目录并执行命令 - Command execution with working directory
// 4. 输出处理：根据配置处理标准输出和错误输出 - Output handling based on configuration
//
// 安全特性 - Security features:
//   - 命令白名单验证 - Command whitelist validation
//   - 变量替换支持 - Variable substitution support
//   - 工作目录控制 - Working directory control
//
// 输出处理模式 - Output handling modes:
//   - Log模式：输出发送到调试日志 - Log mode: output to debug logging
//   - Replace模式：输出替换消息数据 - Replace mode: output replaces message data
type ExecCommandNode struct {
	// Config 节点配置
	// Config holds the node configuration including command and execution options
	Config ExecCommandNodeConfiguration

	// CommandWhitelist 允许的命令列表
	// CommandWhitelist contains the list of allowed commands for security validation
	CommandWhitelist []string

	// template 命令模板
	// template holds the compiled command and arguments templates
	template *ExecCommandTemplate
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *ExecCommandNode) Type() string {
	return "exec"
}

// New 创建新实例
// New creates a new instance.
func (x *ExecCommandNode) New() types.Node {
	return &ExecCommandNode{}
}

// Init 初始化组件
// Init initializes the component.
func (x *ExecCommandNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	x.CommandWhitelist = strings.Split(ruleConfig.Properties.GetValue(KeyExecNodeWhitelist), ",")
	
	// 构建命令模板
	if template, err := x.buildCommandTemplate(&x.Config); err != nil {
		return err
	} else {
		x.template = template
	}
	return nil
}

// buildCommandTemplate 构建命令模板
// buildCommandTemplate builds command templates for variable substitution.
func (x *ExecCommandNode) buildCommandTemplate(config *ExecCommandNodeConfiguration) (*ExecCommandTemplate, error) {
	template := &ExecCommandTemplate{}
	
	// 构建命令模板
	cmdTemplate, err := el.NewTemplate(config.Cmd)
	if err != nil {
		return nil, err
	}
	template.CmdTemplate = cmdTemplate
	template.HasVar = cmdTemplate.HasVar()
	
	// 构建参数模板 - 保持原有的分割逻辑
	for _, arg := range config.Args {
		// 如果参数不以引号开头，按空格分割
		if !strings.HasPrefix(arg, "\"") {
			v := strings.Split(arg, " ")
			for _, item := range v {
				argTemplate, err := el.NewTemplate(item)
				if err != nil {
					return nil, err
				}
				template.ArgsTemplate = append(template.ArgsTemplate, argTemplate)
				if argTemplate.HasVar() {
					template.HasVar = true
				}
			}
		} else {
			// 如果以引号开头，作为整体处理
			argTemplate, err := el.NewTemplate(arg)
			if err != nil {
				return nil, err
			}
			template.ArgsTemplate = append(template.ArgsTemplate, argTemplate)
			if argTemplate.HasVar() {
				template.HasVar = true
			}
		}
	}
	
	return template, nil
}

// isCommandWhitelisted 检查命令是否在白名单中
// isCommandWhitelisted checks if a command is allowed by the whitelist configuration.
func (x *ExecCommandNode) isCommandWhitelisted(command string) bool {
	for _, whitelistedCommand := range x.CommandWhitelist {
		if command == whitelistedCommand {
			return true
		}
	}
	return false
}

// OnMsg 处理消息，执行配置的命令
// OnMsg processes incoming messages by executing the configured command with security validation.
func (x *ExecCommandNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var evn map[string]interface{}
	if x.template.HasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	
	// 使用模板替换命令中的占位符
	command := x.template.CmdTemplate.ExecuteAsString(evn)

	// 检查命令是否在白名单中
	if !x.isCommandWhitelisted(command) {
		ctx.TellFailure(msg, ErrCmdNotAllowed)
		return
	}

	// 使用模板替换参数中的占位符
	var args []string
	for _, argTemplate := range x.template.ArgsTemplate {
		processedArg := argTemplate.ExecuteAsString(evn)
		args = append(args, processedArg)
	}

	// 执行命令
	cmd := exec.Command(command, args...)
	// 设置命令的工作目录
	cmd.Dir = msg.Metadata.GetValue(KeyWorkDir)
	var stdoutBuf, stderrBuf bytes.Buffer
	if x.Config.Log {
		x.printLog(ctx, msg, cmd, &stdoutBuf, &stderrBuf)
	} else if x.Config.ReplaceData {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	// 等待命令执行完成
	if err := cmd.Wait(); err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	if x.Config.ReplaceData {
		stdoutStr := stdoutBuf.String()
		if stdoutStr != "" {
			msg.SetData(stdoutStr)
		} else {
			msg.SetData(stderrBuf.String())
		}
	}
	ctx.TellSuccess(msg)
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *ExecCommandNode) Destroy() {
	// 无资源需要清理
	// No resources to clean up
}

// printLog 配置命令输出重定向到调试日志
// printLog configures command output redirection for debug logging.
func (x *ExecCommandNode) printLog(ctx types.RuleContext, msg types.RuleMsg, cmd *exec.Cmd, bufOut *bytes.Buffer, bufErr *bytes.Buffer) {
	// 启用日志记录
	var chainId = ""
	if ctx.RuleChain() != nil {
		chainId = ctx.RuleChain().GetNodeId().Id
	}
	msgCopy := msg.Copy()
	// 创建 DebugWriter 实例
	debugWriter := &OnDebugWriter{
		ctx:          ctx,
		msg:          msgCopy,
		relationType: "info",
		chainId:      chainId,
	}
	errWriter := &OnDebugWriter{
		ctx:          ctx,
		msg:          msgCopy,
		relationType: "error",
		chainId:      chainId,
	}
	// 将命令的输出重定向到 DebugWriter
	cmd.Stdout = io.MultiWriter(bufOut, debugWriter)
	cmd.Stderr = io.MultiWriter(bufErr, errWriter)
}



// OnDebugWriter 将命令输出重定向到规则引擎调试系统的自定义写入器
// OnDebugWriter is a custom writer that redirects command output to the rule engine's debug system.
type OnDebugWriter struct {
	// ctx 规则处理上下文
	// ctx provides access to the rule processing context for debug callbacks
	ctx types.RuleContext

	// msg 调试输出中包含的消息
	// msg holds the message to include in debug output
	msg types.RuleMsg

	// relationType 调试关系类型（"info" 或 "error"）
	// relationType specifies the debug relation type ("info" or "error")
	relationType string

	// chainId 规则链ID
	// chainId identifies the rule chain for debug context
	chainId string
}

// Write 实现io.Writer接口，捕获命令输出并发送到调试日志
// Write implements the io.Writer interface to capture command output and send it to debug logging.
func (w *OnDebugWriter) Write(p []byte) (n int, err error) {
	// 将接收到的数据转换为字符串
	w.msg.SetData(string(p))
	// 调用 OnDebug 方法来记录日志
	w.ctx.Config().OnDebug(w.chainId, types.Log, w.ctx.GetSelfId(), w.msg, w.relationType, nil)
	// 返回写入的字节数和nil错误
	return len(p), nil
}
