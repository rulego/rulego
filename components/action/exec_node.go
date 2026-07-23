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

// The error of the ErrCmdNotAllowed command not being on the whitelist
// ErrCmdNotAllowed is returned when attempting to execute a command not in the whitelist.
var ErrCmdNotAllowed = errors.New("cmd not allowed error")

// Error in the ErrCmdDenied command in the blacklist
// ErrCmdDenied is returned when attempting to execute a denied command or with denied arguments.
var ErrCmdDenied = errors.New("cmd is denied error")

const (
	// KeyExecNodeWhitelist command Whitelist configuration key
	// KeyExecNodeWhitelist is the configuration key for the ExecCommandNode command whitelist.
	KeyExecNodeWhitelist = "execNodeWhitelist"

	// KeyExecNodeMode Configuration key for safe mode
	// KeyExecNodeMode is the configuration key for the security mode (allow or deny).
	KeyExecNodeMode = "execNodeMode"

	// KeyExecNodeDeny command blacklist configuration key
	// KeyExecNodeDeny is the configuration key for the ExecCommandNode command deny list.
	KeyExecNodeDeny = "execNodeDeny"

	// KeyExecNodeDenyArgs rejects the configuration key for parameter mode
	// KeyExecNodeDenyArgs is the configuration key for denied argument patterns.
	KeyExecNodeDenyArgs = "execNodeDenyArgs"

	// KeyWorkDir is the metadata key for the working directory
	// KeyWorkDir is the metadata key for specifying the command working directory.
	KeyWorkDir = "workDir"
)

// SecurityMode type
// SecurityMode defines the security mode for command execution.
type SecurityMode string

const (
	// ModeAllow Whitelist Mode: Only allows commands in the list (default)
	// ModeAllow allows only commands in the whitelist.
	ModeAllow SecurityMode = "allow"
	// ModeDeny Blacklist Mode: Allows all commands that are not rejected from the list
	// ModeDeny allows all commands except those in the deny list.
	ModeDeny SecurityMode = "deny"
)

// init registers the ExecCommandNode component
// init registers the ExecCommandNode component with the default registry.
func init() {
	Registry.Add(&ExecCommandNode{})
}

// ExecCommandNodeConfiguration ExecCommandNode configuration structure
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

// ExecCommandTemplate executes the command template structure
// ExecCommandTemplate defines the template structure for command execution.
type ExecCommandTemplate struct {
	// CmdTemplate command template
	// CmdTemplate holds the command template for variable substitution
	CmdTemplate el.Template

	// ArgsTemplate parameter template list
	// ArgsTemplate holds the argument templates for variable substitution
	ArgsTemplate []el.Template

	// Does HasVar contain variables?
	// HasVar indicates whether the template contains variables
	HasVar bool
}

// ExecCommandNode is the action component that executes local system commands and has security controls
// ExecCommandNode is an action component that executes local system commands with security controls.
//
// Core algorithm:
// Core Algorithm:
// 1. Variable Substitution: Parse ${metadata.key} and ${msg.key} in commands and arguments - Variable substitution in command and arguments
// 2. Security check: Security check based on mode (allow/deny)
// 3. Command execution: Set the working directory and execute commands - Command execution with working directory
// 4. Output handling: Output handling based on configuration
//
// Security modes:
//   - allow: Command must be in whitelist
//   - deny (Blacklist Mode): Allow all except denied commands
//
// Output handling modes:
//   - Log mode: output to debug logging
//   - Replace mode: Output replaces message data
type ExecCommandNode struct {
	// Config defines the node configuration
	// Config holds the node configuration including command and execution options
	Config ExecCommandNodeConfiguration

	// CommandWhitelist is a list of allowed commands
	// CommandWhitelist contains the list of allowed commands for security validation
	CommandWhitelist []string

	// Mode Safe Mode: allow (whitelist mode) or deny (blacklist mode)
	// Mode specifies the security mode: allow (whitelist) or deny (blacklist).
	Mode SecurityMode

	// CommandDeny rejects the command list
	// CommandDeny contains the list of denied commands.
	CommandDeny []string

	// List of parameter patterns rejected by DenyArgs
	// DenyArgs contains the list of denied argument patterns.
	DenyArgs []string

	// template command
	// template holds the compiled command and arguments templates
	template *ExecCommandTemplate
}

// Type returns the component type
// Type returns the component type identifier.
func (x *ExecCommandNode) Type() string {
	return "exec"
}

// New creates an instance
// New creates a new instance.
func (x *ExecCommandNode) New() types.Node {
	return &ExecCommandNode{}
}

// Init initializes the component
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

	// Build command templates
	if template, err := x.buildCommandTemplate(&x.Config); err != nil {
		return err
	} else {
		x.template = template
	}
	return nil
}

// buildCommandTemplate: Build a command template
// buildCommandTemplate builds command templates for variable substitution.
func (x *ExecCommandNode) buildCommandTemplate(config *ExecCommandNodeConfiguration) (*ExecCommandTemplate, error) {
	template := &ExecCommandTemplate{}

	// Build command templates
	cmdTemplate, err := el.NewTemplate(config.Cmd)
	if err != nil {
		return nil, err
	}
	template.CmdTemplate = cmdTemplate
	template.HasVar = cmdTemplate.HasVar()

	// Build parameter templates - Maintain the original splitting logic
	for _, arg := range config.Args {
		// If the parameter does not start with quotes, split it by space
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
			// If it starts with quotation marks, treat it as a whole
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

// isCommandWhitelisted checks whether the command is on the whitelist
// isCommandWhitelisted checks if a command is allowed by the whitelist configuration.
func (x *ExecCommandNode) isCommandWhitelisted(command string) bool {
	return str.Contains(x.CommandWhitelist, command)
}

// isCommandDenied checks whether the command is on the blacklist
// isCommandDenied checks if a command is in the deny list.
func (x *ExecCommandNode) isCommandDenied(command string) bool {
	return str.Contains(x.CommandDeny, command)
}

// hasDeniedArgs checks whether the full command contains a rejection parameter pattern
// hasDeniedArgs checks if the full command contains any denied argument patterns.
func (x *ExecCommandNode) hasDeniedArgs(fullCommand string) bool {
	for _, denied := range x.DenyArgs {
		if strings.Contains(fullCommand, denied) {
			return true
		}
	}
	return false
}

// OnMsg processes messages and executes configured commands
// OnMsg processes incoming messages by executing the configured command with security validation.
func (x *ExecCommandNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var evn map[string]interface{}
	if x.template.HasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}

	// Use template to replace placeholders in commands
	command := x.template.CmdTemplate.ExecuteAsString(evn)

	// Use templates to replace placeholders in parameters
	var args []string
	for _, argTemplate := range x.template.ArgsTemplate {
		processedArg := argTemplate.ExecuteAsString(evn)
		args = append(args, processedArg)
	}

	// Construct a complete command string for parameter mode checking
	fullCommand := command
	if len(args) > 0 {
		fullCommand = command + " " + strings.Join(args, " ")
	}

	// 1. Blacklist Check (Always Effective)
	if x.isCommandDenied(command) {
		ctx.TellFailure(msg, ErrCmdDenied)
		return
	}

	// 2. Reject parameter checks (always active)
	if x.hasDeniedArgs(fullCommand) {
		ctx.TellFailure(msg, ErrCmdDenied)
		return
	}

	// 3. Pattern check
	if x.Mode == ModeAllow && !x.isCommandWhitelisted(command) {
		ctx.TellFailure(msg, ErrCmdNotAllowed)
		return
	}

	// Execute the command
	cmd := exec.Command(command, args...)
	// Set the working directory of the command
	cmd.Dir = msg.Metadata.GetValue(KeyWorkDir)
	var stdoutBuf, stderrBuf bytes.Buffer
	if x.Config.Log {
		x.printLog(ctx, msg, cmd, &stdoutBuf, &stderrBuf)
	} else if x.Config.ReplaceData {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	}

	// Start command
	if err := cmd.Start(); err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	// Wait for the command to finish
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

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *ExecCommandNode) Destroy() {
	// No resources to clean
	// No resources to clean up
}

// printLog configuration command output redirects to the debug log
// printLog configures command output redirection for debug logging.
func (x *ExecCommandNode) printLog(ctx types.RuleContext, msg types.RuleMsg, cmd *exec.Cmd, bufOut *bytes.Buffer, bufErr *bytes.Buffer) {
	// Enable logging records
	var chainId = ""
	if ctx.RuleChain() != nil {
		chainId = ctx.RuleChain().GetNodeId().Id
	}
	msgCopy := msg.Copy()
	// Create a DebugWriter instance
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
	// Redirect command output to DebugWriter
	cmd.Stdout = io.MultiWriter(bufOut, debugWriter)
	cmd.Stderr = io.MultiWriter(bufErr, errWriter)
}

// splitAndFilter splits the string by comma and filters out empty entries
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

// OnDebugWriter redirects command output to the custom writer for debugging the rule engine system
// OnDebugWriter is a custom writer that redirects command output to the rule engine's debug system.
type OnDebugWriter struct {
	// CTX rules handle context
	// ctx provides access to the rule processing context for debug callbacks
	ctx types.RuleContext

	// The message contained in msg debugging output
	// msg holds the message to include in debug output
	msg types.RuleMsg

	// relationType Debugging relationship type ("info" or "error")
	// relationType specifies the debug relation type ("info" or "error")
	relationType string

	// chainId rules: chain ID
	// chainId identifies the rule chain for debug context
	chainId string
}

// Write to implement io.Writer interface, captures command output, and sends it to the debug log
// Write implements the io.Writer interface to capture command output and send it to debug logging.
func (w *OnDebugWriter) Write(p []byte) (n int, err error) {
	// Convert the received data into strings
	w.msg.SetData(string(p))
	// Call the OnDebug method to log the log
	w.ctx.Config().OnDebug(w.chainId, types.Log, w.ctx.GetSelfId(), w.msg, w.relationType, nil)
	// Returns the number of bytes written and nil errors
	return len(p), nil
}
