package model

import "encoding/json"

// Event 一次规则链运行的记录（运行日志表的一行）。
type Event struct {
	Id            string          `json:"id"`
	ChainId       string          `json:"chainId"`
	ChainName     string          `json:"chainName"`
	StartTs       int64           `json:"startTs"`
	EndTs         int64           `json:"endTs"`
	Success       bool            `json:"success"`
	ErrorMsg      string          `json:"errorMsg,omitempty"`
	Logs          json.RawMessage `json:"logs,omitempty"`
	TriggerSource string          `json:"triggerSource,omitempty"` // manual/http/chat/timer/mqtt/ws
	Level         string          `json:"level"`                   // summary 或 detail，off 不落库
}
