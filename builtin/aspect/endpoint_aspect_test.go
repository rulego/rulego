package aspect

import (
	"testing"

	"github.com/rulego/rulego/api/types"
)

func TestProcessEndpointDsl(t *testing.T) {
	config := types.NewConfig()
	ruleChain := &types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{
			Configuration: types.Configuration{
				types.Vars: map[string]interface{}{
					"port":      "8080",
					"path":      "/api/test",
					"processor": "log",
					"param":     "GET",
					"target":    "chain:default",
				},
			},
		},
	}

	endpointDsl := &types.EndpointDsl{
		Processors: []string{"${vars.processor}", "auth"},
		Routers: []*types.RouterDsl{
			{
				Params: []interface{}{"${vars.param}", "POST"},
				From: types.FromDsl{
					Path: "${vars.path}",
					Configuration: types.Configuration{
						"timeout": "${vars.port}",
					},
					Processors: []string{"${vars.processor}"},
				},
				To: types.ToDsl{
					Path: "${vars.target}",
					Configuration: types.Configuration{
						"retry": "3",
					},
					Processors: []string{"transform"},
				},
			},
		},
	}
	// Set some initial configuration to verify it's processed too
	endpointDsl.Configuration = types.Configuration{
		"server": "http://localhost:${vars.port}",
	}

	processEndpointDsl(config, ruleChain, endpointDsl)

	if endpointDsl.Processors[0] != "log" {
		t.Errorf("Expected Processors[0] to be 'log', got '%s'", endpointDsl.Processors[0])
	}
	if endpointDsl.Processors[1] != "auth" {
		t.Errorf("Expected Processors[1] to be 'auth', got '%s'", endpointDsl.Processors[1])
	}
	if endpointDsl.Configuration["server"] != "http://localhost:8080" {
		t.Errorf("Expected Configuration['server'] to be 'http://localhost:8080', got '%s'", endpointDsl.Configuration["server"])
	}

	router := endpointDsl.Routers[0]
	if router.Params[0] != "GET" {
		t.Errorf("Expected Params[0] to be 'GET', got '%s'", router.Params[0])
	}
	if router.Params[1] != "POST" {
		t.Errorf("Expected Params[1] to be 'POST', got '%s'", router.Params[1])
	}

	if router.From.Path != "/api/test" {
		t.Errorf("Expected From.Path to be '/api/test', got '%s'", router.From.Path)
	}
	if router.From.Configuration["timeout"] != "8080" {
		t.Errorf("Expected From.Configuration['timeout'] to be '8080', got '%s'", router.From.Configuration["timeout"])
	}
	if router.From.Processors[0] != "log" {
		t.Errorf("Expected From.Processors[0] to be 'log', got '%s'", router.From.Processors[0])
	}

	if router.To.Path != "chain:default" {
		t.Errorf("Expected To.Path to be 'chain:default', got '%s'", router.To.Path)
	}
}
