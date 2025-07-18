# MQTT端点示例

这个示例演示了如何使用RuleGo的MQTT端点来处理二进制和JSON数据，并使用JavaScript转换节点进行数据处理。这是一个单向消息处理示例（不需要响应）。

## 功能特性

- **MQTT端点服务器**：基于MQTT协议的消息订阅端点
- **多主题订阅**：支持多个MQTT主题模式
- **数据类型支持**：自动识别和处理JSON和二进制数据
- **JavaScript处理**：使用js_transform_node进行灵活的数据转换
- **主题路由**：基于MQTT主题通配符的消息路由
- **单向处理**：专注于消息接收和处理，无响应机制

## 项目结构

```
mqtt_endpoint_example/
├── server/           # 服务端
│   ├── server.go     # MQTT端点服务器
│   └── chain_dsl.json # 规则链DSL配置
├── client/           # 客户端  
│   └── client.go     # MQTT客户端示例
└── README.md         # 说明文档
```

## 数据处理功能

### JSON传感器数据处理（sensors/+/data）
- **传感器监控**：温度、湿度、电池电量监控
- **多级报警**：
  - 温度>30°C：高温报警
  - 湿度>70%：高湿度报警  
  - 电量<20%：低电量报警
- **状态跟踪**：处理时间戳和状态记录
- **主题解析**：自动提取传感器ID

### 二进制设备命令处理（devices/+/command）
- **协议解析**：deviceId(2字节) + command(1字节) + value(4字节)
- **命令类型**：
  - `0x01` SET_PARAMETER - 设置参数
  - `0x02` GET_STATUS - 获取状态
  - `0x03` RESET - 设备重置
  - `0x04` SET_THRESHOLD - 设置阈值
- **优先级管理**：根据命令类型设置处理优先级
- **设备追踪**：记录设备ID和命令历史

### 系统消息处理（system/#）
- **日志级别**：INFO、WARNING、ERROR
- **紧急程度**：
  - ERROR：高紧急度，需要立即关注
  - WARNING：中等紧急度，需要监控
  - INFO：低紧急度，仅记录日志
- **系统上下文**：主机名、环境、版本信息
- **来源追踪**：消息来源和模块标识

## 快速开始

### 前置条件

确保有可用的MQTT代理：
```bash
# 使用Docker启动MQTT代理
docker run -it -p 1883:1883 eclipse-mosquitto:latest

# 或者安装本地MQTT代理
# Ubuntu/Debian: sudo apt-get install mosquitto
# macOS: brew install mosquitto
```

### 1. 启动服务器

```bash
cd examples/mqtt_endpoint_example/server
go run server.go
```

服务器将连接到 `127.0.0.1:1883` 并订阅：
- `sensors/+/data` - JSON传感器数据
- `devices/+/command` - 二进制设备命令
- `system/#` - 系统消息

### 2. 运行客户端

```bash
cd examples/mqtt_endpoint_example/client
go run client.go
```

客户端将发布：
- 3条传感器JSON数据到不同传感器主题
- 4条二进制设备命令到不同设备主题
- 3条系统消息到不同级别主题

## 示例数据

### JSON传感器数据
```json
{
  "sensorId": "TEMP_001",
  "temperature": 35.8,
  "humidity": 58.7,
  "timestamp": 1735534123,
  "location": "Warehouse B",
  "batteryLevel": 92.3
}
```

### 二进制命令格式
```
设备ID(2字节) + 命令(1字节) + 值(4字节)
例如：03 E9 01 00 00 00 64 (设备1001, 命令0x01, 值100)
```

### 系统消息
```json
{
  "messageId": "SYS_002",
  "level": "WARNING", 
  "source": "TemperatureMonitor",
  "content": "High temperature detected in zone B",
  "timestamp": 1735534123
}
```

## MQTT主题结构

### 传感器数据主题
- `sensors/TEMP_001/data` - 温度传感器001
- `sensors/TEMP_002/data` - 温度传感器002  
- `sensors/HUM_001/data` - 湿度传感器001

### 设备命令主题
- `devices/1001/command` - 设备1001命令
- `devices/1002/command` - 设备1002命令
- `devices/1003/command` - 设备1003命令

### 系统消息主题
- `system/INFO` - 信息级别消息
- `system/WARNING` - 警告级别消息
- `system/ERROR` - 错误级别消息

## 配置说明

### MQTT连接配置
- **代理地址**：127.0.0.1:1883
- **QoS级别**：1（至少一次投递）
- **客户端ID**：每个端点使用不同的客户端ID
- **认证**：无（演示环境）

### 端点配置（chain_dsl.json）
- **端点数量**：3个独立的MQTT端点
- **主题订阅**：
  - JSON数据端点：`sensors/+/data`
  - 二进制数据端点：`devices/+/command`
  - 系统消息端点：`system/#`

### 处理器配置
- **setJsonDataType**：设置JSON数据类型
- **setBinaryDataType**：设置二进制数据类型

## 处理结果示例

### 传感器数据处理结果
```json
{
  "sensorId": "TEMP_002",
  "temperature": 35.8,
  "humidity": 58.7,
  "alert": "HIGH_TEMPERATURE",
  "alertLevel": "WARNING",
  "humidityAlert": "NORMAL",
  "processedAt": "2024-06-30T15:30:45.120Z",
  "status": "processed"
}
```

### 设备命令处理结果
```json
{
  "deviceId": 1001,
  "command": 1,
  "value": 100,
  "commandName": "SET_PARAMETER", 
  "response": "参数已设置为 100",
  "priority": "NORMAL",
  "status": "processed",
  "processedAt": "2024-06-30T15:30:45.125Z"
}
```

## 技术要点

1. **DSL嵌入式端点**：使用规则链DSL中的endpoints定义
2. **多端点架构**：每种数据类型使用独立的MQTT端点
3. **主题通配符**：支持MQTT标准通配符（+和#）
4. **数据类型处理器**：使用内置处理器设置正确的数据类型
5. **JavaScript处理**：复杂业务逻辑通过JS脚本实现
6. **QoS保证**：使用QoS 1确保消息可靠传递

## 扩展建议

1. **认证**：添加MQTT用户名/密码认证
2. **TLS**：启用MQTT over TLS加密传输
3. **持久化**：将处理结果存储到数据库
4. **监控**：集成Prometheus/Grafana监控
5. **告警**：添加告警通知机制（邮件、短信等）

## 注意事项

- 需要运行MQTT代理（mosquitto等）
- 确保端口1883未被占用或防火墙阻止
- 客户端和服务器可以独立运行
- 服务器使用Ctrl+C优雅关闭
- 支持多个客户端同时发布消息