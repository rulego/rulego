// Package services defines service interfaces and container keyname constants.
package services

// Service keyname constants in containers.
// Service key constants for container lookups.
const (
	KeyRuleCatalog        = "module.rule.catalog"
	KeyRuleExecutor       = "module.rule.executor"
	KeyRuleManager        = "module.rule.manager"
	KeyEngineManager      = "module.rule.engine_manager"
	KeyNodeService        = "module.node.service"
	KeyRunLogService      = "module.runlog.service"
	KeyLocaleService      = "module.locale.service"
	KeyMarketplaceService = "module.marketplace.service"
	KeyMcpService         = "module.mcp.service"
	KeyConfigService      = "module.system.settings"
	KeyAuthService        = "module.user.auth"
	KeyUserProfile        = "module.user.profile"
	KeyAuthenticator      = "module.user.authenticator"
	KeyAuthorizer         = "module.user.authorizer"
	KeySkillService       = "module.skill.service"
	KeyDebugService       = "module.debug.service"
)
