# RuleGo

English| [中文](README_ZH.md)

<img src="doc/imgs/logo.png" width="100">   

`RuleGo` is a lightweight, high-performance, embedded orchestration rule engine based on `Go` language. It is also a flexible and highly customizable event processing framework. It can aggregate, distribute, filter, transform, enrich and execute various actions on input messages.

This project is largely inspired by [thingsboard](https://github.com/thingsboard/thingsboard) .

## Features

* Development language: Go 1.18
* Lightweight: No external middleware dependencies, can efficiently process and link data on low-cost devices, suitable for IoT edge computing.
* High performance: Thanks to the high-performance characteristics of `Go`, in addition, `RuleGo` adopts technologies such as coroutine pool and object pool. For 10W data processing `JS script filtering->JS script data processing->HTTP push`, the average time is 9 seconds.
* Embedded: Support embedding `RuleGo` into existing projects, non-intrusively utilizing its features.
* Componentized: All business logic is componentized and can be flexibly configured and reused.
* Rule chain: You can flexibly combine and reuse different components to achieve highly customizable and scalable business processes.
* Process orchestration: Support dynamic orchestration of rule chains, you can encapsulate your business into `RuleGo` components, and then achieve your highly changing business needs by building blocks.
* Easy to extend: Provide rich and flexible extension interfaces and hooks, such as: custom components, component registration management, rule chain DSL parser, coroutine pool, rule node message inflow/outflow callback, rule chain processing end callback.
* Dynamic loading: Support dynamic loading of components and extension components through `Go plugin`.
* Built-in common components: `Message type Switch`,`JavaScript Switch`,`JavaScript filter`,`JavaScript converter`,`HTTP push`,`MQTT push`,`Send email`,`Log record` and other components. You can extend other components by yourself.
* Context isolation mechanism: Reliable context isolation mechanism, no need to worry about data streaming in high concurrency situations.


## Use Cases

`RuleGo` is a rule engine based on orchestration, which is best at decoupling your system.

- If your system is complex and bloated with code
- If your business scenario is highly customized or frequently changed
- Or you need an end-to-end IoT solution
- Or you need to process data from heterogeneous systems centrally
- Or you want to try hot deployment in `Go` language...
  Then `RuleGo` framework will be a very good solution.

#### Typical use cases

* **Edge computing:** For example: You can deploy `RuleGo` on the edge server, preprocess, filter, aggregate or calculate the data before reporting it to the cloud. The data processing rules and distribution rules can be dynamically configured and modified through the rule chain without restarting the system.
* **Internet of Things:** For example: Collect device data reporting, and after the rule judgment of the rule chain, trigger one or more actions, such as: send email, send alarm, and link with other devices or systems.
* **Data distribution:** For example: You can distribute data to different systems according to different message types, such as HTTP, MQTT or gRPC.
* **Application integration:** For example: kafka, message queue, third-party system integration.
* **Data processing from heterogeneous systems:** For example: Receive data from different data sources (such as MQTT, HTTP, etc.), and then filter, format conversion, and then distribute to databases, business systems or dashboards.
* **Highly customized business:** For example: Decouple highly customized or frequently changed business and hand it over to `RuleGo` rule chain for management. Business requirements change without restarting the main program.
* **Complex business orchestration:** For example: Encapsulate the business into custom components, and use `RuleGo` to orchestrate and drive these custom components, and support dynamic adjustment.
* **Microservice orchestration:** For example: Use `RuleGo` to orchestrate and drive microservices, or dynamically call third-party services to process business and return results.
* **Business code and business logic decoupling:** For example: User points calculation system, risk control system.
* **Flexible configuration and highly customized event processing framework:** For example: Asynchronously or synchronously process different message types.


## Installation

Use the `go get` command to install `RuleGo`:

```bash
go get github.com/rulego/rulego
```

## Usage

Use Json format to define the rule chain DSL:     
The following example defines 3 rule nodes, and the rule chain logic is as follows: (For more examples, refer to [testcases/](testcases))      

<img src="doc/imgs/rulechain/img_1.png" style="height:50%;width:80%;">

```json
{
  "ruleChain": {
    "name": "Test rule chain",
    "root": true,
    "debugMode": false
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "Filtering Data",
        "debugMode": true,
        "configuration": {
          "jsScript": "return msg!='bb';"
        }
      },
      {
        "id": "s2",
        "type": "jsTransform",
        "name": "Transform Data",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['test']='test02';\n metadata['index']=50;\n msgType='TEST_MSG_TYPE2';\n var msg2=JSON.parse(msg);\n msg2['aa']=66;\n return {'msg':msg2,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s3",
        "type": "restApiCall",
        "name": "Call Rest Api Push Data",
        "debugMode": true,
        "configuration": {
          "restEndpointUrlPattern": "http://192.168.216.21:9099/api/socket/msg",
          "requestMethod": "POST",
          "maxParallelRequestsCount": 200
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "True"
      },
      {
        "fromId": "s2",
        "toId": "s3",
        "type": "Success"
      }
    ],
    "ruleChainConnections": null
  }
}
```
Description:
- `ruleChain`: The root object of the rule chain definition, which contains the following fields:
  - `name`: The name of the rule chain, which can be any string.
  - `root`: A boolean value indicating whether this rule chain is the root rule chain or a sub-rule chain. Only one root rule chain is allowed per rule engine instance.
  - `debugMode`: A boolean value indicating whether this rule chain is in debug mode or not. If true, the debug callback function will be triggered when the rule chain processes messages.
- `metadata`: An object that contains the information of the nodes and connections in the rule chain, which has the following fields:
  - `nodes`: An array of objects, each representing a rule node in the rule chain. Each node object has the following fields:
    - `id`: A unique identifier for the node, which can be any string.
    - `type`: The type of the node, which determines the logic and behavior of the node. It should match one of the registered node types in the rule engine.
    - `name`: The name of the node, which can be any string.
    - `debugMode`: A boolean value indicating whether this node is in debug mode or not. If true, the debug callback function will be triggered when the node processes messages.
    - `configuration`: An object that contains the configuration parameters for the node, which vary depending on the node type. For example, a JS filter node may have a `jsScript` field that defines the filtering logic, while a REST API call node may have a `restEndpointUrlPattern` field that defines the URL to call.
  - `connections`: An array of objects, each representing a connection between two nodes in the rule chain. Each connection object has the following fields:
    - `fromId`: The id of the source node of the connection, which should match one of the node ids in the nodes array.
    - `toId`: The id of the destination node of the connection, which should match one of the node ids in the nodes array.
    - `type`: The type of the connection, which determines when and how messages are sent from one node to another. It should match one of the supported connection types by the source node type. For example, a JS filter node may support two connection types: "True" and "False", indicating whether messages pass or fail the filter condition.
  - `ruleChainConnections`: An array of objects, each representing a connection between a node and a sub-rule chain in the rule chain. Each rule chain connection object has the following fields:
    - `fromId`: The id of the source node of the connection, which should match one of the node ids in the nodes array.
    - `toId`: The id of the destination sub-rule chain of the connection, which should match one of the registered sub-rule chains in the rule engine.
    - `type`: The type of the connection, which determines when and how messages are sent from one node to another. It should match one of the supported connection types by the source node type.

Import the `RuleGo` package and create a rule engine instance:

```go
import "github.com/rulego/rulego"

//Create a rule engine instance, each rule engine instance has only one root rule chain
ruleEngine, err := rulego.New("rule01", []byte(ruleFile))
```

Give the message, message type, and message metadata to the rule engine instance for processing:

```go
//Define message metadata
metaData := types.NewMetadata()
metaData.PutValue("productType", "test01")
//Define message and message type
msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")

//Give the message to the rule engine for processing
ruleEngine.OnMsg(msg)

//with end callback call
ruleEngine.OnMsgWithOptions(msg,types.WithEndFunc(func(msg types.RuleMsg, err error) {
//Rule chain asynchronous callback result 
//Note: If the rule chain has multiple branch endpoints, it will be called multiple times
}))

//whit context.Context call 
//Used for sharing data between different component instances
ruleEngine.OnMsgWithOptions(msg,types.WithContext(context.WithValue(context.Background(), "shareKey", "shareValue")))

```

Add sub-rule chains:

```go
//Create a rule engine instance and sub-rule chains together
ruleEngine, err := rulego.New("rule01", []byte(ruleFile), rulego.WithAddSubChain("rule_chain_test", subRuleFile))
//Or by creating or updating
ruleEngine.ReloadChild(types.EmptyRuleNodeId, types.RuleNodeId{Id: "rule_chain_test", Type: types.CHAIN}, subRuleFile)

```

Update rule chain

```go
//Update root rule chain
err := ruleEngine.ReloadSelf([]byte(ruleFile))
//Update sub-rule chain
ruleEngine.ReloadChild(types.EmptyRuleNodeId, types.RuleNodeId{Id: "rule_chain_test", Type: types.CHAIN}, subRuleFile)
//Update a node of the rule chain, see the following method for details
ruleEngine.ReloadChild(chainId types.RuleNodeId, ruleNodeId types.RuleNodeId, dls []byte)

```

Rule engine instance management:

```go
//Get the created rule engine instance by ID
ruleEngine, ok := rulego.Get("rule01")
//Delete the created rule engine instance
rulego.Del("rule01")
```

### Configuration

See `types.Config` for details

```go
//Create a default configuration
config := rulego.NewConfig()
//Debug node callback, the node configuration must be configured with debugMode:true to trigger the call
//Both node input and output information will call this callback function
config.OnDebug = func (flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
}
//Global rule chain end callback
//If you just want to call for a single message, use the ruleEngine.OnMsgWithOptions method
//Note: If the rule chain has multiple branch endpoints, it will be called multiple times
config.OnEnd = func (msg types.RuleMsg, err error) {
}
//Configuration usage
ruleEngine, err := rulego.New("rule01", []byte(ruleFile), rulego.WithConfig(config))
```

## About rule chain

### Rule node

Rule node is the basic component of the rule engine, it processes a single incoming message at a time and generates one or more outgoing messages. Rule node is the main logic unit of the rule engine. Rule nodes can filter, enrich, transform incoming messages, execute actions or communicate with external systems. You can easily encapsulate your business into `RuleGo`
node components, and then flexibly configure and reuse them, like building blocks to achieve your business needs. Define `RuleGo` custom node component method:

* Method 1: Implement the `types.Node` interface, refer to the examples in [components](components)
  For example:

```go
//Define Node component
//UpperNode A plugin that converts the message data to uppercase
type UpperNode struct{}

func (n *UpperNode) Type() string {
return "test/upper"
}
func (n *UpperNode) New() types.Node {
return &UpperNode{}
}
func (n *UpperNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
// Do some initialization work
return nil
}
//Process message
func (n *UpperNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
msg.Data = strings.ToUpper(msg.Data)
// Send the modified message to the next node
ctx.TellSuccess(msg)
return nil
}

func (n *UpperNode) Destroy() {
// Do some cleanup work
}
//Register to rulego default registrar
rulego.Registry.Register(&MyNode{})
```

* Method 2: Use `go plugin` to implement the interface `types.PluginRegistry` interface. And export the variable name: `Plugins`, refer to [testcases/plugin](testcases/plugin/)
  For example:

```go
// plugin entry point
var Plugins MyPlugins

type MyPlugins struct{}

func (p *MyPlugins) Init() error {
return nil
}
func (p *MyPlugins) Components() []types.Node {
//A plugin can provide multiple components
return []types.Node{&UpperNode{}, &TimeNode{}, &FilterNode{}}
}
//go build -buildmode=plugin -o plugin.so plugin.go # Compile the plugin and generate the plugin.so file, need to compile in mac or linux environment
//Register to rulego default registrar
rulego.Registry.RegisterPlugin("test", "./plugin.so")
```

Then use your component in the rule chain DSL file

```json
{
  "ruleChain": {
    "name": "Test rule chain",
    "root": true,
    "debugMode": false
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "test/upper",
        "name": "Name",
        "debugMode": true,
        "configuration": {
          "field1": "Configuration parameters defined by the component",
          "....": "..."
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "Connect to the next component ID",
        "type": "The connection relationship with the component"
      }
    ],
    "ruleChainConnections": null
  }
}
```

`RuleGo` built-in some common components are divided into the following types:
* Filter component: Filter messages.
* Transformation component: Transform and enhance messages.
* Action component: Perform some actions or link with external systems.

### Rule chain

Rule chain is a logical group of `rule nodes` and their `relationTypes`. It receives outbound messages from nodes and sends them to the next node or nodes through a specified `relationship`. Here are some common rule chain examples:

#### Sequential execution:
  <img src="doc/imgs/rulechain/img_1.png" style="height:50%;width:80%;">

--------
#### Asynchronous + sequential execution:
  <img src="doc/imgs/rulechain/img_2.png" style="height:50%;width:80%;">

--------
#### Using sub-rule chain method:
  <img src="doc/imgs/rulechain/img_3.png" style="height:50%;width:80%;">

--------
#### Some complex examples:
  <img src="doc/imgs/rulechain/img_4.png" style="height:50%;width:80%;">

--------

## Data Integration

`RuleGo` provides `Endpoint` module for unified data integration and processing of heterogeneous systems.       
For more details, please refer to: [Endpoint](endpoint/README.md)

## Performance

`RuleGo` almost does not increase system overhead, resource consumption is extremely low, because it uses object coroutine pool and object pool, even higher performance than directly calling business methods, especially suitable for running on edge servers.

--------
Machine: Raspberry Pi 2 (900MHz Cortex-A7*4,1GB LPDDR2)  
Data size: 260B   
Rule chain: JS script filtering->JS complex transformation->HTTP push   
Test results: 100 concurrent and 500 concurrent, memory consumption does not change much around 19M

## Contribution

Any form of contribution is welcome, including submitting issues, suggestions, documentation, tests or code. Please follow these steps:

* Clone the project repository to your local machine
* Create a new branch and make modifications
* Submit a merge request to the main branch
* Wait for review and feedback

## License

`RuleGo` uses Apache 2.0 license, please refer to [LICENSE](LICENSE) file for details.