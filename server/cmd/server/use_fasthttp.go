//go:build use_fasthttp

package main

import (
	// Use FastHTTP Endpoint instead of the standard HTTP Endpoint component
	// Use `go build -tags with_fasthttp .` to include the FastHTTP extension components in the executable
	// Above 300 concurrency, performance is tripled compared to standard HTTP endpoint components
	_ "github.com/rulego/rulego-components/endpoint/fasthttp"
	_ "github.com/rulego/rulego-components/external/fasthttp"
)
