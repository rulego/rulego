//go:build with_extend || with_all

package main

import (
	// Endpoint 组件
	_ "github.com/rulego/rulego-components/endpoint/grpc_stream"
	_ "github.com/rulego/rulego-components/endpoint/kafka"
	_ "github.com/rulego/rulego-components/endpoint/nats"
	_ "github.com/rulego/rulego-components/endpoint/nsq"
	_ "github.com/rulego/rulego-components/endpoint/pulsar"
	_ "github.com/rulego/rulego-components/endpoint/rabbitmq"
	_ "github.com/rulego/rulego-components/endpoint/redis"
	_ "github.com/rulego/rulego-components/endpoint/redis_stream"
	_ "github.com/rulego/rulego-components/endpoint/wukongim"

	// External 组件
	_ "github.com/rulego/rulego-components/external/email"
	_ "github.com/rulego/rulego-components/external/file"
	_ "github.com/rulego/rulego-components/external/grpc" // 编译后文件大约增加7M
	_ "github.com/rulego/rulego-components/external/kafka"
	_ "github.com/rulego/rulego-components/external/mongodb"
	_ "github.com/rulego/rulego-components/external/nats"
	_ "github.com/rulego/rulego-components/external/nsq"
	_ "github.com/rulego/rulego-components/external/opengemini"
	_ "github.com/rulego/rulego-components/external/otel"
	_ "github.com/rulego/rulego-components/external/pulsar"
	_ "github.com/rulego/rulego-components/external/rabbitmq"
	_ "github.com/rulego/rulego-components/external/redis"
	_ "github.com/rulego/rulego-components/external/wukongim"

	// 脚本支持
	_ "github.com/rulego/rulego-components/action/python"
	_ "github.com/rulego/rulego-components/filter/lua"
	_ "github.com/rulego/rulego-components/stats/streamsql"
	_ "github.com/rulego/rulego-components/transform/lua"
)
