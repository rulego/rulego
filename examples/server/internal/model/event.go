package model

type Event struct {
	Type string `json:"Type"`
}

// DebugData 调试数据
// OnDebug 回调函数提供的数据
type DebugData struct {
	//debug数据发生时间
	Ts int64 `json:"ts"`
	//节点ID
	NodeId string `json:"nodeId"`
	//流向OUT/IN
	FlowType string `json:"flowType"`
	//消息类型
	MsgType string `json:"msgType"`
	//消息ID
	MsgId string `json:"msgId"`
	//消息内容
	Data string `json:"data"`
	//消息元数据
	Metadata string `json:"metadata"`
	//Err 错误
	Err string `json:"err"`
	//关系
	RelationType string `json:"relationType"`
}

// DebugDataPage 分页返回数据
type DebugDataPage struct {
	//每页多少条，默认读取所有
	Size int `json:"PageSize"`
	//当前第几页，默认读取所有
	Current int `json:"current"`
	//总数
	Total int `json:"total"`
	//记录
	Items []DebugData `json:"items"`
}
