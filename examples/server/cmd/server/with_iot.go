//go:build with_iot

package main

import (
	"examples/server/internal/service"

	"github.com/rulego/rulego-components-iot/external/serial"

	// 注册扩展组件库
	// 使用`go build -tags with_iot .`把扩展组件编译到运行文件
	_ "github.com/rulego/rulego-components-iot/endpoint/opcua"
	_ "github.com/rulego/rulego-components-iot/external/modbus"
	_ "github.com/rulego/rulego-components-iot/external/opcua"
)

func init() {
	// 获取串口列表函数，支持实时获取
	getSerialPorts := func() interface{} {
		serialPortsList, _ := serial.GetPortsList()
		return map[string]interface{}{
			"port": serialPortsList,
		}
	}
	service.RegisterBuiltin("x/serialIn", getSerialPorts)
	service.RegisterBuiltin("x/serialOut", getSerialPorts)
	service.RegisterBuiltin("x/serialRequest", getSerialPorts)
	service.RegisterBuiltin("x/serialControl", getSerialPorts)
}
