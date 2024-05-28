# Endpoint

English| [中文](README_ZH.md)

**Endpoint** is a module that abstracts different input source data routing, providing a **consistent** user experience for different protocols. It is an optional module of `RuleGo` that enables RuleGo to run independently and provide services.

It allows you to easily create and start different receiving services, such as http, mqtt, kafka, gRpc, websocket, schedule, tpc, udp, etc., to achieve data integration of heterogeneous systems, and then perform conversion, processing, flow, etc. operations according to different requests or messages, and finally hand them over to the rule chain or component for processing.

Additionally, it supports dynamic creation and updates through `DSL`.

<img src="../doc/imgs/endpoint/endpoint.png">
<div style="text-align: center;">Endpoint architecture diagram</div>

## Usage

1. First define the route, which provides a stream-like calling method, including the input end, processing function and output end. Different Endpoint types have **consistent** route processing

```go
router := endpoint.Registry.NewRouter().From("/api/v1/msg/").Process(func(exchange *endpoint.Exchange) bool {
//processing logic
return true
}).To("chain:default")
```
For different `Endpoint` types, the meaning of the input end `From` will be different, but it will eventually route to the router according to the `From` value:
- http/websocket endpoint: represents path routing, creating an http service according to the `From` value. For example: From("/api/v1/msg/") means creating /api/v1/msg/ http service.
- mqtt/kafka endpoint: represents the subscribed topic, subscribing to the relevant topic according to the `From` value. For example: From("/api/v1/msg/") means subscribing to the /api/v1/msg/ topic.
- schedule endpoint: represents the cron expression, creating a related timed task according to the `From` value. For example: From("*/1 * * * * *") means triggering the router every 1 second.
- tpc/udp endpoint: represents a regular expression, forwarding the message that meets the condition to the router according to the `From` value. For example: From("^{.*") means data that satisfies `{` at the beginning.

2. Then create the Endpoint service, the creation interface is also **consistent**:

```go
//For example: create http service
restEndpoint, err := endpoint.Registry.New(rest.Type, config, rest.Config{Server: ":9090",})
// or use map to set configuration
restEndpoint, err := endpoint.Registry.New(rest.Type, config, types.Configuration{"server": ":9090",})

//For example: create mqtt subscription service
mqttEndpoint, err := endpoint.Registry.New(mqtt.Type, config, mqtt.Config{Server: "127.0.0.1:1883",})
// or use map to set configuration
mqttEndpoint, err := endpoint.Registry.New(mqtt.Type, config, types.Configuration{"server": "127.0.0.1:1883",})

//For example: create ws service
wsEndpoint, err := endpoint.Registry.New(websocket.Type, config, websocket.Config{Server: ":9090"})

//For example: create tcp service
tcpEndpoint, err := endpoint.Registry.New(net.Type, config, Config{Protocol: "tcp", Server:   ":8888",})

//For example: create schedule endpoint service
scheduleEndpoint, err := endpoint.Registry.New(schedule.Type, config, nil)
```

3. Register the route to the endpoint service and start the service
```go
//http endpoint register route
_, err = restEndpoint.AddRouter(router1,"POST")
_, err = restEndpoint.AddRouter(router2,"GET")
_ = restEndpoint.Start()

//mqtt endpoint register route
_, err = mqttEndpoint.AddRouter(router1)
_, err = mqttEndpoint.AddRouter(router2)
_ = mqttEndpoint.Start()
```

4. Endpoint supports responding to the caller
```go
router5 := endpoint.Registry.NewRouter().From("/api/v1/msgToComponent2/:msgType").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
    //respond to the client
    exchange.Out.Headers().Set("Content-Type", "application/json")
    exchange.Out.SetBody([]byte("ok"))
    return true
})
//If you need to synchronize the rule chain execution result to the client, add the wait semantics
router5 := endpoint.Registry.NewRouter().From("/api/v1/msg2Chain4/:chainId").
To("chain:${chainId}").
//Must add Wait, asynchronous to synchronous, http can respond normally, if not synchronous response, do not add this sentence, will affect the throughput
Wait().
Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
  err := exchange.Out.GetError()
  if err != nil {
    //error
    exchange.Out.SetStatusCode(400)
    exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
    } else {
    //respond the processing result to the client, http endpoint must add Wait(), otherwise it cannot respond normally
    outMsg := exchange.Out.GetMsg()
    exchange.Out.Headers().Set("Content-Type", "application/json")
    exchange.Out.SetBody([]byte(outMsg.Data))
  }

  return true
}).End()
```

5. Add global interceptors to perform permission verification and other logic
```go
restEndpoint.AddInterceptors(func(exchange *endpoint.Exchange) bool {
  //permission verification logic
  return true
})
```

## Router

Refer to the [documentation](https://rulego.cc/pages/45008b/)

## Examples

The following are examples of using endpoint:
- [RestEndpoint](/examples/http_endpoint/http_endpoint.go)
- [WebsocketEndpoint](/endpoint/websocket/websocket_test.go)
- [MqttEndpoint](/endpoint/mqtt/mqtt_test.go)
- [ScheduleEndpoint](/endpoint/schedule/schedule_test.go)
- [NetEndpoint](/endpoint/net/net_test.go)
- [KafkaEndpoint](https://github.com/rulego/rulego-components/blob/main/endpoint/kafka/kafka_test.go) (extension component library)

## Extend endpoint

**Endpoint module** provides some built-in receiving service types, but you can also customize or extend other types of receiving services. To achieve this, you need to follow these steps:

1. Implement the [Message interface](/endpoint/endpoint.go#L62) . The Message interface is an interface that abstracts different input source data, and it defines some methods to get or set the message content, header, source, parameters, status code, etc. You need to implement this interface for your receiving service type, so that your message type can interact with other types in the endpoint package.
2. Implement the [Endpoint interface](/endpoint/endpoint.go#L40) . The Endpoint interface is an interface that defines different receiving service types, and it defines some methods to start, stop, add routes and interceptors, etc. You need to implement this interface for your receiving service type, so that your service type can interact with other types in the endpoint package.

The above are the basic steps to extend the endpoint package, you can refer to the existing endpoint type implementations to write your own code:
- [rest](https://github.com/rulego/rulego/tree/main/endpoint/rest/rest.go)
- [websocket](https://github.com/rulego/rulego/tree/main/endpoint/websocket/websocket.go)
- [mqtt](https://github.com/rulego/rulego/tree/main/endpoint/mqtt/mqtt.go)
- [schedule](https://github.com/rulego/rulego/tree/main/endpoint/schedule/schedule.go)
- [tcp/udp](https://github.com/rulego/rulego/tree/main/endpoint/net/net.go)
- [Kafka](https://github.com/rulego/rulego-components/blob/main/endpoint/kafka/kafka.go) (extension component library)