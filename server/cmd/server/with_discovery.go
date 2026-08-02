//go:build with_discovery || with_all

package main

import (
	// 注册 nacos 服务发现组件（节点 + endpoint），由各包 init() 自注册到 Registry
	_ "github.com/rulego/rulego-components-discovery/endpoint/nacos"
	_ "github.com/rulego/rulego-components-discovery/external/nacos"
)
