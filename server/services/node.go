package services

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/node_pool"
)

// ComponentCatalog Component directory service interface
type ComponentCatalog interface {
	List(keywords string, size, page int) ([]types.RuleChain, int, error)
	Get(nodeType string) ([]byte, error)
}

// ComponentAdmin Component Management Service interface
type ComponentAdmin interface {
	Install(id string, dsl []byte) error
	Upgrade(id string, dsl []byte) error
	Uninstall(nodeType string) error
	LoadComponents()
	ComponentsRegistry() types.ComponentRegistry
}

// McpToolService MCP tool interface is used to synchronize MCP tools during component installation/uninstallation
type McpToolService interface {
	// AddToolsFromComponent: Adds MCP tools from component definitions, scoped to user
	AddToolsFromComponent(username, componentType string, def types.ComponentForm)
	// DeleteTools Removes MCP tools, scoped to user
	DeleteTools(username string, names ...string)
}

// NodePoolCatalog The node pool directory interface
type NodePoolCatalog interface {
	List(page, size int, keywords, category string) ([]interface{}, int, error)
	Get(id, nodeType string) (*types.RuleNode, error)
}

// NodePoolAdmin Node pool management interface
type NodePoolAdmin interface {
	SaveNode(node types.RuleNode) error
	SaveEndpoint(endpoint types.EndpointDsl) error
	Delete(id, nodeType string) error
	Load() error
	GetPool() *node_pool.NodePool
}

// NodeService is the node service storefront, providing user-isolated component and node pool management
type NodeService interface {
	// Component operations
	ListComponents(username, keywords string, size, page int) ([]types.RuleChain, int, error)
	GetComponent(username, nodeType string) ([]byte, error)
	InstallComponent(username, id string, dsl []byte) error
	UpgradeComponent(username, id string, dsl []byte) error
	UninstallComponent(username, nodeType string) error
	// NodePool operations
	ListNodePool(username string, page, size int, keywords, category string) ([]interface{}, int, error)
	GetNodePool(username, id, nodeType string) (*types.RuleNode, error)
	SaveNodePoolNode(username string, node types.RuleNode) error
	SaveNodePoolEndpoint(username string, endpointDef types.EndpointDsl) error
	DeleteNodePool(username, id, nodeType string) error
	GetNodePoolDefs(username string) (map[string][]*types.RuleNode, error)
	// ComponentForms
	GetComponentForms(username string) []types.ComponentForm
}
