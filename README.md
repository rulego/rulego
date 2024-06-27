# RuleGo

[![GoDoc](https://pkg.go.dev/badge/github.com/rulego/rulego)](https://pkg.go.dev/github.com/rulego/rulego) 
[![Go Report](https://goreportcard.com/badge/github.com/rulego/rulego)](https://goreportcard.com/report/github.com/rulego/rulego)
[![codecov](https://codecov.io/gh/rulego/rulego/graph/badge.svg?token=G6XCGY7KVN)](https://codecov.io/gh/rulego/rulego)
[![test](https://github.com/rulego/rulego/workflows/test/badge.svg)](https://github.com/rulego/rulego/actions/workflows/test.yml)
[![build](https://github.com/rulego/rulego/workflows/build/badge.svg)](https://github.com/rulego/rulego/actions/workflows/build.yml)
[![build](https://github.com/rulego/rulego/workflows/build/badge.svg)](https://github.com/rulego/rulego/actions/workflows/build.yml)
[![QQ-720103251](https://img.shields.io/badge/QQ-720103251-orange)](https://qm.qq.com/q/8RDaYcOry8)

English| [中文](README_ZH.md)

<img src="doc/imgs/logo.png" width="100">   

RuleGo is a lightweight, high-performance, embedded, orchestrable component-based rule engine built on the Go language. It supports heterogeneous system data integration and can aggregate, distribute, filter, transform, enrich, and perform various actions on input messages.

<h3>Your encouragement is our motivation to move forward. If this project is helpful to you, please give it a Star in the top right corner.</h3>

## Documentation

The official documentation is hosted at: [rulego.cc](https://rulego.cc/en)

## Features

* **Lightweight:** No external middleware dependencies, efficient data processing and linkage on low-cost devices, suitable for IoT edge computing.
* **High Performance:** Thanks to Go's high-performance characteristics, RuleGo also employs technologies such as coroutine pools and object pools.
* **Embedded:** Supports embedding RuleGo into existing projects, non-invasively utilizing its features.
* **Componentized:** All business logic is component-based, allowing flexible configuration and reuse.
* **Rule Chains:** Flexibly combine and reuse different components to achieve highly customized and scalable business processes.
* **Workflow Orchestration:** Supports dynamic orchestration of rule chain components, replacing or adding business logic without restarting the application.
* **Easy Extension:** Provides rich and flexible extension interfaces, making it easy to implement custom components or introduce third-party components.
* **Dynamic Loading:** Supports dynamic loading of components and extensions through Go plugins.
* **Nested Rule Chains:** Supports nesting of sub-rule chains to reuse processes.
* **Built-in Components:** Includes a large number of components such as `Message Type Switch`, `JavaScript Switch`, `JavaScript Filter`, `JavaScript Transformer`, `HTTP Push`, `MQTT Push`, `Send Email`, `Log Recording`, etc. Other components can be extended as needed.
* **Context Isolation Mechanism:** Reliable context isolation mechanism, no need to worry about data streaming in high concurrency situations.
* **AOP Mechanism:** Allows adding extra behavior to the execution of rule chains or directly replacing the original logic of rule chains or nodes without modifying their original logic.
* **Data Integration:** Allows dynamic configuration of Endpoints, such as `HTTP Endpoint`, `MQTT Endpoint`, `TCP/UDP Endpoint`, `UDP Endpoint`, `Kafka Endpoint`, `Schedule Endpoint`, etc.

## Use Cases

RuleGo is an orchestrable rule engine that excels at decoupling your systems.

- If your system's business is complex and the code is bloated
- If your business scenarios are highly customized or frequently changing
- If your system needs to interface with a large number of third-party applications or protocols
- Or if you need an end-to-end IoT solution
- Or if you need centralized processing of heterogeneous system data
- Or if you want to try hot deployment in the Go language...
  Then the RuleGo framework will be a very good solution.

#### Typical Use Cases

* **Edge Computing:** Deploy RuleGo on edge servers to preprocess data, filter, aggregate, or compute before reporting to the cloud. Data processing rules and distribution rules can be dynamically configured and modified through rule chains without restarting the system.
* **IoT:** Collect device data reports, make rule judgments through rule chains, and trigger one or more actions, such as sending emails, alarms, and linking with other devices or systems.
* **Data Distribution:** Distribute data to different systems using HTTP, MQTT, or gRPC based on different message types.
* **Application Integration:** Use RuleGo as glue to connect various systems or protocols, such as SSH, webhook, Kafka, message queues, databases, ChatGPT, third-party application systems.
* **Centralized Processing of Heterogeneous System Data:** Receive data from different sources (such as MQTT, HTTP, WS, TCP/UDP, etc.), then filter, format convert, and distribute to databases, business systems, or dashboards.
* **Highly Customized Business:** Decouple highly customized or frequently changing business and manage it with RuleGo rule chains. Business requirements change without needing to restart the main program.
* **Complex Business Orchestration:** Encapsulate business into custom components, orchestrate and drive these custom components through RuleGo, and support dynamic adjustment and replacement of business logic.
* **Microservice Orchestration:** Orchestrate and drive microservices through RuleGo, or dynamically call third-party services to process business and return results.
* **Decoupling of Business Code and Logic:** For example, user points calculation systems, risk control systems.
* **Automation:** For example, CI/CD systems, process automation systems, marketing automation systems.
* **Low Code:** For example, low-code platforms, iPaaS systems, ETL, LangFlow-like systems (interfacing with large models to extract user intent, then triggering rule chains to interact with other systems or process business).

## Architecture Diagram

<img src="doc/imgs/architecture.png" width="100%">
<p align="center">RuleGo Architecture Diagram</p>

## Rule Chain Running Example Diagram

  <img src="doc/imgs/rulechain/img_2.png" style="height:40%;width:70%;"/>

[More Running Modes](https://rulego.cc/en/pages/6f46fc/#%E8%A7%84%E5%88%99%E9%93%BE%E6%94%AF%E6%8C%81%E7%9A%84%E8%BF%90%E8%A1%8C%E6%96%B9%E5%BC%8F)

## Installation

Install RuleGo using the `go get` command:

```bash
go get github.com/rulego/rulego

or
go get gitee.com/rulego/rulego
```

## Usage

RuleGo is extremely simple and lightweight to use. Just follow these 3 steps:

1. Define rule chains using JSON:
   [Rule Chain DSL](https://rulego.cc/en/pages/10e1c0/)

2. Import the RuleGo package and use the rule chain definition to create a rule engine instance:

```go
import "github.com/rulego/rulego"

// Create a rule engine instance using the rule chain definition
ruleEngine, err := rulego.New("rule01", []byte(ruleFile))
```

3. Hand over the message payload, message type, and message metadata to the rule engine instance for processing, and then the rule engine will process the message according to the rule chain's definition:

```go
// Define message metadata
metaData := types.NewMetadata()
metaData.PutValue("productType", "test01")
// Define message payload and message type
msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")

// Hand over the message to the rule engine for processing
ruleEngine.OnMsg(msg)
```

### Rule Engine Management API

Dynamically update rule chains

```go
// Update the root rule chain
err := ruleEngine.ReloadSelf([]byte(ruleFile))
// Update a node under the rule chain
ruleEngine.ReloadChild("rule_chain_test", nodeFile)
// Get the rule chain definition
ruleEngine.DSL()
```

Rule Engine Instance Management:

```go
// Load all rule chain definitions in a folder into the rule engine pool
rulego.Load("/rules", rulego.WithConfig(config))
// Get an already created rule engine instance by ID
ruleEngine, ok := rulego.Get("rule01")
// Delete an already created rule engine instance
rulego.Del("rule01")
```

Configuration:

See [Documentation](https://rulego.cc/en/pages/d59341/)

```go
// Create a default configuration
config := rulego.NewConfig()
// Debug node callback, the node configuration must be set to debugMode:true to trigger the call
// Both node entry and exit information will call this callback function
config.OnDebug = func (chainId,flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
}
// Use the configuration
ruleEngine, err := rulego.New("rule01", []byte(ruleFile), rulego.WithConfig(config))
```
### Rule Chain Definition DSL
[Rule Chain Definition DSL](https://rulego.cc/en/pages/10e1c0/)

### Node Components
- [Standard Components](https://rulego.cc/en/pages/88fc3c/)
- [rulego-components](https://github.com/rulego/rulego-components)  [Documentation](https://rulego.cc/en/pages/d7fc43/)
- [rulego-components-ai](https://github.com/rulego/rulego-components-ai)
- [rulego-components-ci](https://github.com/rulego/rulego-components-ci)
- [rulego-components-iot](https://github.com/rulego/rulego-components-iot)
- [Custom Node Component Example](examples/custom_component) [Documentation](https://rulego.cc/en/pages/caed1b/)

## Data Integration
RuleGo provides the Endpoint module for unified data integration and processing of heterogeneous systems. For details, refer to: [Endpoint](endpoint/README.md)

### Endpoint Components
- [Endpoint Components](https://rulego.cc/en/pages/691dd3/)
- [Endpoint DSL](https://rulego.cc/en/pages/390ad7/)

## Performance

`RuleGo` almost does not increase system overhead, resource consumption is extremely low, especially suitable for running on edge servers.
In addition, RuleGo uses a directed acyclic graph to represent the rule chain, and each input message only needs to be processed along the path in the graph, without matching all the rules, This greatly improves the efficiency and speed of message processing, and also saves resources and time. The routing algorithm can achieve: no matter how many nodes the rule chain has, it will not affect the node routing performance.

Performance test cases:
```
Machine: Raspberry Pi 2 (900MHz Cortex-A7*4,1GB LPDDR2)  
Data size: 260B   
Rule chain: JS script filtering->JS complex transformation->HTTP push   
Test results: 100 concurrent and 500 concurrent, memory consumption does not change much around 19M
```

[More performance test cases](https://rulego.cc/en/pages/f60381/)

## Ecosystem

- [RuleGo-Editor](https://editor.rulego.cc/) :Rule chain visual editor
- [RuleGo-CI](http://8.134.32.225:9090/ui/) :RuleGo CI/CD APP
- [rulego-components](https://github.com/rulego/rulego-components) :Extension component library:
- [examples/server](examples/server): A standalone example project
- [examples](examples): More examples

## Contribution

Any form of contribution is welcome, including submitting issues, suggestions, documentation, tests or code. Please follow these steps:

* Clone the project repository to your local machine
* Create a new branch and make modifications
* Submit a merge request to the main branch
* Wait for review and feedback

## License

`RuleGo` uses Apache 2.0 license, please refer to [LICENSE](LICENSE) file for details.