//go:build with_iot || with_all

package main

import (
	"github.com/rulego/rulego-components-iot/external/serial"
	"github.com/rulego/rulego/server/internal/registry"

	// IoT Endpoint
	_ "github.com/rulego/rulego-components-iot/endpoint/opcua"

	// IoT External
	_ "github.com/rulego/rulego-components-iot/external/modbus"
	_ "github.com/rulego/rulego-components-iot/external/opcua"
)

func init() {
	// 注册串口组件选项
	getSerialPorts := func() interface{} {
		serialPortsList, _ := serial.GetPortsList()
		return map[string]interface{}{
			"port": serialPortsList,
		}
	}
	registry.RegisterBuiltin("x/serialIn", getSerialPorts)
	registry.RegisterBuiltin("x/serialOut", getSerialPorts)
	registry.RegisterBuiltin("x/serialRequest", getSerialPorts)
	registry.RegisterBuiltin("x/serialControl", getSerialPorts)
}
