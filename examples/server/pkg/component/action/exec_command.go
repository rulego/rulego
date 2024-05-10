package action

import (
	"bytes"
	"errors"
	"examples/server/internal/constants"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// ErrCmdNotAllowed 不允许执行的命令
var ErrCmdNotAllowed = errors.New("cmd not allowed error")

// KeyExecCommandNodeWhitelist 节点白名单配置
const KeyExecCommandNodeWhitelist = "execCommandNodeWhitelist"

func init() {
	rulego.Registry.Register(&ExecCommandNode{})
}

// ExecCommandNodeConfiguration 节点配置
type ExecCommandNodeConfiguration struct {
	// Command 执行的命令
	Command string
	// Args 命令参数
	Args []string
	// Log 是否打印标准输出
	Log bool
	//是否把输出输出到下一个节点
	ReplaceData bool
}

// ExecCommandNode 实现命令执行
type ExecCommandNode struct {
	// 节点配置
	Config ExecCommandNodeConfiguration
	// 白名单命令列表
	CommandWhitelist []string
}

// Type 组件类型
func (x *ExecCommandNode) Type() string {
	return "ci/exec"
}

func (x *ExecCommandNode) New() types.Node {
	return &ExecCommandNode{}
}

// Init 初始化
func (x *ExecCommandNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	x.CommandWhitelist = strings.Split(ruleConfig.Properties.GetValue(KeyExecCommandNodeWhitelist), ",")
	return nil
}

// OnMsg 处理消息
func (x *ExecCommandNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 创建 metadata 的副本并添加消息内容
	metadataCopy := msg.Metadata.Copy()
	metadataCopy.PutValue("msg.data", msg.Data)
	metadataCopy.PutValue("msg.type", msg.Type)

	// 替换命令中的占位符
	command := str.SprintfDict(x.Config.Command, metadataCopy.Values())

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
				args = append(args, str.SprintfDict(item, metadataCopy.Values()))
			}
		} else {
			args = append(args, str.SprintfDict(arg, metadataCopy.Values()))
		}
	}

	// 执行命令
	cmd := exec.Command(command, args...)
	// 设置命令的工作目录
	cmd.Dir = msg.Metadata.GetValue(constants.KeyWorkDir)
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
		msg.Data = stdoutBuf.String()
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
		//buf:          bufOut,
	}
	errWriter := &OnDebugWriter{
		ctx:          ctx,
		msg:          msgCopy,
		relationType: "error",
		chainId:      chainId,
		//buf:          bufErr,
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
	//w.buf.WriteString(w.msg.Data)
	// 调用 OnDebug 方法来记录日志
	w.ctx.Config().OnDebug(w.chainId, types.Log, w.ctx.Self().GetNodeId().Id, w.msg, w.relationType, nil)
	// 返回写入的字节数和nil错误
	return len(p), nil
}

// ansiToHTML 函数将ANSI颜色代码转换为HTML颜色代码
func ansiToHTML(text string) string {
	// 正则表达式匹配ANSI颜色代码
	ansiRegex := regexp.MustCompile(`\x1B\[\d+(;\d+)*m`)
	// 将文本分割为ANSI代码和普通文本
	parts := ansiRegex.Split(text, -1)
	var html strings.Builder

	for i, part := range parts {
		if i%2 == 0 {
			// 普通文本部分
			html.WriteString(part)
		} else {
			// ANSI代码部分
			code := part[2:] // 忽略前两个字符"\x1B["
			color, _ := parseAnsiColorCode(code)
			if color != "" {
				html.WriteString(fmt.Sprintf(`<span style="color: %s">`, color))
				html.WriteString(parts[i+1])
				html.WriteString(`</span>`)
			}
		}
	}

	return html.String()
}

// parseAnsiColorCode 函数解析ANSI颜色代码并返回相应的CSS颜色值
func parseAnsiColorCode(code string) (string, error) {
	// 根据ANSI代码解析颜色（这里只是一个简单的映射示例，实际需要更复杂的逻辑）
	colorMap := map[string]string{
		"31": "red",
		"32": "green",
		"33": "yellow",
		// 根据需要添加更多颜色映射...
	}

	if color, ok := colorMap[code]; ok {
		return color, nil
	}
	return "", fmt.Errorf("unsupported ansi color code: %s", code)
}
