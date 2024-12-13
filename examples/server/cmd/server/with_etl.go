//go:build with_etl

package main

import (
	// 注册ETL扩展组件库
	// 使用`go build -tags with_etl .`把ETL扩展组件编译到运行文件
	_ "github.com/rulego/rulego-components-etl/endpoint/mysql_cdc"
)
