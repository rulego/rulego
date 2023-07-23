#server

------
The API service provided by this package does not implement all the features of `RuleGo`, only for verifying the functionality of `RuleGo`.
It provides the following features:
* Provide data reporting via HTTP, and hand it over to the rule engine for processing according to the root rule chain definition.
* Provide HTTP management of root rule chain definition
* Provide MQTT client, specify subscription to specific topic data, and hand it over to the rule engine for processing according to the root rule chain definition.

Limitation: Only one rule engine instance is allowed to be initialized.

##HTTP API

* Data reporting API
  - POST /api/v1/msg/{msgType}
  - msgType: message type
  - body: message body

* Query rule chain
  - GET /api/v1/rule/{nodeId}
  - nodeId: empty to query root rule chain definition, otherwise query node definition of specified node ID in root rule chain

* Update rule chain
  - PUT /api/v1/rule/{nodeId}
  - nodeId: empty to update root rule chain definition, otherwise update node definition of specified node ID in root rule chain
  - body: update content

##MQTT client subscription data

Subscribe to all topic data by default, and the data published to this topic will be handed over to the rule engine for processing according to the root rule chain definition.
You can modify the subscription topic by `-topics`, multiple topics are separated by `,`

##server compilation

go build .

##server startup

```shell
./server -server="127.0.0.1:1883" -rulefile="/home/pi/rulego/tests/chain_call_rest_api.json"
```

Or start in the background
```shell
nohup ./server -server="127.0.0.1:1883" -rulefile="/home/pi/rulego/tests/chain_call_rest_api.json" >> console.log &
```

Startup parameters    
server: Connect to mqtt broker       
username：Connect to mqtt broker username    
password：Connect to mqtt broker password    
topics：Connect to mqtt broker subscription message topic, multiple topics are separated by `,`    
rulefile: Root rule chain    
port: http server port    
logfile: Log storage file path

#types

------
Shared interfaces and structures for rule engine and components. Components implemented using go plugins can only reference this package file, preventing rule engine code changes
causing plugins to recompile.