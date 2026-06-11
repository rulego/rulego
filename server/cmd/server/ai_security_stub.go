//go:build !with_ai && !with_all

package main

import "github.com/rulego/rulego/server/app"

// registerAiSecurityHook 无 AI 组件时的空实现
func registerAiSecurityHook(_ *app.App) {}
