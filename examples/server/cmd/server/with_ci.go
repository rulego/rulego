//go:build with_ci

package main

import (
	// Register the CI/CD extension component library
	// Use `go build -tags with_ci .` to include the CI/CD extension components in the executable
	_ "github.com/rulego/rulego-components-ci/ci/action"
)
