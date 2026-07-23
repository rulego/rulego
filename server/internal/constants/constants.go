package constants

const (
	// DirWorkflows workflow directory
	DirWorkflows          = "workflows"
	DirPublic             = "public"
	DirWorkflowsRun       = "runs"
	DirWorkflowsRule      = "rules"
	DirWorkflowsComponent = "components"
	// DirSystem system-level data directory
	DirSystem = "system"
	// DirSystemAgents system-level built-in agent directory
	DirSystemAgents = "system/agents"
	// FileNameIndex indexes the filename
	FileNameIndex = "index"
)

const (
	KeyMsgType           = "msgType"
	KeyMsgId             = "msgId"
	KeyChainId           = "chainId"
	KeyNodeId            = "nodeId"
	KeyUsername          = "username"
	KeyClientId          = "clientId"
	KeyKeywords          = "keywords"
	KeyId                = "id"
	KeyLang              = "lang"
	KeyRoot              = "root"
	KeyDisabled          = "disabled"
	KeySize              = "size"
	KeyPage              = "page"
	KeyType              = "type"
	KeyWorkDir           = "workDir"
	KeyUpdateTime        = "updateTime"
	KeyCategory          = "category"
	KeySystemAgent       = "systemAgent"
	KeyFilePathWhitelist = "filePathWhitelist"
)

// Query parameter names
const (
	ParamMsgId             = "_msgId"
	ParamHeadersToMetadata = "_headersToMetadata"
	ParamOnlyNodeId        = "_onlyNodeId"
	ParamFromNodeId        = "_fromNodeId"
	ParamTargetNodePath    = "_targetNodePath"
	ParamRootChainId       = "_rootChainId"
)

// Metadata key
const (
	MetaApiKey   = "apiKey"
	MetaStream   = "stream"
	MetaDebugKey = "debug_"
)

// Routing paths
const (
	PathHealth = "/health"
	PathEditor = "/editor/"
	PathLogin  = "/login"
	PathApi    = "/api/"
)

// Certification-related
const (
	BearerPrefix = "Bearer "
)

// Message type
const (
	MsgTypeChatCompletions = "chat.completions"
)

// The service name inside the container
const (
	SvcRuleCatalog       = "module.rule.catalog"
	SvcRuleExecutor      = "module.rule.executor"
	SvcRuleManager       = "module.rule.manager"
	SvcRuleEngineManager = "module.rule.engine_manager"
	SvcNodeService       = "module.node.service"
	SvcRunLogService     = "module.runlog.service"
	SvcLocaleService     = "module.locale.service"
	SvcMarketplaceSvc    = "module.marketplace.service"
	SvcMcpService        = "module.mcp.service"
	SvcConfigService     = "module.system.settings"
)

const (
	SettingKeyLatestChainId = "latestChainId"
	SettingKeyMainChainId   = "mainChainId"
)

const (
	RuleChainFileSuffix = ".json"
	RunLogFileSuffix    = ".jsonl"
	RunLogDbFile        = "runlog.db"
)

const (
	AddiKeyMessage = "message"
)

const (
	LoadLuaLibs = "load_lua_libs"
)

const (
	KeyInMessage = "inMessage"
	KeyBody      = "body"
)
