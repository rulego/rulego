package model

// RuleChainMeta 规则链元数据
type RuleChainMeta struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	RootRuleId  string `json:"rootRuleId"`
	Disabled    bool   `json:"disabled"`
	CreateTime  int64  `json:"createTime"`
	UpdateTime  int64  `json:"updateTime"`
}

// Variable 变量定义
type Variable struct {
	Title       string `json:"title"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Owner       string `json:"owner"`
}
