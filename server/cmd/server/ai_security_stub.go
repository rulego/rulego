//go:build !with_ai && !with_all

package main

import "github.com/rulego/rulego/server/app"

// registerAiSecurityHook is an empty implementation without AI components
func registerAiSecurityHook(_ *app.App) {}
