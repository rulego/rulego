package constants

const (
	// DirWorkflows 工作流目录
	DirWorkflows          = "workflows"
	DirPublic             = "public"
	DirWorkflowsRun       = "runs"
	DirWorkflowsRule      = "rules"
	DirWorkflowsComponent = "components"
	// DirSystem 系统级数据目录
	DirSystem = "system"
	// DirSystemAgents 系统级内置智能体目录
	DirSystemAgents = "system/agents"
	// FileNameIndex 索引文件名
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

// 查询参数名
const (
	ParamMsgId             = "_msgId"
	ParamHeadersToMetadata = "_headersToMetadata"
	ParamOnlyNodeId        = "_onlyNodeId"
	ParamFromNodeId        = "_fromNodeId"
	ParamTargetNodePath    = "_targetNodePath"
)

// 元数据键
const (
	MetaApiKey   = "apiKey"
	MetaStream   = "stream"
	MetaDebugKey = "debug_"
)

// 路由路径
const (
	PathHealth   = "/health"
	PathEditor   = "/editor/"
	PathLogin    = "/login"
	PathApi      = "/api/"
)

// 认证相关
const (
	BearerPrefix = "Bearer "
)

// 消息类型
const (
	MsgTypeChatCompletions = "chat.completions"
)

// 容器中的服务名
const (
	SvcRuleCatalog      = "module.rule.catalog"
	SvcRuleExecutor     = "module.rule.executor"
	SvcRuleManager      = "module.rule.manager"
	SvcRuleEngineManager = "module.rule.engine_manager"
	SvcNodeService      = "module.node.service"
	SvcRunLogService    = "module.runlog.service"
	SvcLocaleService    = "module.locale.service"
	SvcMarketplaceSvc   = "module.marketplace.service"
	SvcMcpService       = "module.mcp.service"
	SvcConfigService    = "module.system.settings"
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
