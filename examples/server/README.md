# server

English| [中文](README_ZH.md)

This sample project demonstrates how to use RuleGo as a standalone rule engine service.You can do secondary development based on this project, or you can directly download the executable [binary file](https://github.com/rulego/rulego/releases) .

If you need visualization, you can use this tool: [RuleGoEditor](https://editor.rulego.cc/), configure the HTTP API of this project, and manage and debug the rule chain.

The following features are provided:
* Report data API, and pass it to the rule engine according to the rule chain definition.
* Create rule chain API.
* Update rule chain API.
* Get node debug log API.
* Component list API.
* Subscribe to MQTT data, and pass it to the rule engine according to the root rule chain definition.

## HTTP API

* Get all component list
  - GET /api/v1/components

* Report data API
  - POST /api/v1/msg/{chainId}/{msgType}
  - chainId: The rule chain ID that processes the data
  - msgType: Message type
  - body: Message body

* Query rule chain
  - GET /api/v1/rule/{chainId}/{nodeId}
  - chainId: Rule chain ID
  - nodeId: Empty to query the rule chain definition, otherwise query the node definition of the specified node ID in the rule chain

* Save or update rule chain
  - POST /api/v1/rule/{chainId}/{nodeId}
  - chainId: Rule chain ID
  - nodeId: Empty to update the rule chain definition, otherwise update the node definition of the specified node ID in the rule chain
  - body: Update content

* Get node debug log API
  - Get /api/v1/event/debug?&chainId={chainId}&nodeId={nodeId}
  - chainId: Rule chain ID
  - nodeId:  Node ID

When the node `debugMode` is `true`, debug logs will be recorded. Currently, this interface logs are stored in memory, and each node saves the latest 40. If you need to get historical data, please implement the interface to store it in the database.

## MQTT client subscription data

Subscribe to all topic data by default, and the data published to this topic will be passed to the rule engine according to the `default` rule chain definition.
You can modify the subscription topic through `-topics`, separated by `,`

## Server compilation

To save the size of the compiled file, the extension component [rulego-components](https://github.com/rulego/rulego-components) is not imported by default. The default compilation is:

```shell
go build .
```

If you need to import the extension component [rulego-components](https://github.com/rulego/rulego-components), use the `with_extend` tag to compile:

```shell
go build -tags with_extend .
```

## server startup

```shell
./server -rule_file="./rules/"
```

Or start in the background
```shell
nohup ./server -rule_file="./rules/" >> console.log &
```

Startup parameters
- rule_file: Rule chain folder. default: ./rules/
- port: http server port. default: 9090
- log_file: Log storage file path. default: print to console
- debug: Whether to print node debug logs to log file
- mqtt: Whether to enable mqtt subscription. default: false
- server: Connect to mqtt broker. default: 127.0.0.1:1883
- username: Connect to mqtt broker username
- password: Connect to mqtt broker password
- topics: Connect to mqtt broker subscription message topic, multiple topics separated by `,`. default: #