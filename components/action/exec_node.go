package action

import (
	"bytes"
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"io"
	"os/exec"
	"strings"
)

// ErrCmdNotAllowed 不允许执行的命令
var ErrCmdNotAllowed = errors.New("cmd not allowed error")

const (
	// KeyExecNodeWhitelist ExecCommandNode node whitelist list
	KeyExecNodeWhitelist = "execNodeWhitelist"
	//KeyWorkDir cmd working directory
	KeyWorkDir = "workDir"
)

func init() {
	Registry.Add(&ExecCommandNode{})
}

// ExecCommandNodeConfiguration 节点配置
type ExecCommandNodeConfiguration struct {
	// Cmd 执行的命令，可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Cmd string
	// Args 命令参数，可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Args []string
	// Log 是否打印标准输出。true:命令标准输出会触发OnDebug函数输出
	Log bool
	//是否把标准输出到下一个节点
	ReplaceData bool
}

// ExecCommandNode 执行本地命令，在白名单的命令才允许执行，通过config.Properties `key=execNodeWhitelist`，设置白名单，多个与`,`隔开
// 示例：config.Properties.PutValue(KeyExecNodeWhitelist,"cd,ls,go")
// 允许通过上一个节点通过元数据`key=workDir`，设置命令的执行目录，如：Metadata.PutValue("workDir","./data")
type ExecCommandNode struct {
	// 节点配置
	Config ExecCommandNodeConfiguration
	// 白名单命令列表
	CommandWhitelist []string
}

// Type 组件类型
func (x *ExecCommandNode) Type() string {
	return "exec"
}

func (x *ExecCommandNode) New() types.Node {
	return &ExecCommandNode{}
}

// Init 初始化
func (x *ExecCommandNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	x.CommandWhitelist = strings.Split(ruleConfig.Properties.GetValue(KeyExecNodeWhitelist), ",")
	return nil
}

// OnMsg 处理消息
func (x *ExecCommandNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {

	evn := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	// 替换命令中的占位符
	command := str.ExecuteTemplate(x.Config.Cmd, evn)

	// 检查命令是否在白名单中
	if !isCommandWhitelisted(command, x.CommandWhitelist) {
		ctx.TellFailure(msg, ErrCmdNotAllowed)
		return
	}

	// 替换参数中的占位符
	var args []string
	for _, arg := range x.Config.Args {
		if !strings.HasPrefix(arg, "\"") {
			v := strings.Split(arg, " ")
			for _, item := range v {
				args = append(args, str.ExecuteTemplate(item, evn))
			}
		} else {
			args = append(args, str.ExecuteTemplate(arg, evn))
		}
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
			msg.Data = stdoutStr
		} else {
			msg.Data = stderrBuf.String()
		}
	}
	ctx.TellSuccess(msg)
}

// Destroy 销毁
func (x *ExecCommandNode) Destroy() {
}

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

// isCommandWhitelisted 检查命令是否在白名单中
func isCommandWhitelisted(command string, whitelist []string) bool {
	for _, whitelistedCommand := range whitelist {
		if command == whitelistedCommand {
			return true
		}
	}
	return false
}

type OnDebugWriter struct {
	ctx          types.RuleContext
	msg          types.RuleMsg
	relationType string
	chainId      string
}

func (w *OnDebugWriter) Write(p []byte) (n int, err error) {
	// 将接收到的数据转换为字符串
	w.msg.Data = string(p)
	// 调用 OnDebug 方法来记录日志
	w.ctx.Config().OnDebug(w.chainId, types.Log, w.ctx.GetSelfId(), w.msg, w.relationType, nil)
	// 返回写入的字节数和nil错误
	return len(p), nil
}
