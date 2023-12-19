# server

[English](README.md)| 中文

该示例工程演示如何把RuleGo作为一个独立运行的规则引擎服务。你可以基于该工程进行二次开发，也可以直接下载可执行[二进制文件](https://github.com/rulego/rulego/releases)。

如果需要可视化，可以使用这个规则链编辑器工具：[RuleGoEditor](https://editor.rulego.cc/) ，配置该工程HTTP API，可以对规则链管理和调试。

提供以下功能：

* 上报数据API，并根据规则链定义交给规则引擎处理。
* 创建规则链API。
* 更新规则链API。
* 获取节点调试日志API。
* 组件列表API。
* 订阅MQTT数据，并根据根规则链定义交给规则引擎处理。

## HTTP API

* 获取所有组件列表
    - GET /api/v1/components

* 上报数据API
    - POST /api/v1/msg/{chainId}/{msgType}
    - chainId：处理数据的规则链ID
    - msgType：消息类型
    - body：消息体

* 查询规则链
    - GET /api/v1/rule/{chainId}/{nodeId}
    - chainId：规则链ID
    - nodeId:空则查询规则链定义，否则查询规则链指定节点ID节点定义

* 保存或更新规则链
    - POST /api/v1/rule/{chainId}/{nodeId}
    - chainId：规则链ID
    - nodeId：空则更新规则链定义，否则更新规则链指定节点ID节点定义
    - body：更新内容

* 获取节点调试日志API
    - Get /api/v1/event/debug?&chainId={chainId}&nodeId={nodeId}
    - chainId：规则链ID
    - nodeId：节点ID

  当节点debugMode打开后，会记录调试日志。目前该接口日志存放在内存，每个节点保存最新的40条，如果需要获取历史数据，请实现接口存储到数据库。

## MQTT客户端订阅数据

默认订阅所有主题数据，往该主题发布的数据都会根据`default`规则链定义交给规则引擎处理。 可以通过`-topics`修改订阅主题，多个以`,`号隔开

## server编译

为了节省编译后文件大小，默认不引入扩展组件[rulego-components](https://github.com/rulego/rulego-components) ，默认编译：

```shell
go build .
```

如果需要引入扩展组件[rulego-components](https://github.com/rulego/rulego-components) ，使用`with_extend`tag进行编译：

```shell
go build -tags with_extend .
```

## server启动

```shell
./server -rule_file="./rules/"
```

或者后台启动

```shell
nohup ./server -rule_file="./rules/" >> console.log &
```

启动参数

- rule_file: 规则链存储文件夹。默认:./rules/
- port: http服务器端口。默认:9090
- log_file: 日志存储文件路径。默认打印到控制台
- debug: "是否把节点调试日志打印到日志文件
- mqtt: mqtt订阅是否开启。默认:false
- server: 连接mqtt broker。默认:127.0.0.1:1883
- username：连接mqtt broker 用户名
- password：连接mqtt broker 密码
- topics：连接mqtt broker 订阅消息主题，多个主题与`,`号隔开。默认:#
 