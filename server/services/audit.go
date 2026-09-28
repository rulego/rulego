package services

import (
	"github.com/rulego/rulego/server/model"
)

// AuditService 管理面操作审计服务。Record 非阻塞异步落盘，失败只影响审计；
// List 同步查询，供审计查询接口使用。
type AuditService interface {
	Record(event model.AuditEvent)
	List(filter model.AuditFilter) ([]model.AuditEvent, int64, error)
}
