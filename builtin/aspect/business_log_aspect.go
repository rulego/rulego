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

package aspect

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/str"
)

// Metadata keys carried by every business log event message.
// 业务日志事件消息携带的 metadata 键。
const (
	// MetaLogScope event origin: "node" or "chain".
	MetaLogScope = "scope"
	// MetaLogChainId id of the chain that emitted the event.
	MetaLogChainId = "chainId"
	// MetaLogNodeId node id, empty for chain-scope events.
	MetaLogNodeId = "nodeId"
	// MetaLogNodeName node name, empty for chain-scope events.
	MetaLogNodeName = "nodeName"
	// MetaLogMsgId id of the business message that triggered the event.
	MetaLogMsgId = "msgId"
	// MetaLogPhase event phase: "in" (enter node) / "out" (node output) / "end" (chain end point).
	MetaLogPhase = "phase"
	// MetaLogRelation output relation of an "out"/"end" event, e.g. Success/Failure.
	MetaLogRelation = "relationType"
	// MetaLogError error text on failure.
	MetaLogError = "error"
	// MetaLogTs event timestamp in nanoseconds.
	MetaLogTs = "ts"
	// MetaLogEventFlag loop guard: messages carrying this flag are log events themselves
	// and never produce further events.
	MetaLogEventFlag = "_logEvent"
	// MetaLogRenderError set when template rendering fails; event Data falls back to the raw template.
	MetaLogRenderError = "renderError"
)

const (
	scopeNode  = "node"
	scopeChain = "chain"

	phaseIn  = "in"
	phaseOut = "out"
	phaseEnd = "end"
)

type nodeLog struct {
	before, after       el.Template
	beforeRaw, afterRaw string
}

// logState 是不可变快照：init 钩子克隆后整体发布，运行期只做一次原子加载。
// 引擎热更复用切面实例、在原实例上重建快照，不可变发布保证在途消息不与重建竞态。
type logState struct {
	handlerId string
	chainEnd  bool
	nodes     map[string]*nodeLog
	// 事件文案里的节点名在 init 期预存，运行期不从节点定义回查
	names map[string]string
}

func (s *logState) clone() *logState {
	ns := &logState{
		handlerId: s.handlerId, chainEnd: s.chainEnd,
		nodes: make(map[string]*nodeLog, len(s.nodes)+1),
		names: make(map[string]string, len(s.names)+1),
	}
	for k, v := range s.nodes {
		ns.nodes[k] = v
	}
	for k, v := range s.names {
		ns.names[k] = v
	}
	return ns
}

// BusinessLog emits business log events declared by nodes (RuleNode.LogConfig)
// and the chain-level end-of-execution switch (ruleChain.Configuration[LogEvents]).
// Events are delivered asynchronously through two channels and never affect the
// business flow: the OnDebug Log channel (visible in run logs / WebSocket) and
// the optional handler chain (ruleChain.Configuration[LogHandler]).
//
// BusinessLog 产生节点声明（RuleNode.LogConfig）与链级结束上报开关
// （ruleChain.Configuration[LogEvents]）定义的业务日志事件，经 OnDebug Log 通道
// 与可选处理链（ruleChain.Configuration[LogHandler]）异步双出口投递，不影响业务流程。
type BusinessLog struct {
	state atomic.Value
}

var (
	_ types.BeforeAspect            = (*BusinessLog)(nil)
	_ types.AfterAspect             = (*BusinessLog)(nil)
	_ types.EndAspect               = (*BusinessLog)(nil)
	_ types.OnChainBeforeInitAspect = (*BusinessLog)(nil)
	_ types.OnNodeBeforeInitAspect  = (*BusinessLog)(nil)
)

// Order returns 950, after the Debug aspect (900).
func (a *BusinessLog) Order() int { return 950 }

// New creates a per-engine instance. Engine reload reuses instances, so all
// instance state lives in the atomic snapshot rebuilt by the init hooks.
func (a *BusinessLog) New() types.Aspect {
	return &BusinessLog{}
}

func (a *BusinessLog) Type() string { return "businessLog" }

// loadState lazily seeds an empty snapshot; init hooks publish the real one.
func (a *BusinessLog) loadState() *logState {
	if st := a.snap(); st != nil {
		return st
	}
	a.state.Store(&logState{nodes: map[string]*nodeLog{}, names: map[string]string{}})
	return a.snap()
}

// snap returns the current snapshot, nil before the first publish.
func (a *BusinessLog) snap() *logState {
	st, _ := a.state.Load().(*logState)
	return st
}

// OnChainBeforeInit rebuilds the snapshot from chain-level configuration; it runs
// on both first load and reload (both go through InitRuleChainCtx).
func (a *BusinessLog) OnChainBeforeInit(config types.Config, def *types.RuleChain) error {
	st := &logState{nodes: map[string]*nodeLog{}, names: map[string]string{}}
	if def != nil {
		if v, ok := def.RuleChain.Configuration[types.LogHandler]; ok {
			if s, ok2 := v.(string); ok2 {
				st.handlerId = strings.TrimSpace(s)
			}
		}
		if v, ok := def.RuleChain.Configuration[types.LogEvents]; ok {
			st.chainEnd = parseLogEvents(v)
		}
	}
	a.state.Store(st)
	return nil
}

// parseLogEvents accepts both []string (built programmatically) and
// []interface{} (decoded from JSON) forms of the logEvents configuration.
func parseLogEvents(v interface{}) (chainEnd bool) {
	apply := func(s string) {
		if s == types.LogEventChainEnd {
			chainEnd = true
		}
	}
	switch items := v.(type) {
	case []string:
		for _, it := range items {
			apply(it)
		}
	case []interface{}:
		for _, it := range items {
			if s, ok := it.(string); ok {
				apply(s)
			}
		}
	}
	return
}

// OnNodeBeforeInit precompiles node templates into the snapshot. A template that
// fails to compile is skipped with a warning: bad log config must not fail the chain.
func (a *BusinessLog) OnNodeBeforeInit(config types.Config, node *types.RuleNode) error {
	if node == nil {
		return nil
	}
	cur := a.loadState()
	lc := node.LogConfig
	need := lc != nil && (lc.Before != "" || lc.After != "")
	if !need {
		return nil
	}
	ns := cur.clone()
	nl := &nodeLog{}
	if lc.Before != "" {
		nl.before, nl.beforeRaw = compileLogTemplate(lc.Before, config)
	}
	if lc.After != "" {
		nl.after, nl.afterRaw = compileLogTemplate(lc.After, config)
	}
	if nl.before != nil || nl.after != nil {
		ns.nodes[node.Id] = nl
		ns.names[node.Id] = node.Name
	}
	a.state.Store(ns)
	return nil
}

func compileLogTemplate(tpl string, config types.Config) (el.Template, string) {
	t, err := el.NewTemplate(tpl)
	if err != nil {
		if config.Logger != nil {
			config.Logger.Warnf("business log template compile failed, skipped: %s", err.Error())
		}
		return nil, ""
	}
	return t, tpl
}

func (a *BusinessLog) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	st := a.snap()
	if st == nil {
		return false
	}
	// engine 的 onEnd 对切面统一走 PointCut 门控，
	// 结束事件挂在 End 钩子上，必须在 PointCut 放行整链
	if st.chainEnd {
		return true
	}
	if ctx == nil || ctx.Self() == nil {
		return false
	}
	_, ok := st.nodes[ctx.Self().GetNodeId().Id]
	return ok
}

func (a *BusinessLog) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	st := a.snap()
	if st == nil || ctx == nil || ctx.Self() == nil {
		return msg
	}
	nodeId := ctx.Self().GetNodeId().Id
	nl := st.nodes[nodeId]
	if nl == nil || nl.before == nil || isLogEventMsg(msg) {
		return msg
	}
	content, renderErr := renderLogTemplate(nl.before, nl.beforeRaw, ctx, msg)
	a.emit(ctx, st, msg, scopeNode, nodeId, st.names[nodeId], phaseIn, content, renderErr, "", nil)
	return msg
}

func (a *BusinessLog) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	st := a.snap()
	if st == nil || ctx == nil || ctx.Self() == nil {
		return msg
	}
	nodeId := ctx.Self().GetNodeId().Id
	if nl := st.nodes[nodeId]; nl != nil && nl.after != nil {
		if isLogEventMsg(msg) {
			return msg
		}
		content, renderErr := renderLogTemplate(nl.after, nl.afterRaw, ctx, msg)
		a.emit(ctx, st, msg, scopeNode, nodeId, st.names[nodeId], phaseOut, content, renderErr, relationType, err)
	}
	return msg
}

// End reports one event per terminal branch, carrying the terminal message data;
// relationType and error tell success from failure. Chains with an end node
// trigger only there (engine gating in DoOnEnd), others at every leaf.
// End 在每个终点分支各上报一条结束事件，Data 为触发点消息数据，
// relationType/error 区分成败。链内有结束节点时仅结束节点触发（引擎 DoOnEnd 门控），
// 否则每个无下游的分支终点各触发一次。
func (a *BusinessLog) End(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	st := a.snap()
	if st == nil || !st.chainEnd || isLogEventMsg(msg) || ctx == nil {
		return msg
	}
	a.emit(ctx, st, msg, scopeChain, "", "", phaseEnd, msg.GetData(), "", relationType, err)
	return msg
}

func isLogEventMsg(msg types.RuleMsg) bool {
	return msg.Metadata != nil && msg.Metadata.GetValue(MetaLogEventFlag) != ""
}

// renderLogTemplate renders on the hot path (message may be mutated downstream),
// falling back to the raw template plus MetaLogRenderError on failure.
// global 段（宿主配置，含全局秘钥明文）的值一律掩码：日志事件不外泄宿主配置，
// 消息自身数据（metadata/data）与用户自管的链级 vars 不受限。
func renderLogTemplate(tpl el.Template, raw string, ctx types.RuleContext, msg types.RuleMsg) (content, renderErr string) {
	env := ctx.GetEnv(msg, true)
	if g, ok := env[types.Global].(map[string]string); ok && len(g) > 0 {
		masked := make(map[string]string, len(g))
		for k := range g {
			masked[k] = "***"
		}
		env[types.Global] = masked
	}
	out, err := tpl.Execute(env)
	if err != nil {
		return raw, err.Error()
	}
	return str.ToString(out), ""
}

// emit constructs the event synchronously (rendering and metadata must observe
// the message at this moment) and dispatches asynchronously. Dispatch failures
// only log a warning and never propagate to the business chain.
func (a *BusinessLog) emit(ctx types.RuleContext, st *logState, srcMsg types.RuleMsg,
	scope, nodeId, nodeName, phase, content, renderErr, relationType string, err error) {
	chainId := ""
	if ctx.RuleChain() != nil {
		chainId = ctx.RuleChain().GetNodeId().Id
	}
	md := types.NewMetadata()
	md.PutValue(MetaLogScope, scope)
	md.PutValue(MetaLogChainId, chainId)
	if nodeId != "" {
		md.PutValue(MetaLogNodeId, nodeId)
	}
	if nodeName != "" {
		md.PutValue(MetaLogNodeName, nodeName)
	}
	md.PutValue(MetaLogMsgId, srcMsg.Id)
	md.PutValue(MetaLogPhase, phase)
	if relationType != "" {
		md.PutValue(MetaLogRelation, relationType)
	}
	if err != nil {
		md.PutValue(MetaLogError, err.Error())
	}
	md.PutValue(MetaLogTs, strconv.FormatInt(time.Now().UnixNano(), 10))
	md.PutValue(MetaLogEventFlag, "1")
	if renderErr != "" {
		md.PutValue(MetaLogRenderError, renderErr)
	}
	eventMsg := ctx.NewMsg(types.MsgTypeLog, md, content)

	// 出口①：OnDebug Log 事件不受 debugMode 门控，运行记录/WS 直接可见
	ctx.OnDebug(chainId, types.Log, nodeId, eventMsg, relationType, err)

	// 出口②：处理链独立执行，不用 TellFlow（其失败会回写原链 Failure 分支）
	if st.handlerId == "" {
		return
	}
	chainCtx, ok := ctx.RuleChain().(types.ChainCtx)
	if !ok || chainCtx == nil {
		return
	}
	pool := chainCtx.GetRuleEnginePool()
	if pool == nil {
		return
	}
	config := ctx.Config()
	handlerId := st.handlerId
	ctx.SubmitTask(func() {
		defer func() {
			if r := recover(); r != nil {
				if config.Logger != nil {
					config.Logger.Errorf("business log handler dispatch panic: %v", r)
				}
			}
		}()
		if e, ok := pool.Get(handlerId); ok {
			e.OnMsg(eventMsg)
		} else if config.Logger != nil {
			config.Logger.Warnf("business log handler chain %s not found, event dropped", handlerId)
		}
	})
}
