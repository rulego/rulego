# server

------

This sample project demonstrates how to use RuleGo as a standalone rule chain engine service.

The rule chain editor [RuleGoEditor](https://editor.rulego.cc/) has implemented all the interfaces of this project, and provides a visual configuration interface. After configuring the RuleGo backend HTTP API, you can manage and debug the rule chains.

The following features are provided:
* Report data API, and pass it to the rule engine according to the rule chain definition.
* Create rule chain API.
* Update rule chain API.
* Get node debug log API
* Component list API
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
  - nodeId: Specified node ID

When the node `debugMode` is `true`, debug logs will be recorded. Currently, this interface logs are stored in memory, and each node saves the latest 40. If you need to get historical data, please implement the interface to store it in the database.

## MQTT client subscription data

Subscribe to all topic data by default, and the data published to this topic will be passed to the rule engine according to the `default` rule chain definition.
You can modify the subscription topic through `-topics`, separated by `,`

## server compilation

go build .

## server startup

```shell
./server -rule_file="/home/pi/rulego/rules"
```

Or start in the background
```shell
nohup ./server -rule_file="/home/pi/rulego/rules" >> console.log &
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