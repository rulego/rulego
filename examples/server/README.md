# server

English| [中文](README_ZH.md)

This example project demonstrates how to use RuleGo as an independently running rule engine service. This project also serves as a scaffold for developing RuleGo applications. You can develop further based on this project, or you can directly download the executable [releases](https://github.com/rulego/rulego/releases).

Here is the translation:

Front-end online debugging interface: [example.rulego.cc](https://example.rulego.cc/).

Additionally, the rule chain editor tool: [RuleGo-Editor](https://editor.rulego.cc/), configure the project's HTTP API for rule chain management and debugging.

The following features are provided:

* Execute the rule chain and get the execution result API.
* Report data to the rule chain API, without focusing on the execution result.
* Create rule chain API.
* Update rule chain API.
* Get node debugging log API.
* Execute the rule chain and get the execution result API.
* Real-time push execution log.
* Save execution snapshot.
* Component list API.
* Subscribe to MQTT data and hand it over to the rule engine for processing according to the root rule chain definition.

## HTTP API

* Get all component lists
  - GET /api/v1/components

* Execute the rule chain and get the execution result API
  - POST /api/v1/rule/:chainId/execute/:msgType
  - chainId: The rule chain ID that processes the data
  - msgType: Message type
  - body: Message body

* Report data to the rule chain API, without focusing on the execution result
  - POST /api/v1/rule/:chainId/notify/:msgType
  - chainId: The rule chain ID that processes the data
  - msgType: Message type
  - body: Message body

* Query rule chain
  - GET /api/v1/rule/{chainId}/{nodeId}
  - chainId: Rule chain ID
  - nodeId: If empty, query the rule chain definition; otherwise, query the specified node ID in the rule chain

* Save or update rule chain
  - POST /api/v1/rule/{chainId}/{nodeId}
  - chainId: Rule chain ID
  - nodeId: If empty, update the rule chain definition; otherwise, update the specified node ID in the rule chain
  - body: Update content

* Save rule chain Configuration
  - POST /api/v1/rule/:chainId/saveConfig/:varType
  - chainId: Rule chain ID
  - varType: vars/secrets
  - body: Configuration content

* Get node debugging log API
  - Get /api/v1/event/debug?&chainId={chainId}&nodeId={nodeId}
  - chainId: Rule chain ID
  - nodeId: Node ID

  When the node's debugMode is turned on, debugging logs will be recorded. Currently, this interface's logs are stored in memory, with each node saving the latest 40 entries. If historical data is needed, please implement an interface to store it in the database.

## server compilation

To save the size of the compiled file, the extension component [rulego-components](https://github.com/rulego/rulego-components) is not included by default. Compile with the default setting:

```shell
cd cmd/server
go build .
```

If you need to include the extension component [rulego-components](https://github.com/rulego/rulego-components), compile with the `with_extend` tag:

```shell
cd cmd/server
go build -tags with_extend .
```
Other extension component library tags:
- To register the extension component [rulego-components](https://github.com/rulego/rulego-components), compile with the `with_extend` tag.
- To register the AI extension component [rulego-components-ai](https://github.com/rulego/rulego-components-ai), compile with the `with_ai` tag.
- To register the CI/CD extension component [rulego-components-ci](https://github.com/rulego/rulego-components-ci), compile with the `with_ci` tag.
- To register the IoT extension component [rulego-components-iot](https://github.com/rulego/rulego-components-iot), compile with the `with_iot` tag.

If you need to include multiple extension component libraries at the same time, you can compile with the `go build -tags "with_extend,with_ai,with_ci,with_iot" .` tag.

## server startup

```shell
./server -c="./config.conf"
```

Start in the background

```shell
nohup ./server -c="./config.conf" >> console.log &
```

## Configuration file parameters
```ini
# Data directory
data_dir = ./data
# cmd component command whitelist
cmd_white_list = cp,scp,mvn,npm,yarn,git,make,cmake,docker,kubectl,helm,ansible,puppet,pytest,python,python3,pip,go,java,dotnet,gcc,g++,ctest
# Whether to load Lua third-party libraries
load_lua_libs = true
# http server
server = :9090
# Default user
default_username = admin
# Whether to print node execution logs to the log file
debug = true
# Maximum node log size, default 40
max_node_log_size =40
# Node pool file
node_pool_file=./node_pool.json

# mqtt configuration
[mqtt]
# Whether to enable mqtt
enabled = false
# mqtt server
server = 127.0.0.1:1883
# Subscription topics, separated by `,`. Default: #
topics = `#`
# Which rule chain to handle the subscribed data
to_chain_id = chain_call_rest_api

# Global custom configuration, components can take values through the ${global.xxx}
[global]
# example
sqlDriver = mysql
sqlDsn = root:root@tcp(127.0.0.1:3306)/test
```