//go:build with_ci

package main

import (
	// 注册CI/CD扩展组件库
	// 使用`go build -tags with_ci .`把CI/CD扩展组件编译到运行文件
	_ "github.com/rulego/rulego-components-ci/ci/action"
)
