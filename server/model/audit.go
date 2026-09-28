package model

import "time"

// 审计事件结果值
const (
	AuditResultOK     = "ok"     // 动作成功
	AuditResultDenied = "denied" // 认证/授权拒绝（含登录失败）
	AuditResultError  = "error"  // 动作执行出错
)

// AuditEvent 管理面操作审计事件：谁在什么时候对什么做了什么、结果如何。
// 数据面（链内消息流转）不在此记录，那是运行日志的职责。
// Detail 只放摘要，永不携带密码、API Key、JWT 等敏感值。
type AuditEvent struct {
	Ts        int64  `json:"ts"`                  // 毫秒时间戳，零值由服务端补当前时间
	Actor     string `json:"actor"`               // 操作者用户名
	ActorType string `json:"actorType,omitempty"` // user/apikey/anonymous
	IP        string `json:"ip,omitempty"`
	UA        string `json:"ua,omitempty"`     // 客户端标识（User-Agent，截断存储）
	Action    string `json:"action"`           // 资源:动作，如 rule:operate
	Op        string `json:"op,omitempty"`     // 动作细分，如 deploy/undeploy/create
	Target    string `json:"target,omitempty"` // 目标资源，如 rule:<chainId>
	Result    string `json:"result"`           // ok/denied/error
	Detail    string `json:"detail,omitempty"`
}

// AuditFilter 审计查询过滤条件
type AuditFilter struct {
	StartTime time.Time // 含
	EndTime   time.Time // 含
	Actor     string    // 精确匹配
	Action    string    // 精确匹配
	Result    string    // 精确匹配
	Target    string    // 前缀匹配
	Size      int
	Page      int
}
