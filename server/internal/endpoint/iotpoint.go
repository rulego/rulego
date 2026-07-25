package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/iotpoint"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerIoTPointRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /iot/point-templates - 模板列表（?protocol=&category= 筛选）
	ep.GET(endpoint.NewRouter().From(base+"/iot/point-templates").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := getService[*iotpoint.Module](s, exchange, services.KeyIoTPointService)
		if !ok {
			return false
		}
		msg := exchange.In.GetMsg()
		protocol := strings.TrimSpace(msg.Metadata.GetValue("protocol"))
		category := strings.TrimSpace(msg.Metadata.GetValue("category"))
		writeJSON(exchange, svc.List(protocol, category))
		return true
	}).End())

	// GET /iot/point-templates/:id - 模板详情
	ep.GET(endpoint.NewRouter().From(base+"/iot/point-templates/:id").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := getService[*iotpoint.Module](s, exchange, services.KeyIoTPointService)
		if !ok {
			return false
		}
		id := strings.TrimSpace(metadataValue(exchange, constants.KeyId))
		tpl, err := svc.Get(id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				exchange.Out.SetBody([]byte(`{"error":"template not found"}`))
				return false
			}
			writeBadRequest(exchange, err)
			return false
		}
		writeJSON(exchange, tpl)
		return true
	}).End())

	// POST /iot/point-templates - 创建模板
	ep.POST(endpoint.NewRouter().From(base+"/iot/point-templates").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := getService[*iotpoint.Module](s, exchange, services.KeyIoTPointService)
		if !ok {
			return false
		}
		var tpl iotpoint.PointTemplate
		if err := json.Unmarshal(exchange.In.Body(), &tpl); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := svc.Create(tpl); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		writeJSON(exchange, tpl)
		return true
	}).End())

	// PUT /iot/point-templates/:id - 更新模板
	ep.PUT(endpoint.NewRouter().From(base+"/iot/point-templates/:id").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := getService[*iotpoint.Module](s, exchange, services.KeyIoTPointService)
		if !ok {
			return false
		}
		id := strings.TrimSpace(metadataValue(exchange, constants.KeyId))
		var tpl iotpoint.PointTemplate
		if err := json.Unmarshal(exchange.In.Body(), &tpl); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := svc.Update(id, tpl); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				exchange.Out.SetBody([]byte(`{"error":"template not found"}`))
				return false
			}
			writeBadRequest(exchange, err)
			return false
		}
		writeJSON(exchange, tpl)
		return true
	}).End())

	// DELETE /iot/point-templates/:id - 删除模板（内置模板不可删）
	ep.DELETE(endpoint.NewRouter().From(base+"/iot/point-templates/:id").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := getService[*iotpoint.Module](s, exchange, services.KeyIoTPointService)
		if !ok {
			return false
		}
		id := strings.TrimSpace(metadataValue(exchange, constants.KeyId))
		if err := svc.Delete(id); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				exchange.Out.SetBody([]byte(`{"error":"template not found"}`))
				return false
			}
			writeBadRequest(exchange, err)
			return false
		}
		writeNoContent(exchange)
		return true
	}).End())
}
