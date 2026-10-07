package model

import "encoding/json"

// RuleVersion 规则链历史版本快照（列表接口不带 Dsl，详情接口才带）
type RuleVersion struct {
	Id        string          `json:"id"`
	ChainId   string          `json:"chainId"`
	ChainName string          `json:"chainName,omitempty"`
	// Seq 链内单调递增的版本号（v1、v2…），保留裁剪不回收重排
	Seq       int             `json:"seq"`
	Ts        int64           `json:"ts"`
	Source    string          `json:"source"` // save|rollback
	NodeCount int             `json:"nodeCount"`
	DslSize   int             `json:"dslSize"`
	Dsl       json.RawMessage `json:"dsl,omitempty"`
}
