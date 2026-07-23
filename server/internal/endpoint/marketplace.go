package endpoint

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/marketplace"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerMarketplaceRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /marketplace/components - Get marketplace components
	ep.GET(endpoint.NewRouter().From(base + "/marketplace/components").Process(s.authWithPermission("marketplace", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		marketplaceSvc, ok := getService[*marketplace.Module](s, exchange, services.KeyMarketplaceService)
		if !ok {
			return false
		}
		// Inject NodeService for local component acquisition
		if nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService); ok {
			marketplaceSvc.SetNodeService(nodeSvc)
		}
		msg := exchange.In.GetMsg()
		keywords := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords))
		page := intParam(msg, constants.KeyPage, 1)
		size := intParam(msg, constants.KeySize, 20)

		result, err := marketplaceSvc.GetComponents(keywords, page, size)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		if result == nil {
			writeListResult(exchange, []interface{}{}, 0, page, size)
			return true
		}

		// checkMy: Tags installed and upgradeable components
		checkMyStr := msg.Metadata.GetValue("checkMy")
		if b, _ := strconv.ParseBool(checkMyStr); b {
			s.markInstalled(exchange, result.Items)
		}

		writeListResult(exchange, result.Items, result.Total, result.Page, result.Size)
		return true
	}).End())

	// GET /marketplace/chains - Get the marketplace rule chain
	ep.GET(endpoint.NewRouter().From(base + "/marketplace/chains").Process(s.authWithPermission("marketplace", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		marketplaceSvc, ok := getService[*marketplace.Module](s, exchange, services.KeyMarketplaceService)
		if !ok {
			return false
		}
		// Inject NodeService for local component acquisition
		if nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService); ok {
			marketplaceSvc.SetNodeService(nodeSvc)
		}
		msg := exchange.In.GetMsg()
		keywords := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords))
		page := intParam(msg, constants.KeyPage, 1)
		size := intParam(msg, constants.KeySize, 20)
		var root *bool
		if v := msg.Metadata.GetValue("root"); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				root = &b
			}
		}

		result, err := marketplaceSvc.GetChains(root, keywords, page, size)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		if result == nil {
			writeListResult(exchange, []interface{}{}, 0, page, size)
			return true
		}
		writeListResult(exchange, result.Items, result.Total, result.Page, result.Size)
		return true
	}).End())
}

// markInstalled marks the installed/upgraded status on the list of market components
func (s *Server) markInstalled(exchange *endpointApi.Exchange, items []interface{}) {
	nodeSvc, ok := getService[services.NodeService](s, exchange, services.KeyNodeService)
	if !ok {
		return
	}
	username := metadataUsername(exchange)
	forms := nodeSvc.GetComponentForms(username)
	if len(forms) == 0 {
		return
	}
	installedMap := make(map[string]types.ComponentForm, len(forms))
	for _, f := range forms {
		installedMap[f.Type] = f
	}
	for i, item := range items {
		// Convert types.RuleChain structures to map to modify additionalInfo in place
		if _, ok := item.(types.RuleChain); ok {
			data, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}
			items[i] = m
			item = m
		}
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rc, ok := m["ruleChain"].(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := rc["id"].(string)
		info, _ := rc["additionalInfo"].(map[string]interface{})
		if info == nil {
			info = make(map[string]interface{})
			rc["additionalInfo"] = info
		}
		if f, found := installedMap[id]; found {
			info["installed"] = true
			if version, _ := info["version"].(string); version != "" && version != f.Version {
				info["upgraded"] = true
			}
		}
	}
}
