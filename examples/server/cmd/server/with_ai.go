//go:build with_ai

package main

import (
	//注册AI扩展组件库
	// 使用`go build -tags with_ai .`把扩展组件编译到运行文件
	_ "github.com/rulego/rulego-components-ai/ai/action"
	_ "github.com/rulego/rulego-components-ai/ai/agent"
	_ "github.com/rulego/rulego-components-ai/ai/endpoint"
	_ "github.com/rulego/rulego-components-ai/ai/external/embedding"
	_ "github.com/rulego/rulego-components-ai/ai/external/mcp"
	_ "github.com/rulego/rulego-components-ai/ai/external/redis"
)
