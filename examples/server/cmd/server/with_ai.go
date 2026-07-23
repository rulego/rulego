//go:build with_ai

package main

import (
	//Register for the AI Extension Component Library
	// Use `go build -tags with_ai .` to include the AI extension components in the executable
	_ "github.com/rulego/rulego-components-ai/ai/action"
	_ "github.com/rulego/rulego-components-ai/ai/endpoint"
)
