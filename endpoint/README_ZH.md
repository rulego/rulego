# Endpoint

[English](README.md)| 中文

endpoint是一个用来抽象不同输入源数据路由的包，它可以让你方便地创建和启动不同的接收服务，如HTTP或MQTT，实现对异构系统数据集成，然后根据不同的请求或消息，进行转换、处理、流转等操作，最终交给规则链或者组件处理。

<img src="../doc/imgs/endpoint/endpoint.png" style="height:50%;width:80%;">

## 使用

### 创建Router

Router是用来定义路由规则的类型，它可以指定输入端、转换函数、处理函数、输出端等。你可以使用NewRouter函数来创建一个Router类型的指针，然后使用From方法来指定输入端，返回一个From类型的指针。

```go
router := endpoint.NewRouter().From("/api/v1/msg/")
```

### 添加处理函数

From类型有两个方法可以用来添加处理函数：Transform和Process。Transform方法用来转换输入消息为RuleMsg类型，Process方法用来处理输入或输出消息。这两个方法都接收一个Process类型的函数作为参数，返回一个From类型的指针。Process类型的函数接收一个Exchange类型的指针作为参数，返回一个布尔值表示是否继续执行下一个处理函数。Exchange类型是一个结构体，包含了一个输入消息和一个输出消息，用来在处理函数中传递数据。

```go
router := endpoint.NewRouter().From("/api/v1/msg/").Transform(func(exchange *endpoint.Exchange) bool {
    //转换逻辑
    return true
}).Process(func(exchange *endpoint.Exchange) bool {
    //处理逻辑
    return true
})
```

### 设置输出端

From类型有两个方法可以用来设置输出端：To和ToComponent。To方法用来指定流转目标路径或组件，ToComponent方法用来指定输出组件。这两个方法都返回一个To类型的指针。

To方法的参数是一个字符串，表示组件路径，格式为{executorType}:{path}。executorType是执行器组件类型，path是组件路径。例如："chain:{chainId}"表示执行rulego中注册的规则链，"component:{nodeType}"表示执行在config.ComponentsRegistry中注册的组件。你可以在DefaultExecutorFactory中注册自定义执行器组件类型。To方法还可以接收一些组件配置参数作为可选参数。

```go
router := endpoint.NewRouter().From("/api/v1/msg/").Transform(func(exchange *endpoint.Exchange) bool {
    //转换逻辑
    return true
}).To("chain:default")
```

ToComponent方法的参数是一个types.Node类型的组件，你可以自定义或使用已有的组件。

```go
router := endpoint.NewRouter().From("/api/v1/msg/").Transform(func(exchange *endpoint.Exchange) bool {
    //转换逻辑
    return true
}).ToComponent(func() types.Node {
        //定义日志组件，处理数据
        var configuration = make(types.Configuration)
        configuration["jsScript"] = `
        return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `
        logNode := &action.LogNode{}
        _ = logNode.Init(config, configuration)
        return logNode
}())
```

也可以使用To方法调用组件
```go
router := endpoint.NewRouter().From("/api/v1/msg/").Transform(func(exchange *endpoint.Exchange) bool {
    //转换逻辑
    return true
}).To"component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
`})
```

### 结束路由

To类型有一个方法可以用来结束路由：End。End方法返回一个Router类型的指针。

```go
router := endpoint.NewRouter().From("/api/v1/msg/").Transform(func(exchange *endpoint.Exchange) bool {
    //转换逻辑
    return true
}).To("chain:default").End()
```

### 创建RestEndPoint

RestEndPoint是一个用来创建和启动HTTP接收服务的类型，它可以注册不同的路由来处理不同的请求。你可以创建一个Rest类型的指针，并指定服务的地址和其他配置。

```go
restEndpoint := &Rest{Config: Config{Addr: ":9090"}}
```

你可以使用restEndpoint.AddInterceptors方法添加全局拦截器，用来进行权限校验等逻辑。

```go
restEndpoint.AddInterceptors(func(exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
})
```

你可以使用restEndpoint.GET或restEndpoint.POST方法注册路由，分别对应GET或POST请求方式。这些方法接收一个或多个Router类型的指针作为参数。

```go
restEndpoint.GET(router1, router2)
restEndpoint.POST(router3, router4)
```

你可以使用restEndpoint.Start方法启动服务。

```go
_ = restEndpoint.Start()
```

### 创建MqttEndpoint

MqttEndpoint是一个用来创建和启动MQTT接收服务的类型，它可以订阅不同的主题来处理不同的消息。你可以创建一个Mqtt类型的指针，并指定服务的地址和其他配置。

```go
mqttEndpoint := &Mqtt{
		Config: mqtt.Config{
			Server: "127.0.0.1:1883",
		},
}
```

你可以使用mqttEndpoint.AddInterceptors方法添加全局拦截器，用来进行权限校验等逻辑。

```go
mqttEndpoint.AddInterceptors(func(exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
})
```

你可以使用mqttEndpoint.AddRouter方法注册路由，这个方法接收一个Router类型的指针作为参数。

```go
_ = mqttEndpoint.AddRouter(router1)
```

你可以使用mqttEndpoint.Start方法启动服务。

```go
_ = mqttEndpoint.Start()
```

## 示例

以下是一些使用endpoint包的示例代码：       
[RestEndpoint](rest/rest_test.go)       
[MqttEndpoint](mqtt/mqtt_test.go)      

## 扩展endpoint

endpoint包提供了一些内置的接收服务类型，如Rest和Mqtt，但是你也可以自定义或扩展其他类型的接收服务，例如Kafka。要实现这个功能，你需要遵循以下步骤：

1. 实现Message接口。Message接口是一个用来抽象不同输入源数据的接口，它定义了一些方法来获取或设置消息的内容、头部、来源、参数、状态码等。你需要为你的接收服务类型实现这个接口，使得你的消息类型可以和endpoint包中的其他类型进行交互。
2. 实现EndPoint接口。EndPoint接口是一个用来定义不同接收服务类型的接口，它定义了一些方法来启动、停止、添加路由和拦截器等。你需要为你的接收服务类型实现这个接口，使得你的服务类型可以和endpoint包中的其他类型进行交互。
3. 注册Executor类型。Executor接口是一个用来定义不同输出端执行器的接口，它定义了一些方法来初始化、执行、获取路径等。你可以为你的输出端组件实现这个接口，并在DefaultExecutorFactory中注册你的Executor类型，使得你的组件可以被endpoint包中的其他类型调用。

以上就是扩展endpoint包的基本步骤，你可以参考endpoint包中已有的[Rest](rest/rest.go)和[Mqtt](mqtt/mqtt.go)类型的实现来编写你自己的代码。