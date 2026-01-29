# Endpoint

[English](README.md)| 中文

**Endpoint** 是一个用来抽象不同输入源数据路由的模块，针对不同协议提供**一致**的使用体验，它是`RuleGo`一个可选模块，能让RuleGo实现独立运行提供服务的能力。

它可以让你方便地创建和启动不同的接收服务，如http、mqtt、kafka、gRpc、websocket、schedule、tpc、udp等，实现对异构系统数据集成，然后根据不同的请求或消息，进行转换、处理、流转等操作，最终交给规则链或者组件处理。

另外它支持通过`DSL`动态方式创建和更新。

<img src="../doc/imgs/endpoint/endpoint.png">
<div style="text-align: center;">Endpoint架构图</div>

## 使用

1. 首先定义路由，路由提供了流式的调用方式，包括输入端、处理函数和输出端。不同Endpoint其路由处理是`一致`的

```go
router := endpoint.Registry.NewRouter().From("/api/v1/msg/").Process(func(exchange *endpoint.Exchange) bool {
//处理逻辑
return true
}).To("chain:default")
```
不同`Endpoint`类型，输入端`From`代表的含义会有不同，但最终会根据`From`值路由到该路由器：
- http/websocket endpoint：代表路径路由，根据`From`值创建指定的http服务。例如：From("/api/v1/msg/")表示创建/api/v1/msg/ http服务。
- mqtt/kafka endpoint：代表订阅的主题，根据`From`值订阅相关主题。例如：From("/api/v1/msg/")表示订阅/api/v1/msg/主题。
- schedule endpoint：代表cron表达式，根据`From`值创建相关定时任务。例如：From("*/1 * * * * *")表示每隔1秒触发该路由器。
- tpc/udp endpoint：代表正则表达式，根据`From`值把满足条件的消息转发到该路由。例如：From("^{.*")表示满足`{`开头的数据。

2. 然后创建Endpoint服务，创建接口也是`一致`的：

```go
//例如：创建http 服务
restEndpoint, err := endpoint.Registry.New(rest.Type, config, rest.Config{Server: ":9090",})
// 或者使用map方式设置配置
restEndpoint, err := endpoint.Registry.New(rest.Type, config, types.Configuration{"server": ":9090",})

//例如：创建mqtt订阅 服务
mqttEndpoint, err := endpoint.Registry.New(mqtt.Type, config, mqtt.Config{Server: "127.0.0.1:1883",})
// 或者使用map方式设置配置
mqttEndpoint, err := endpoint.Registry.New(mqtt.Type, config, types.Configuration{"server": "127.0.0.1:1883",})

//例如：创建ws服务
wsEndpoint, err := endpoint.Registry.New(websocket.Type, config, websocket.Config{Server: ":9090"})

//例如：创建tcp服务
tcpEndpoint, err := endpoint.Registry.New(net.Type, config, Config{Protocol: "tcp", Server:   ":8888",})

//例如： 创建schedule endpoint服务
scheduleEndpoint, err := endpoint.Registry.New(schedule.Type, config, nil)
```

3. 把路由注册到endpoint服务中，并启动服务
```go
//http endpoint注册路由
_, err = restEndpoint.AddRouter(router1,"POST")
_, err = restEndpoint.AddRouter(router2,"GET")
_ = restEndpoint.Start()

//mqtt endpoint注册路由
_, err = mqttEndpoint.AddRouter(router1)
_, err = mqttEndpoint.AddRouter(router2)
_ = mqttEndpoint.Start()
```

4. Endpoint支持响应给调用方
```go
router5 := endpoint.Registry.NewRouter().From("/api/v1/msgToComponent2/:msgType").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
    //响应给客户端
    exchange.Out.Headers().Set("Content-Type", "application/json")
    exchange.Out.SetBody([]byte("ok"))
    return true
})
//如果需要把规则链执行结果同步响应给客户端，则增加wait语义
router5 := endpoint.Registry.NewRouter().From("/api/v1/msg2Chain4/:chainId").
To("chain:${chainId}").
//必须增加Wait，异步转同步，http才能正常响应，如果不响应同步响应，不要加这一句，会影响吞吐量
Wait().
Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
  err := exchange.Out.GetError()
  if err != nil {
    //错误
    exchange.Out.SetStatusCode(400)
    exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
    } else {
    //把处理结果响应给客户端，http endpoint 必须增加 Wait()，否则无法正常响应
    outMsg := exchange.Out.GetMsg()
    exchange.Out.Headers().Set("Content-Type", "application/json")
    exchange.Out.SetBody([]byte(outMsg.Data))
  }

  return true
}).End()
```

5.  添加全局拦截器，用来进行权限校验等逻辑
```go
restEndpoint.AddInterceptors(func(exchange *endpoint.Exchange) bool {
  //权限校验逻辑
  return true
})
```

## Router

参考[文档](https://rulego.cc/pages/endpoint-router/) 

## 示例

以下是使用endpoint的示例代码：
- [RestEndpoint](/examples/http_endpoint/http_endpoint.go)
- [WebsocketEndpoint](/endpoint/websocket/websocket_test.go)
- [MqttEndpoint](/endpoint/mqtt/mqtt_test.go)
- [ScheduleEndpoint](/endpoint/schedule/schedule_test.go)
- [NetEndpoint](/endpoint/net/net_test.go)
- [KafkaEndpoint](https://github.com/rulego/rulego-components/blob/main/endpoint/kafka/kafka_test.go) （扩展组件库）    

## 扩展endpoint

**Endpoint模块** 提供了一些内置的接收服务类型，但是你也可以自定义或扩展其他类型的接收服务。要实现这个功能，你需要遵循以下步骤：

1. 实现[Message接口](/endpoint/endpoint.go#L62) 。Message接口是一个用来抽象不同输入源数据的接口，它定义了一些方法来获取或设置消息的内容、头部、来源、参数、状态码等。你需要为你的接收服务类型实现这个接口，使得你的消息类型可以和endpoint包中的其他类型进行交互。
2. 实现[Endpoint接口](/endpoint/endpoint.go#L40) 。Endpoint接口是一个用来定义不同接收服务类型的接口，它定义了一些方法来启动、停止、添加路由和拦截器等。你需要为你的接收服务类型实现这个接口，使得你的服务类型可以和endpoint包中的其他类型进行交互。

以上就是扩展endpoint包的基本步骤，你可以参考已经有的endpoint类型实现来编写你自己的代码：
- [rest](https://github.com/rulego/rulego/tree/main/endpoint/rest/rest.go)
- [websocket](https://github.com/rulego/rulego/tree/main/endpoint/websocket/websocket.go)
- [mqtt](https://github.com/rulego/rulego/tree/main/endpoint/mqtt/mqtt.go)
- [schedule](https://github.com/rulego/rulego/tree/main/endpoint/schedule/schedule.go)
- [tcp/udp](https://github.com/rulego/rulego/tree/main/endpoint/net/net.go)
- [Kafka](https://github.com/rulego/rulego-components/blob/main/endpoint/kafka/kafka.go) （扩展组件库）