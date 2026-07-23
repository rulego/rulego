package constants

const (
	// DirWorkflows workflow directory
	DirWorkflows          = "workflows"
	DirLocales            = "locales"
	DirWorkflowsRun       = "runs"
	DirWorkflowsRule      = "rules"
	DirWorkflowsComponent = "components"
	// FileNameIndex indexes the filename
	FileNameIndex = "index"
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
	KeyRoot            = "root"
	KeyDisabled        = "disabled"
	KeyWebhookSecret   = "webhookSecret"
	KeyIntegrationType = "integrationType"
	// KeyWorkDir work directory
	KeyWorkDir = "workDir"
	// KeyDefaultIntegrationChainId applies the Integration Rule Chain ID
	KeyDefaultIntegrationChainId = "$event_bus"
	KeyUpdateTime                = "updateTime"
	KeyHeadersToMetadata         = "headersToMetadata"
	KeyInMessage                 = "inMessage"
	KeyBody                      = "body"

	// KeyFilePathWhitelist configuration key for the file path whitelist
	KeyFilePathWhitelist = "filePathWhitelist"
)

const (
	// OperateDeploy deploys the rule chain.
	OperateDeploy = "start"
	// OperateUndeploy is taken down
	OperateUndeploy = "stop"
	// OperateSetToMain is set to the main rule chain
	OperateSetToMain = "set-to-main"
)
const (
	// SettingKeyLatestChainId The latest opened rule chain
	SettingKeyLatestChainId = "latestChainId"
	// SettingKeyMainChainId is the main rule chain, and all server events are sent here
	SettingKeyMainChainId = "mainChainId"
)

const (
	UserSuper = "super"
	UserAdmin = "admin"
)
const (
	RuleChainFileSuffix = ".json"
)
const (
	// AddiKeyMessage records error loading of the rule chain, extension field error information Key
	AddiKeyMessage = "message"
)
const (
	KeyAuthorization = "Authorization"
	KeyBearer        = "Bearer "
)

// LoadLuaLibs loads the lua library key
const LoadLuaLibs = "load_lua_libs"

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
