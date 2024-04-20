package model

// Variable 变量
type Variable struct {
	// 标题
	Title string `json:"title"`
	// 名称，唯一的
	Name string `json:"name"`
	// 内容
	Value string `json:"value"`
	// 描述
	Description string `json:"description"`
	// 类型 0:变量；1:机密的；2:秘钥
	Type int `json:"type"`
	// 所属用户
	Owner string `json:"owner"`
}
