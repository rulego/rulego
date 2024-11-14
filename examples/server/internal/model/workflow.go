package model

// Workflow 工作流
type Workflow struct {
	// 名称
	Name string `json:"name"`
	// 所属用户
	Owner string `json:"owner"`
	// 描述
	Description string `json:"description"`
	// 创建时间
	CreateTime int64 `json:"createTime"`
	// 更新时间
	UpdateTime int64 `json:"updateTime"`
	// 扩展信息
	AdditionalInfo map[string]interface{} `json:"additionalInfo"`
	//规则链定义
	RuleChain string `json:"rulechain"`
}
