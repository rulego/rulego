package services

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/node_pool"
)

// ComponentCatalog 组件目录服务接口
type ComponentCatalog interface {
	List(keywords string, size, page int) ([]types.RuleChain, int, error)
	Get(nodeType string) ([]byte, error)
}

// ComponentAdmin 组件管理服务接口
type ComponentAdmin interface {
	Install(id string, dsl []byte) error
	Upgrade(id string, dsl []byte) error
	Uninstall(nodeType string) error
	LoadComponents()
	ComponentsRegistry() types.ComponentRegistry
}

// McpToolService MCP 工具服务接口，用于组件安装/卸载时同步 MCP 工具
type McpToolService interface {
	// AddToolsFromComponent 从组件定义添加 MCP 工具，scoped to user
	AddToolsFromComponent(username, componentType string, def types.ComponentForm)
	// DeleteTools 删除 MCP 工具，scoped to user
	DeleteTools(username string, names ...string)
}

// NodePoolCatalog 节点池目录接口
type NodePoolCatalog interface {
	List(page, size int, keywords, category string) ([]interface{}, int, error)
	Get(id, nodeType string) (*types.RuleNode, error)
}

// NodePoolAdmin 节点池管理接口
type NodePoolAdmin interface {
	SaveNode(node types.RuleNode) error
	SaveEndpoint(endpoint types.EndpointDsl) error
	Delete(id, nodeType string) error
	Load() error
	GetPool() *node_pool.NodePool
}

// NodeService 节点服务门面，提供按用户隔离的组件和节点池管理
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
