// modules.go 提供通用模块目录,应用按需挑选,无需为每个应用维护预设子集。
package bootstrap

import (
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/modules/iotpoint"
	"github.com/rulego/rulego/server/internal/modules/locale"
	"github.com/rulego/rulego/server/internal/modules/marketplace"
	"github.com/rulego/rulego/server/internal/modules/mcp"
	"github.com/rulego/rulego/server/internal/modules/node"
	"github.com/rulego/rulego/server/internal/modules/rule"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/internal/modules/skill"
	"github.com/rulego/rulego/server/internal/modules/system"
	"github.com/rulego/rulego/server/internal/modules/user"
)

// 通用模块构造器。各 New() 返回 *Module 具体类型,这里统一包成 func() app.Module,
// 供应用按需挑选。应用据此自行声明所需子集,server 不再为每个应用硬编码预设函数。
var (
	User        = func() app.Module { return user.New() }
	Rule        = func() app.Module { return rule.New() }
	Node        = func() app.Module { return node.New() }
	RunLog      = func() app.Module { return runlog.New() }
	Locale      = func() app.Module { return locale.New() }
	Skill       = func() app.Module { return skill.New() }
	System      = func() app.Module { return system.New() }
	Marketplace = func() app.Module { return marketplace.New() }
	MCP         = func() app.Module { return mcp.New() }
	IoTPoint    = func() app.Module { return iotpoint.New() }
)

// Modules 按传入的构造器生成模块列表。应用自行声明所需模块,server 无需为每个应用维护预设。
func Modules(factories ...func() app.Module) []app.Module {
	out := make([]app.Module, len(factories))
	for i, f := range factories {
		out[i] = f()
	}
	return out
}
