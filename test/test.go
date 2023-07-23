package test

import (
	"rulego/api/types"
	"time"
)

//NodeTestRuleContext
//只为测试单节点，临时创建的上下文
//无法把多个节点组成链式
//callback 回调处理结果
type NodeTestRuleContext struct {
	config   types.Config
	callback func(msg types.RuleMsg, relationType string)
}

func NewRuleContext(config types.Config, callback func(msg types.RuleMsg, relationType string)) types.RuleContext {
	return &NodeTestRuleContext{
		config:   config,
		callback: callback,
	}
}
func (ctx *NodeTestRuleContext) TellSuccess(msg types.RuleMsg) {
	ctx.callback(msg, types.Success)
}
func (ctx *NodeTestRuleContext) TellFailure(msg types.RuleMsg, err error) {
	ctx.callback(msg, types.Failure)
}
func (ctx *NodeTestRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	for _, relationType := range relationTypes {
		ctx.callback(msg, relationType)
	}

}
func (ctx *NodeTestRuleContext) TellSelf(msg types.RuleMsg, delayMs int64) {
	time.AfterFunc(time.Millisecond*time.Duration(delayMs), func() {
		ctx.callback(msg, types.Success)
	})
}
func (ctx *NodeTestRuleContext) NewMsg(msgType string, metaData types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}
func (ctx *NodeTestRuleContext) GetSelfId() string {
	return ""
}

func (ctx *NodeTestRuleContext) Config() types.Config {
	return ctx.config
}

func (ctx *NodeTestRuleContext) SetEndFunc(onEndFunc func(msg types.RuleMsg, err error)) types.RuleContext {
	return ctx
}

func (ctx *NodeTestRuleContext) GetEndFunc() func(msg types.RuleMsg, err error) {
	return nil
}

func (ctx *NodeTestRuleContext) SubmitTack(task func()) {
	go task()
}
