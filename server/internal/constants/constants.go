package constants

const (
	DirWorkflows          = "workflows"
	DirPublic             = "public"
	DirWorkflowsRun       = "runs"
	DirWorkflowsRule      = "rules"
	DirWorkflowsComponent = "components"
	// DirSystem 系统级数据目录
	DirSystem = "system"
	// DirSystemAgents 系统级内置智能体目录
	DirSystemAgents = "system/agents"
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
	ParamRootChainId       = "_rootChainId"
	// ParamTriggerSource 运行日志的触发来源，由 endpoint 写入消息 metadata，
	// OnRuleChainCompleted 回调读取后记入运行日志。
	ParamTriggerSource = "_triggerSource"
)

// 元数据键
const (
	MetaApiKey   = "apiKey"
	MetaStream   = "stream"
	MetaDebugKey = "debug_"
)

// 路由路径
const (
	PathHealth = "/health"
	PathEditor = "/editor/"
	PathLogin  = "/login"
	PathApi    = "/api/"
)

// 认证相关
const (
	BearerPrefix = "Bearer "
)

// ServerVersion 服务端版本号，由 GET /api/v1/version 返回
const ServerVersion = "0.37.0"

// 权限资源名（authWithPermission 的 resource 入参）
const (
	ResourceRule = "rule"
	ResourceLog  = "log"
	ResourceUser = "user"
)

// 消息类型
const (
	MsgTypeChatCompletions = "chat.completions"
)

// 容器中的服务名
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
