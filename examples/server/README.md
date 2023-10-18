# server

------
Provide the following functions:
* Provide data reporting through HTTP, and hand it over to the rule engine for processing according to the rule chain definition.
* Provide HTTP management of the root rule chain definition.
* Provide MQTT client, subscribe to specified topic data, and hand it over to the rule engine for processing according to the root rule chain definition.


## HTTP API

* Get a list of all components
  - GET /api/v1/components

* Data reporting API
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

## MQTT client subscription data

Subscribe to all topic data by default, and the data published to this topic will be handed over to the rule engine for processing according to the `default` rule chain definition.
You can modify the subscription topic by `-topics`, separated by `,` for multiple topics

## server compilation

go build .

## server startup

```shell
./server -server="127.0.0.1:1883" -rule_file="/home/pi/rulego/rules"
```

Or start in the background
```shell
nohup ./server -server="127.0.0.1:1883" -rule_file="/home/pi/rulego/rules" >> console.log &
```

Startup parameters    
server: Connect to mqtt broker       
username：Connect to mqtt broker username    
password：Connect to mqtt broker password    
topics：Connect to mqtt broker subscription message topic, multiple topics separated by `,`    
rule_file: Rule chain folder    
port: http server port  
log_file: Log storage file path