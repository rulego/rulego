package runlogutil

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/internal/constants"
)

// Level 是 server 层自有的运行记录级别枚举。
// 之所以单独定义而非复用 types.RunLogMode：这里需要能表达 Off（不记录），
// 并附带 Int 语义便于比较；而 types.RunLogMode 是字符串别名，用于透传给引擎。
type Level int

const (
	LevelOff     Level = 0 // 不记录
	LevelSummary Level = 1 // 摘要：不收集逐节点日志，近零开销
	LevelDetail  Level = 2 // 完整逐节点日志
)

// ParseLevel 解析级别字符串。空串或未知值视为 Off。
// 字符串值域与引擎层 types.RunLogMode* 对齐（off/summary/detail），
// 另兼容数字 "1"/"2"（历史配置与 UI 入参）。
func ParseLevel(s string) Level {
	switch types.RunLogMode(s) {
	case types.RunLogModeSummary, "1":
		return LevelSummary
	case types.RunLogModeDetail, "2":
		return LevelDetail
	default:
		return LevelOff
	}
}

// String 返回级别对应的引擎字符串（与 types.RunLogMode* 同值域）。
func (l Level) String() string {
	switch l {
	case LevelSummary:
		return string(types.RunLogModeSummary)
	case LevelDetail:
		return string(types.RunLogModeDetail)
	default:
		return string(types.RunLogModeOff)
	}
}

// ChainRunLogMode 读规则链定义的 additionalInfo.runLogMode（链级覆盖值），无则为空串。
func ChainRunLogMode(ctx types.RuleContext) string {
	if ctx == nil {
		return ""
	}
	if chainCtx, ok := ctx.RuleChain().(types.ChainCtx); ok {
		if def := chainCtx.Definition(); def != nil {
			if v, ok := def.RuleChain.GetAdditionalInfo(types.AdditionalInfoKeyRunLogMode); ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

// ResolveLevel 解析最终生效级别：链级配置非空则覆盖全局，否则回退全局。
func ResolveLevel(globalCfg string, ctx types.RuleContext) Level {
	chainLevel := ChainRunLogMode(ctx)
	if chainLevel != "" {
		return ParseLevel(chainLevel)
	}
	return ParseLevel(globalCfg)
}

// UsernameFromCtx 从规则链定义读 additionalInfo.username，用于在引擎回调里归属运行记录。
func UsernameFromCtx(ctx types.RuleContext) string {
	if ctx == nil {
		return ""
	}
	if chainCtx, ok := ctx.RuleChain().(types.ChainCtx); ok {
		if def := chainCtx.Definition(); def != nil {
			if v, ok := def.RuleChain.GetAdditionalInfo(constants.KeyUsername); ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}
