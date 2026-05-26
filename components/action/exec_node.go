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
	"github.com/rulego/rulego/utils/str"
)

// ErrCmdNotAllowed 命令不在白名单中的错误
// ErrCmdNotAllowed is returned when attempting to execute a command not in the whitelist.
var ErrCmdNotAllowed = errors.New("cmd not allowed error")

// ErrCmdDenied 命令在黑名单中的错误
// ErrCmdDenied is returned when attempting to execute a denied command or with denied arguments.
var ErrCmdDenied = errors.New("cmd is denied error")

const (
	// KeyExecNodeWhitelist 命令白名单的配置键
	// KeyExecNodeWhitelist is the configuration key for the ExecCommandNode command whitelist.
	KeyExecNodeWhitelist = "execNodeWhitelist"

	// KeyExecNodeMode 安全模式的配置键
	// KeyExecNodeMode is the configuration key for the security mode (allow or deny).
	KeyExecNodeMode = "execNodeMode"

	// KeyExecNodeDeny 命令黑名单的配置键
	// KeyExecNodeDeny is the configuration key for the ExecCommandNode command deny list.
	KeyExecNodeDeny = "execNodeDeny"

	// KeyExecNodeDenyArgs 拒绝参数模式的配置键
	// KeyExecNodeDenyArgs is the configuration key for denied argument patterns.
	KeyExecNodeDenyArgs = "execNodeDenyArgs"

	// KeyWorkDir 工作目录的元数据键
	// KeyWorkDir is the metadata key for specifying the command working directory.
	KeyWorkDir = "workDir"
)

// SecurityMode 安全模式类型
// SecurityMode defines the security mode for command execution.
type SecurityMode string

const (
	// ModeAllow 白名单模式：只允许列表中的命令（默认）
	// ModeAllow allows only commands in the whitelist.
	ModeAllow SecurityMode = "allow"
	// ModeDeny 黑名单模式：允许所有非拒绝列表的命令
	// ModeDeny allows all commands except those in the deny list.
	ModeDeny SecurityMode = "deny"
)

// init 注册ExecCommandNode组件
// init registers the ExecCommandNode component with the default registry.
func init() {
	Registry.Add(&ExecCommandNode{})
}

// ExecCommandNodeConfiguration ExecCommandNode配置结构
// ExecCommandNodeConfiguration defines the configuration structure for the ExecCommandNode component.
type ExecCommandNodeConfiguration struct {
	// Cmd is the command to execute. Supports ${metadata.key} and ${msg.key} substitution.
	Cmd string `json:"cmd" label:"Command" desc:"Command to execute. Supports ${metadata.key} and ${msg.key} substitution" required:"true"`
	// Args are the command arguments. Each supports variable substitution.
	Args []string `json:"args" label:"Arguments" desc:"Command arguments, each supports ${metadata.key} and ${msg.key} substitution"`
	// Log controls whether to output command stdout to the debug log.
	Log bool `json:"log" label:"Log Output" desc:"true=redirect command stdout to debug log"`
	// ReplaceData controls whether to replace the message data with command output.
	ReplaceData bool `json:"replaceData" label:"Replace Data" desc:"true=replace message data with command output"`
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
// 2. 安全检查：根据安全模式验证命令 - Security check based on mode (allow/deny)
// 3. 命令执行：设置工作目录并执行命令 - Command execution with working directory
// 4. 输出处理：根据配置处理标准输出和错误输出 - Output handling based on configuration
//
// 安全模式 - Security modes:
//   - allow(白名单模式)：命令必须在白名单中 - Command must be in whitelist
//   - deny(黑名单模式)：允许所有非拒绝列表的命令 - Allow all except denied commands
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

	// Mode 安全模式：allow(白名单模式) 或 deny(黑名单模式)
	// Mode specifies the security mode: allow (whitelist) or deny (blacklist).
	Mode SecurityMode

	// CommandDeny 拒绝的命令列表
	// CommandDeny contains the list of denied commands.
	CommandDeny []string

	// DenyArgs 拒绝的参数模式列表
	// DenyArgs contains the list of denied argument patterns.
	DenyArgs []string

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
	x.CommandWhitelist = splitAndFilter(ruleConfig.Properties.GetValue(KeyExecNodeWhitelist))
	x.Mode = SecurityMode(ruleConfig.Properties.GetValue(KeyExecNodeMode))
	if x.Mode == "" {
		x.Mode = ModeAllow
	}
	x.CommandDeny = splitAndFilter(ruleConfig.Properties.GetValue(KeyExecNodeDeny))
	x.DenyArgs = splitAndFilter(ruleConfig.Properties.GetValue(KeyExecNodeDenyArgs))

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
	return str.Contains(x.CommandWhitelist, command)
}

// isCommandDenied 检查命令是否在黑名单中
// isCommandDenied checks if a command is in the deny list.
func (x *ExecCommandNode) isCommandDenied(command string) bool {
	return str.Contains(x.CommandDeny, command)
}

// hasDeniedArgs 检查完整命令是否包含拒绝的参数模式
// hasDeniedArgs checks if the full command contains any denied argument patterns.
func (x *ExecCommandNode) hasDeniedArgs(fullCommand string) bool {
	for _, denied := range x.DenyArgs {
		if strings.Contains(fullCommand, denied) {
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

	// 使用模板替换参数中的占位符
	var args []string
	for _, argTemplate := range x.template.ArgsTemplate {
		processedArg := argTemplate.ExecuteAsString(evn)
		args = append(args, processedArg)
	}

	// 构建完整命令字符串用于参数模式检查
	fullCommand := command
	if len(args) > 0 {
		fullCommand = command + " " + strings.Join(args, " ")
	}

	// 1. 黑名单检查（始终生效）
	if x.isCommandDenied(command) {
		ctx.TellFailure(msg, ErrCmdDenied)
		return
	}

	// 2. 拒绝参数检查（始终生效）
	if x.hasDeniedArgs(fullCommand) {
		ctx.TellFailure(msg, ErrCmdDenied)
		return
	}

	// 3. 模式检查
	if x.Mode == ModeAllow && !x.isCommandWhitelisted(command) {
		ctx.TellFailure(msg, ErrCmdNotAllowed)
		return
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

// splitAndFilter 按逗号分割字符串并过滤空项
// splitAndFilter splits a string by comma and filters out empty items.
func splitAndFilter(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// Desc returns the component description
func (x *ExecCommandNode) Desc() string {
	return "Execute local system commands with security controls (whitelist/deny). Supports ${metadata.key} and ${msg.key} substitution. Routes to Success/Failure"
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
