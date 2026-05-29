//go:build use_fasthttp

package main

import (
	// 使用fasthttp endpoint代替标准的http endpoint组件
	// 使用`go build -tags with_fasthttp .`把扩展组件编译到运行文件
	// 在300并发以上，相对于标准的http endpoint组件，性能提升3倍
	_ "github.com/rulego/rulego-components/endpoint/fasthttp"
	_ "github.com/rulego/rulego-components/external/fasthttp"
)
