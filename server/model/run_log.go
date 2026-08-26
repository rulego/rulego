package model

import "encoding/json"

// Event 一次规则链运行的记录（运行日志表的一行）。
//
// 字段分三组：
//   - 运行元数据：Id/ChainId/ChainName/TriggerSource/Level
//   - 结果：StartTs/EndTs/DurationMs/Success/ErrorMsg
//   - 内容：MsgType/MsgData（链出口消息的摘要，所有级别都有）；
//     Logs（detail 级逐节点日志，含每个节点的入口/出口消息，summary 级为空）
type Event struct {
	Id            string          `json:"id"`
	ChainId       string          `json:"chainId"`
	ChainName     string          `json:"chainName"`
	TriggerSource string          `json:"triggerSource,omitempty"` // manual/http/chat/automation 或 endpoint/<type>（如 endpoint/schedule）
	Level         string          `json:"level"`                   // summary 或 detail，off 不落库
	StartTs       int64           `json:"startTs"`
	EndTs         int64           `json:"endTs"`
	DurationMs    int64           `json:"durationMs"`
	Success       bool            `json:"success"`
	ErrorMsg      string          `json:"errorMsg,omitempty"`
	MsgType       string          `json:"msgType,omitempty"`
	MsgData       string          `json:"msgData,omitempty"`
	Logs          json.RawMessage `json:"logs,omitempty"`
}
