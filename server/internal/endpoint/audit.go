package endpoint

import (
	"strconv"
	"strings"
	"time"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerAuditRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /audit/logs - 管理面操作审计查询（仅 admin）
	ep.GET(endpoint.NewRouter().From(base + "/audit/logs").Process(s.authWithPermission(constants.ResourceAudit, "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		auditSvc, ok := getService[services.AuditService](s, exchange, services.KeyAuditService)
		if !ok {
			return false
		}
		page := intParam(exchange.In.GetMsg(), constants.KeyPage, 1)
		size := intParam(exchange.In.GetMsg(), constants.KeySize, 20)

		var filter model.AuditFilter
		filter.Page, filter.Size = page, size
		if st := strings.TrimSpace(exchange.In.GetParam("startTime")); st != "" {
			if ms, err := strconv.ParseInt(st, 10, 64); err == nil && ms > 0 {
				filter.StartTime = time.UnixMilli(ms)
			}
		}
		if et := strings.TrimSpace(exchange.In.GetParam("endTime")); et != "" {
			if ms, err := strconv.ParseInt(et, 10, 64); err == nil && ms > 0 {
				filter.EndTime = time.UnixMilli(ms)
			}
		}
		filter.Actor = strings.TrimSpace(exchange.In.GetParam("actor"))
		filter.Action = strings.TrimSpace(exchange.In.GetParam("action"))
		filter.Result = strings.TrimSpace(exchange.In.GetParam("result"))
		filter.Target = strings.TrimSpace(exchange.In.GetParam("target"))

		events, total, err := auditSvc.List(filter)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeListResult(exchange, events, int(total), page, size)
		return true
	}).End())
}

// auditRecord 记录管理面操作审计。IP 与客户端标识在此统一填充，调用方只填业务字段；
// 审计关闭或宿主未装 audit 模块时静默跳过。
func (s *Server) auditRecord(exchange *endpointApi.Exchange, event model.AuditEvent) {
	event.IP = clientIP(exchange)
	if ua := exchange.In.Headers().Get("User-Agent"); ua != "" {
		event.UA = ua
	}
	if svc, err := getServiceRaw[services.AuditService](s, services.KeyAuditService); err == nil {
		svc.Record(event)
	}
}
