package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rulego/rulego/server/bootstrap"
	"github.com/rulego/rulego/server/internal/constants"
)

var (
	configFile = flag.String("c", "config.conf", "配置文件路径")
	version    = flag.Bool("v", false, "显示版本")
)

// BuildTime 由发布流水线经 -ldflags -X 注入；版本号取 constants.ServerVersion，
// 同样支持 -ldflags -X 注入，本地构建显示默认值
var BuildTime = "unknown"

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("rulego-server %s (built %s)\n", constants.ServerVersion, BuildTime)
		return
	}

	application := bootstrap.DefaultApp(*configFile)

	// AI 工具安全拦截 hook（仅 with_ai/with_all 编译时生效）
	registerAiSecurityHook(application)

	if err := bootstrap.Run(application); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
