//go:build with_iot || with_all

package main

import (
	"context"
	"errors"
	"time"

	"github.com/rulego/rulego-components-iot/external/opcua"
	"github.com/rulego/rulego-components-iot/external/serial"
	opcuaClient "github.com/rulego/rulego-components-iot/pkg/opcua_client"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/internal/registry"
	"github.com/rulego/rulego/utils/maps"

	// IoT Endpoint
	_ "github.com/rulego/rulego-components-iot/endpoint/hj212"
	_ "github.com/rulego/rulego-components-iot/endpoint/modbus_server"
	_ "github.com/rulego/rulego-components-iot/endpoint/opcua"
	_ "github.com/rulego/rulego-components-iot/endpoint/snmp"

	// IoT External（采集/落盘/通用节点，blank import 触发各包 init 注册）
	_ "github.com/rulego/rulego-components-iot/external/deviceio"
	_ "github.com/rulego/rulego-components-iot/external/dlt645"
	_ "github.com/rulego/rulego-components-iot/external/eip"
	_ "github.com/rulego/rulego-components-iot/external/fins"
	_ "github.com/rulego/rulego-components-iot/external/iec104"
	_ "github.com/rulego/rulego-components-iot/external/influxdb"
	_ "github.com/rulego/rulego-components-iot/external/mc"
	_ "github.com/rulego/rulego-components-iot/external/modbus"
	_ "github.com/rulego/rulego-components-iot/external/opengemini"
	_ "github.com/rulego/rulego-components-iot/external/promremote"
	_ "github.com/rulego/rulego-components-iot/external/s7"
	_ "github.com/rulego/rulego-components-iot/external/snmp"
	_ "github.com/rulego/rulego-components-iot/external/tdengine"
	_ "github.com/rulego/rulego-components-iot/external/timescaledb"
	_ "github.com/rulego/rulego-components-iot/external/tsdb"

	// IoT Action
	_ "github.com/rulego/rulego-components-iot/action/control"
)

// browseTimeout 单次在线浏览的最长耗时(连接+遍历)
const browseTimeout = 15 * time.Second

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

	// 注册 OPC UA 在线浏览动态配置（节点配置面板「浏览」按钮调用，枚举服务器地址空间）
	registry.RegisterDynamicBuiltin("opcua-browse", opcuaBrowseHandler)

	// 给 OPC UA 组件的 addr 字段(Points 表行字段)注入在线浏览标记，前端据此显示「浏览」按钮
	opcuaAddrBrowse := func() interface{} {
		return map[string]interface{}{
			"addr": map[string]interface{}{"dynamicBrowse": "opcua-browse"},
		}
	}
	registry.RegisterBuiltin("x/opcuaRead", opcuaAddrBrowse)
	registry.RegisterBuiltin("x/opcuaWrite", opcuaAddrBrowse)
}

// opcuaBrowseHandler 连接 OPC UA 服务器并浏览地址空间，返回节点树。
// 连接参数复用 external/opcua.Configuration(实现 ConfigProp)，全程带超时上限。
func opcuaBrowseHandler(params map[string]interface{}) (interface{}, error) {
	var cfg opcua.Configuration
	if err := maps.Map2Struct(params, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server == "" {
		return nil, errors.New("server is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), browseTimeout)
	defer cancel()

	holder := opcuaClient.DefaultHolder(cfg, types.DefaultLogger())
	holder.Ctx = ctx
	client, err := holder.NewOpcUaClient()
	if err != nil {
		return nil, err
	}
	// Close 用独立短超时 ctx，避免复用可能已到期的 browse ctx 导致服务端 session 未干净关闭
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = client.Close(closeCtx)
	}()
	return opcuaClient.Browse(ctx, client, strParam(params, "nodeId"), intParam(params, "depth"))
}

// strParam 从参数表取字符串值
func strParam(params map[string]interface{}, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

// intParam 从参数表取整数值（JSON 数字为 float64）
func intParam(params map[string]interface{}, key string) int {
	if v, ok := params[key].(float64); ok {
		return int(v)
	}
	return 0
}
