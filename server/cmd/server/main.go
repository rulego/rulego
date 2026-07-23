package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rulego/rulego/server/bootstrap"
)

var (
	configFile = flag.String("c", "config.conf", "配置文件路径")
	version    = flag.Bool("v", false, "显示版本")
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("rulego-server %s (built %s)\n", Version, BuildTime)
		return
	}

	application := bootstrap.DefaultApp(*configFile)

	// AI tool securely intercepts hooks (effective only at with_ai/with_all compile time)
	registerAiSecurityHook(application)

	if err := bootstrap.Run(application); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
