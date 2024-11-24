package constants

const (
	// DirWorkflows 工作流目录
	DirWorkflows     = "workflows"
	DirLocales       = "locales"
	DirWorkflowsRun  = "runs"
	DirWorkflowsRule = "rules"
)

const (
	KeyMsgType         = "msgType"
	KeyMsgId           = "msgId"
	KeyChainId         = "chainId"
	KeyNodeId          = "nodeId"
	KeyUsername        = "username"
	KeyClientId        = "clientId"
	KeyVarType         = "varType"
	KeySize            = "size"
	KeyPage            = "page"
	KeyId              = "id"
	KeyKeywords        = "keywords"
	KeyType            = "type"
	KeyLang            = "lang"
	KeyWebhookSecret   = "webhookSecret"
	KeyIntegrationType = "integrationType"
	// KeyWorkDir 工作目录
	KeyWorkDir = "workDir"
	// KeyDefaultIntegrationChainId 应用集成规则链ID
	KeyDefaultIntegrationChainId = "$event_bus"
)

const (
	OperateDeploy   = "start"
	OperateUndeploy = "stop"
)
const (
	// SettingKeyLatestChainId 最新打开的规则链
	SettingKeyLatestChainId = "latestChainId"
	// SettingKeyCoreChainId 默认规则链，server所有事件都会发送至此
	SettingKeyCoreChainId = "coreChainId"
)

const (
	UserSuper = "super"
	UserAdmin = "admin"
)
const (
	RuleChainFileSuffix = ".json"
)

//const (
//	DefaultPoolDef = `
//	{
//	  "ruleChain": {
//		"id": "$default_node_pool",
//		"name": "全局共享节点池"
//	  },
//	  "metadata": {
//		"endpoints": [
//		  {
//			"id": "core_endpoint_http",
//			"type": "endpoint/http",
//			"name": "http:9090",
//			"configuration": {
//			  "allowCors": true,
//			  "server": ":9090"
//			}
//		  }
//		]
//	  }
//	}
//`
//)
