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

package rulego

import (
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"time"
)

// DefaultRuleContext 默认规则引擎消息处理上下文
type DefaultRuleContext struct {
	//id     string
	config types.Config
	//根规则链上下文
	ruleChainCtx *RuleChainCtx
	//上一个节点上下文
	from types.NodeCtx
	//当前节点上下文
	self types.NodeCtx
	//是否是第一个节点
	isFirst bool
	//协程池
	pool types.Pool
	//当前消息整条规则链处理结束回调函数
	onEnd func(msg types.RuleMsg, err error)
}

//NewRuleContext 创建一个默认规则引擎消息处理上下文实例
func NewRuleContext(config types.Config, ruleChainCtx *RuleChainCtx, from types.NodeCtx, self types.NodeCtx, pool types.Pool, onEnd func(msg types.RuleMsg, err error)) *DefaultRuleContext {
	return &DefaultRuleContext{
		config:       config,
		ruleChainCtx: ruleChainCtx,
		from:         from,
		self:         self,
		pool:         pool,
		onEnd:        onEnd,
	}
}

func (ctx *DefaultRuleContext) TellSuccess(msg types.RuleMsg) {
	ctx.tell(msg, nil, types.Success)
}
func (ctx *DefaultRuleContext) TellFailure(msg types.RuleMsg, err error) {
	ctx.tell(msg, err, types.Failure)
}
func (ctx *DefaultRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	ctx.tell(msg, nil, relationTypes...)
}
func (ctx *DefaultRuleContext) TellSelf(msg types.RuleMsg, delayMs int64) {
	time.AfterFunc(time.Millisecond*time.Duration(delayMs), func() {
		ctx.tell(msg, nil, types.Success)
	})
}
func (ctx *DefaultRuleContext) NewMsg(msgType string, metaData types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}
func (ctx *DefaultRuleContext) GetSelfId() string {
	return ctx.self.GetNodeId().Id
}

func (ctx *DefaultRuleContext) Config() types.Config {
	return ctx.config
}

func (ctx *DefaultRuleContext) SetEndFunc(onEndFunc func(msg types.RuleMsg, err error)) types.RuleContext {
	ctx.onEnd = onEndFunc
	return ctx
}

func (ctx *DefaultRuleContext) GetEndFunc() func(msg types.RuleMsg, err error) {
	return ctx.onEnd
}

func (ctx *DefaultRuleContext) SubmitTack(task func()) {
	if ctx.pool != nil {
		if err := ctx.pool.Submit(task); err != nil {
			ctx.config.Logger.Printf("SubmitTack error:%s", err)
		}
	} else {
		go task()
	}
}

// getNextNodes 获取当前节点指定关系的子节点
func (ctx *DefaultRuleContext) getNextNodes(relationType string) ([]types.NodeCtx, bool) {
	if ctx.ruleChainCtx == nil || ctx.self == nil {
		return nil, false
	}
	return ctx.ruleChainCtx.GetNextNodes(ctx.self.GetNodeId(), relationType)
}

func (ctx *DefaultRuleContext) onDebug(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	if ctx.config.OnDebug != nil {
		ctx.config.OnDebug(flowType, nodeId, msg.Copy(), relationType, err)
	}
}

func (ctx *DefaultRuleContext) tell(msg types.RuleMsg, err error, relationTypes ...string) {
	msgCopy := msg.Copy()
	if ctx.isFirst {
		ctx.SubmitTack(func() {
			ctx.tellNext(msgCopy, ctx.self)
		})
	} else {
		for _, relationType := range relationTypes {
			if ctx.self != nil && ctx.self.IsDebugMode() {
				//记录调试信息
				ctx.SubmitTack(func() {
					ctx.onDebug(types.Out, ctx.GetSelfId(), msgCopy, relationType, err)
				})
			}

			if nodes, ok := ctx.getNextNodes(relationType); ok {
				for _, item := range nodes {
					tmp := item
					ctx.SubmitTack(func() {
						ctx.tellNext(msg.Copy(), tmp)
					})
				}
			} else {
				ctx.doOnEnd(msgCopy, err)
			}
		}
	}

}

func (ctx *DefaultRuleContext) tellNext(msg types.RuleMsg, nextNode types.NodeCtx) {
	nextCtx := NewRuleContext(ctx.config, ctx.ruleChainCtx, ctx.self, nextNode, ctx.pool, ctx.onEnd)
	defer func() {
		//捕捉异常
		if e := recover(); e != nil {
			if nextCtx.self != nil && nextCtx.self.IsDebugMode() {
				//记录异常信息
				ctx.onDebug(types.In, nextCtx.GetSelfId(), msg, "", fmt.Errorf("%v", e))
			}
		}
	}()
	if nextCtx.self != nil && nextCtx.self.IsDebugMode() {
		//记录调试信息
		ctx.onDebug(types.In, nextCtx.GetSelfId(), msg, "", nil)
	}
	if err := nextNode.OnMsg(nextCtx, msg); err != nil {
		ctx.config.Logger.Printf("tellNext error.node type:%s error: %s", nextCtx.self.Type(), err)
	}
}

//规则链执行完成回调函数
func (ctx *DefaultRuleContext) doOnEnd(msg types.RuleMsg, err error) {
	//全局回调
	//通过`Config.OnEnd`设置
	if ctx.config.OnEnd != nil {
		ctx.SubmitTack(func() {
			ctx.config.OnEnd(msg, err)
		})
	}
	//单条消息的context回调
	//通过OnMsgWithEndFunc(msg, endFunc)设置
	if ctx.onEnd != nil {
		ctx.SubmitTack(func() {
			ctx.onEnd(msg, err)
		})
	}
}

// RuleEngine 规则引擎
//每个规则引擎实例只有一个根规则链，如果没设置规则链则无法处理数据
type RuleEngine struct {
	//规则引擎实例标识
	Id string
	//配置
	Config types.Config
	//根规则链
	rootRuleChainCtx *RuleChainCtx
	//子规则链
	subRuleChains map[string][]byte
}

// RuleEngineOption is a function type that modifies the RuleEngine.
type RuleEngineOption func(*RuleEngine) error

func newRuleEngine(id string, def []byte, opts ...RuleEngineOption) (*RuleEngine, error) {
	if len(def) == 0 {
		return nil, errors.New("def can not nil")
	}
	// Create a new RuleEngine with the Id
	ruleEngine := &RuleEngine{
		Id:     id,
		Config: NewConfig(),
	}
	err := ruleEngine.ReloadSelf(def, opts...)
	if err == nil && ruleEngine.rootRuleChainCtx != nil {
		ruleEngine.rootRuleChainCtx.Id = types.RuleNodeId{Id: id, Type: types.CHAIN}
	}
	return ruleEngine, err
}

// ReloadSelf 重新加载规则链
func (e *RuleEngine) ReloadSelf(def []byte, opts ...RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	for _, opt := range opts {
		_ = opt(e)
	}
	//初始化
	if ctx, err := e.Config.Parser.DecodeRuleChain(e.Config, def); err == nil {
		if e.Initialized() {
			e.Stop()
		}
		if e.rootRuleChainCtx != nil {
			ctx.(*RuleChainCtx).Id = e.rootRuleChainCtx.Id
		}
		e.rootRuleChainCtx = ctx.(*RuleChainCtx)
		//初始化子规则链
		for key, value := range e.subRuleChains {
			err := e.ReloadChild(types.EmptyRuleNodeId, types.RuleNodeId{Id: key, Type: types.CHAIN}, value)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		return err
	}
}

// ReloadChild 更新节点,包括根规则链下子节点、子规则链、子规则链下的子节点
//子规则链不存则添加否则更新，子节点不存在更新不成功
//如果chainId和ruleNodeId为空更新根规则链
//chainId 子规则链，如果空，则表示更新根规则链
//ruleNodeId 要更新的子节点或者子规则链
//dsl 子节点/子规则链配置
func (e *RuleEngine) ReloadChild(chainId types.RuleNodeId, ruleNodeId types.RuleNodeId, dsl []byte) error {
	if e.rootRuleChainCtx == nil {
		return errors.New("ReloadNode error.RuleEngine not initialized")
	} else if chainId.Id == "" && ruleNodeId.Id == "" {
		//更新根规则链
		return e.ReloadSelf(dsl)
	} else if chainId.Id == "" && ruleNodeId.Id != "" {
		//更新根规则链子节点
		return e.rootRuleChainCtx.ReloadChild(ruleNodeId, dsl)
	} else if chainId.Id != "" && ruleNodeId.Id != "" {
		//更新指定子规则链节点
		if chainNode, ok := e.rootRuleChainCtx.GetNodeById(chainId); ok {
			return chainNode.ReloadChild(ruleNodeId, dsl)
		}
	}
	return errors.New("ReloadNode error.not found this node")
}

func (e *RuleEngine) DSL() []byte {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.DSL()
	} else {
		return nil
	}
}

func (e *RuleEngine) NodeDSL(chainId types.RuleNodeId, childNodeId types.RuleNodeId) []byte {
	if e.rootRuleChainCtx != nil {
		if chainId.Id == "" {
			if node, ok := e.rootRuleChainCtx.GetNodeById(childNodeId); ok {
				return node.DSL()
			}
		} else {
			if node, ok := e.rootRuleChainCtx.GetNodeById(chainId); ok {
				if childNode, ok := node.GetNodeById(childNodeId); ok {
					return childNode.DSL()
				}
			}
		}
	}
	return nil
}

func (e *RuleEngine) Initialized() bool {
	return e.rootRuleChainCtx != nil
}

//RootRuleChainCtx 获取根规则链
func (e *RuleEngine) RootRuleChainCtx() *RuleChainCtx {
	return e.rootRuleChainCtx
}

func (e *RuleEngine) Stop() {
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.Destroy()
		e.rootRuleChainCtx = nil
	}
}

// OnMsg 把消息交给规则引擎处理，异步执行
//根据规则链节点配置和连接关系处理消息
func (e *RuleEngine) OnMsg(msg types.RuleMsg) {
	e.onMsg(msg, nil)
}

// OnMsgWithEndFunc 把消息交给规则引擎处理，异步执行
//并注册一个规则链执行结束回调函数
//如果规则链有多个结束点，回调函数则会执行多次
func (e *RuleEngine) OnMsgWithEndFunc(msg types.RuleMsg, endFunc func(msg types.RuleMsg, err error)) {
	e.onMsg(msg, endFunc)
}

func (e *RuleEngine) onMsg(msg types.RuleMsg, endFunc func(msg types.RuleMsg, err error)) {
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.rootRuleContext.SetEndFunc(endFunc).TellNext(msg)
	} else {
		//沒有定义根则链或者没初始化
		e.Config.Logger.Printf("onMsg error.RuleEngine not initialized")
	}
}

// NewConfig creates a new Config and applies the options.
func NewConfig(opts ...types.Option) types.Config {
	c := types.NewConfig(opts...)
	if c.Parser == nil {
		c.Parser = &JsonParser{}
	}
	if c.ComponentsRegistry == nil {
		c.ComponentsRegistry = Registry
	}
	return c
}

// WithConfig is an option that sets the Config of the RuleEngine.
func WithConfig(config types.Config) RuleEngineOption {
	return func(re *RuleEngine) error {
		re.Config = config
		return nil
	}
}

//WithAddSubChain 添加子规则链选项
func WithAddSubChain(subChainId string, subChain []byte) RuleEngineOption {
	return func(re *RuleEngine) error {
		if re.subRuleChains == nil {
			re.subRuleChains = make(map[string][]byte)
		}
		re.subRuleChains[subChainId] = subChain
		return nil
	}
}
