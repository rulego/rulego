package endpoint

import (
	"sort"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/registry"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerComponentRoutes(ep endpointApi.HttpEndpoint) {
	ep.GET(endpoint.NewRouter().From(s.apiBasePath() + "/components").Process(s.authWithPermission("component", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		username := metadataUsername(exchange)

		var nodeForms []types.ComponentForm
		var nodePoolDefs map[string][]*types.RuleNode
		if nodeSvc, _ := app.GetAs[services.NodeService](s.container, services.KeyNodeService); nodeSvc != nil {
			nodeForms = nodeSvc.GetComponentForms(username)
			nodePoolDefs, _ = nodeSvc.GetNodePoolDefs(username)
		}

		builtins := map[string]interface{}{
			"endpoints": map[string]interface{}{
				"inProcessors":  processor.InBuiltins.Names(),
				"outProcessors": processor.OutBuiltins.Names(),
			},
			"nodePool": nodePoolDefs,
		}
		for k, v := range registry.Builtins() {
			builtins[k] = v
		}
		if aiTools, ok := builtins["ai/tools"].(map[string]interface{}); ok {
			if tools, ok := aiTools["tools"].([]interface{}); ok {
				aiTools["tools"] = registry.ApplySkillToolPathToForms(tools, s.config.SkillPath)
				builtins["ai/tools"] = aiTools
			}
		}

		// 全局变量名（只含 key，不含值），供前端 ${global.xxx} 补全
		if s.config != nil && s.config.Global != nil {
			globals := make([]string, 0, len(s.config.Global))
			for k := range s.config.Global {
				globals = append(globals, k)
			}
			sort.Strings(globals)
			builtins["globals"] = globals
		}

		writeJSON(exchange, map[string]interface{}{
			"endpoints": endpoint.Registry.GetComponentForms().Values(),
			"nodes":     nodeForms,
			"tools":     nil,
			"builtins":  builtins,
			"skillPath": configuredSkillPath(s.config),
		})
		return true
	}).End())
}
