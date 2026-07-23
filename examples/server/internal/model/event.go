package model

type Event struct {
	Type string `json:"Type"`
}

// DebugData debugging data
// The data provided by the OnDebug callback function
type DebugData struct {
	//Debug data occurrence time
	Ts int64 `json:"ts"`
	//Node ID
	NodeId string `json:"nodeId"`
	//Flow to OUT/IN
	FlowType string `json:"flowType"`
	//Message type
	MsgType string `json:"msgType"`
	//Message ID
	MsgId string `json:"msgId"`
	//News content
	Data string `json:"data"`
	//Message metadata
	Metadata string `json:"metadata"`
	//Err is incorrect
	Err string `json:"err"`
	//Relationships
	RelationType string `json:"relationType"`
}

// DebugDataPage paginates to return data
type DebugDataPage struct {
	//How many entries per page is read by default
	Size int `json:"PageSize"`
	//Current page number, read all by default
	Current int `json:"current"`
	//Total
	Total int `json:"total"`
	//Record
	Items []DebugData `json:"items"`
}
