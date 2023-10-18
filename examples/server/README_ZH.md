# server

------
提供以下功能：
* 提供通过HTTP方式上报数据，并根据规则链定义交给规则引擎处理。
* 提供通过HTTP方式管理根规则链定义。
* 提供MQTT 客户端，指定订阅指定主题数据，并根据根规则链定义交给规则引擎处理。


## HTTP API

* 获取所有组件列表
  - GET /api/v1/components
  
* 上报数据API
  - POST /api/v1/msg/{chainId}/{msgType}
  - chainId：处理数据的规则链ID
  - msgType：消息类型    
  - body：消息体

* 查询规则链
  - GET /api/v1/rule/{chainId}/{nodeId}
  - chainId：规则链ID
  - nodeId:空则查询规则链定义，否则查询规则链指定节点ID节点定义

* 保存或更新规则链
  - POST /api/v1/rule/{chainId}/{nodeId}
  - chainId：规则链ID
  - nodeId：空则更新规则链定义，否则更新规则链指定节点ID节点定义
  - body：更新内容

## MQTT客户端订阅数据

默认订阅所有主题数据，往该主题发布的数据都会根据`default`规则链定义交给规则引擎处理。
可以通过`-topics`修改订阅主题，多个以`,`号隔开   

## server编译

go build .

## server启动

```shell
./server -server="127.0.0.1:1883" -rule_file="/home/pi/rulego/rules"
```

或者后台启动
```shell
nohup ./server -server="127.0.0.1:1883" -rule_file="/home/pi/rulego/rules" >> console.log &
```

启动参数    
server: 连接mqtt broker       
username：连接mqtt broker 用户名    
password：连接mqtt broker 密码    
topics：连接mqtt broker 订阅消息主题，多个主题与`,`号隔开    
rule_file: 规则链文件夹    
port: http服务器端口  
log_file: 日志存储文件路径     
