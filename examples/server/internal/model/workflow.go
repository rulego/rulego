package model

// Workflow
type Workflow struct {
	// Name
	Name string `json:"name"`
	// Affiliated users
	Owner string `json:"owner"`
	// Description
	Description string `json:"description"`
	// Creation date
	CreateTime int64 `json:"createTime"`
	// Update time
	UpdateTime int64 `json:"updateTime"`
	// Expand information
	AdditionalInfo map[string]interface{} `json:"additionalInfo"`
	//Rule chain definition
	RuleChain string `json:"rulechain"`
}
