package endpoint

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerLogRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /logs/runs - 获取运行日志
	ep.GET(endpoint.NewRouter().From(base+"/logs/runs").Process(s.authWithPermission("log", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		runLogSvc, ok := getService[services.RunLogService](s, exchange, services.KeyRunLogService)
		if !ok {
			return false
		}
		username := metadataUsername(exchange)
		chainId := metadataValue(exchange, constants.KeyChainId)
		logId := strings.TrimSpace(exchange.In.GetParam("id"))

		if logId != "" {
			event, err := runLogSvc.Get(username, logId)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				return false
			}
			writeJSON(exchange, event)
			return true
		}

		page := intParam(exchange.In.GetMsg(), constants.KeyPage, 1)
		size := intParam(exchange.In.GetMsg(), constants.KeySize, 20)

		var startTime, endTime time.Time
		if st := strings.TrimSpace(exchange.In.GetParam("startTime")); st != "" {
			if ms, err := strconv.ParseInt(st, 10, 64); err == nil && ms > 0 {
				startTime = time.UnixMilli(ms)
			}
		}
		if et := strings.TrimSpace(exchange.In.GetParam("endTime")); et != "" {
			if ms, err := strconv.ParseInt(et, 10, 64); err == nil && ms > 0 {
				endTime = time.UnixMilli(ms)
			}
		}

		events, total, err := runLogSvc.List(username, chainId, startTime, endTime, size, page)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeListResult(exchange, events, total, page, size)
		return true
	}).End())

	// DELETE /logs/runs - 删除运行日志
	ep.DELETE(endpoint.NewRouter().From(base+"/logs/runs").Process(s.authWithPermission("log", "delete")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		runLogSvc, ok := getService[services.RunLogService](s, exchange, services.KeyRunLogService)
		if !ok {
			return false
		}
		username := metadataUsername(exchange)
		chainId := strings.TrimSpace(exchange.In.GetParam("chainId"))
		logId := strings.TrimSpace(exchange.In.GetParam("id"))

		var err error
		switch {
		case logId != "":
			err = runLogSvc.Delete(username, logId)
		case chainId != "":
			err = runLogSvc.DeleteByChainId(username, chainId)
		default:
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte("chainId or id is required"))
			return false
		}
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeNoContent(exchange)
		return true
	}).End())

	// GET /logs/debug - 获取节点调试日志（从内存存储读取）
	ep.GET(endpoint.NewRouter().From(base+"/logs/debug").Process(s.authWithPermission("log", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyChainId)
		nodeId := msg.Metadata.GetValue(constants.KeyNodeId)
		page := intParam(msg, constants.KeyPage, 1)
		size := intParam(msg, constants.KeySize, 20)

		result := runlog.DefaultDebugDataStore.GetPage(chainId, nodeId, page, size)
		writeJSON(exchange, result)
		return true
	}).End())
}
