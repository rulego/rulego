//go:build with_extend

package main

import (
	// 注册扩展组件库
	// 使用`go build -tags with_extend .`把扩展组件编译到运行文件
	_ "github.com/rulego/rulego-components/endpoint/grpc_stream"
	_ "github.com/rulego/rulego-components/endpoint/kafka"
	_ "github.com/rulego/rulego-components/endpoint/nats"
	_ "github.com/rulego/rulego-components/endpoint/rabbitmq"
	_ "github.com/rulego/rulego-components/endpoint/redis"
	_ "github.com/rulego/rulego-components/endpoint/redis_stream"
	_ "github.com/rulego/rulego-components/external/grpc" //编译后文件大约增加7M
	_ "github.com/rulego/rulego-components/external/kafka"
	_ "github.com/rulego/rulego-components/external/mongodb"
	_ "github.com/rulego/rulego-components/external/nats"
	_ "github.com/rulego/rulego-components/external/opengemini"
	_ "github.com/rulego/rulego-components/external/otel"
	_ "github.com/rulego/rulego-components/external/rabbitmq"
	_ "github.com/rulego/rulego-components/external/redis"
	_ "github.com/rulego/rulego-components/filter"
	_ "github.com/rulego/rulego-components/transform"
)
