package model

import "encoding/json"

// Event 规则链运行事件
type Event struct {
	Id        string          `json:"id"`
	ChainId   string          `json:"chainId"`
	ChainName string          `json:"chainName"`
	StartTs   int64           `json:"startTs"`
	EndTs     int64           `json:"endTs"`
	Success   bool            `json:"success"`
	ErrorMsg  string          `json:"errorMsg,omitempty"`
	Logs      json.RawMessage `json:"logs,omitempty"`
}

