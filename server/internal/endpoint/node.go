package endpoint

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerNodeRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /shared-nodes - 获取共享节点列表
	ep.GET(endpoint.NewRouter().From(base+"/shared-nodes").Process(s.authWithPermission("component", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		msg := exchange.In.GetMsg()
		list, total, err := nodeSvc.ListNodePool(
			metadataUsername(exchange),
			intParam(msg, constants.KeyPage, 1),
			intParam(msg, constants.KeySize, 20),
			strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords)),
			strings.TrimSpace(msg.Metadata.GetValue(constants.KeyType)),
		)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeListResult(exchange, list, total, intParam(msg, constants.KeyPage, 1), intParam(msg, constants.KeySize, 20))
		return true
	}).End())

	// POST /shared-nodes/:id/:type - 添加/更新共享节点
	ep.POST(endpoint.NewRouter().From(base+"/shared-nodes/:id/:type").Process(s.authWithPermission("component", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		username := metadataUsername(exchange)
		if metadataValue(exchange, constants.KeyType) == "endpoint" {
			var endpointDef types.EndpointDsl
			if err := json.Unmarshal(exchange.In.Body(), &endpointDef); err != nil {
				writeBadRequest(exchange, err)
				return false
			}
			if err := nodeSvc.SaveNodePoolEndpoint(username, endpointDef); err != nil {
				writeBadRequest(exchange, err)
			}
		} else {
			var node types.RuleNode
			if err := json.Unmarshal(exchange.In.Body(), &node); err != nil {
				writeBadRequest(exchange, err)
				return false
			}
			if err := nodeSvc.SaveNodePoolNode(username, node); err != nil {
				writeBadRequest(exchange, err)
			}
		}
		return true
	}).End())

	// GET /shared-nodes/:id/:type - 获取共享节点
	ep.GET(endpoint.NewRouter().From(base+"/shared-nodes/:id/:type").Process(s.authWithPermission("component", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		node, err := nodeSvc.GetNodePool(metadataUsername(exchange), metadataValue(exchange, constants.KeyId), metadataValue(exchange, constants.KeyType))
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		if node == nil {
			exchange.Out.SetStatusCode(http.StatusNotFound)
			return false
		}
		writeJSON(exchange, node)
		return true
	}).End())

	// DELETE /shared-nodes/:id/:type - 删除共享节点
	ep.DELETE(endpoint.NewRouter().From(base+"/shared-nodes/:id/:type").Process(s.authWithPermission("component", "delete")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		if err := nodeSvc.DeleteNodePool(metadataUsername(exchange), metadataValue(exchange, constants.KeyId), metadataValue(exchange, constants.KeyType)); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		writeNoContent(exchange)
		return true
	}).End())

	// GET /dynamic-components - 获取动态组件列表
	ep.GET(endpoint.NewRouter().From(base+"/dynamic-components").Process(s.authWithPermission("component", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		msg := exchange.In.GetMsg()
		list, total, err := nodeSvc.ListComponents(
			metadataUsername(exchange),
			strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords)),
			intParam(msg, constants.KeySize, 20),
			intParam(msg, constants.KeyPage, 1),
		)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeListResult(exchange, list, total, intParam(msg, constants.KeyPage, 1), intParam(msg, constants.KeySize, 20))
		return true
	}).End())

	// GET /dynamic-components/:id - 获取动态组件 DSL
	ep.GET(endpoint.NewRouter().From(base+"/dynamic-components/:id").Process(s.authWithPermission("component", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		def, err := nodeSvc.GetComponent(metadataUsername(exchange), metadataValue(exchange, constants.KeyId))
		if err != nil {
			exchange.Out.SetStatusCode(404)
			return false
		}
		exchange.Out.SetBody(def)
		return true
	}).End())

	// POST /dynamic-components/:id - 安装/升级动态组件
	ep.POST(endpoint.NewRouter().From(base+"/dynamic-components/:id").Process(s.authWithPermission("component", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		if err := nodeSvc.UpgradeComponent(metadataUsername(exchange), metadataValue(exchange, constants.KeyId), exchange.In.Body()); err != nil {
			writeBadRequest(exchange, err)
		}
		return true
	}).End())

	// DELETE /dynamic-components/:id - 卸载动态组件
	ep.DELETE(endpoint.NewRouter().From(base+"/dynamic-components/:id").Process(s.authWithPermission("component", "delete")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
		if !ok {
			return false
		}
		if err := nodeSvc.UninstallComponent(metadataUsername(exchange), metadataValue(exchange, constants.KeyId)); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		writeNoContent(exchange)
		return true
	}).End())
}
