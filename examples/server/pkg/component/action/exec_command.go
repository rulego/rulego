package action

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// ErrCmdNotAllowed 不允许执行的命令
var ErrCmdNotAllowed = errors.New("cmd not allowed error")

// ExecCommandNodeWhitelistKey 节点白名单配置
const ExecCommandNodeWhitelistKey = "ExecCommandNodeWhitelist"

func init() {
	rulego.Registry.Register(&ExecCommandNode{})
}

// ExecCommandNodeConfiguration 节点配置
type ExecCommandNodeConfiguration struct {
	// Command 执行的命令
	Command string
	// Args 命令参数
	Args []string
	// Log 是否记录日志
	Log bool
}

// ExecCommandNode 实现命令执行
type ExecCommandNode struct {
	// 节点配置
	Config ExecCommandNodeConfiguration
	// 白名单命令列表
	CommandWhitelist []string
	//// Logger 实例，用于记录日志
	//Logger types.Logger
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
	//x.Logger = ruleConfig.Logger
	x.CommandWhitelist = strings.Split(ruleConfig.Properties.GetValue(ExecCommandNodeWhitelistKey), ",")
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
		args = append(args, str.SprintfDict(arg, metadataCopy.Values()))
	}

	// 执行命令
	cmd := exec.Command(command, args...)

	// 创建缓冲区来保存命令的组合输出
	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	msgCopy := msg.Copy()
	// 设置标准输出管道
	stdoutPipe, _ := cmd.StdoutPipe()
	go func() {
		defer wg.Done()
		// 实时记录日志
		copyAndLog(ctx, msgCopy, &stdoutBuf, stdoutPipe, "info")
	}()

	// 设置标准错误管道
	stderrPipe, _ := cmd.StderrPipe()
	go func() {
		defer wg.Done()
		// 实时记录日志
		copyAndLog(ctx, msgCopy, &stderrBuf, stderrPipe, "error")
	}()

	// 启动命令
	err := cmd.Start()
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	// 等待管道操作完成
	wg.Wait()

	// 等待命令执行完成
	err = cmd.Wait()
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	// 组合输出
	combinedOutput := stdoutBuf.String() + stderrBuf.String()
	msg.Data = combinedOutput
	ctx.TellSuccess(msg)
}

// Destroy 销毁
func (x *ExecCommandNode) Destroy() {
}

// copyAndLog 读取管道输出并记录到日志和缓冲区
func copyAndLog(ctx types.RuleContext, msg types.RuleMsg, buf *bytes.Buffer, pipe io.ReadCloser, relationType string) {
	var chainId = ""
	if ctx.RuleChain() != nil {
		chainId = ctx.RuleChain().GetNodeId().Id
	}
	reader := io.TeeReader(pipe, buf)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		msg.Data = scanner.Text()
		ctx.Config().OnDebug(chainId, types.In, ctx.Self().GetNodeId().Id, msg, relationType, nil)
	}
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
