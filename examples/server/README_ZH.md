# server

------
本包提供的API服务暂没实现`RuleGo`所有特性，只为验证`RuleGo`部分特性使用。
提供以下功能：
* 提供通过HTTP方式上报数据，并根据根规则链定义交给规则引擎处理。
* 提供通过HTTP方式管理根规则链定义
* 提供MQTT 客户端，指定订阅指定主题数据，并根据根规则链定义交给规则引擎处理。

限制:只允许初始化一个规则引擎实例。

## HTTP API

* 上报数据API
  - POST /api/v1/msg/{msgType}
  - msgType：消息类型    
  - body:消息体

* 查询规则链
  - GET /api/v1/rule/{nodeId}
  - nodeId:空则查询根规则链定义，否则查询根规则链指定节点ID节点定义

* 更新规则链
  - PUT /api/v1/rule/{nodeId}
  - nodeId:空则更新根规则链定义，否则更新根规则链指定节点ID节点定义
  - body:更新内容

## MQTT客户端订阅数据

默认订阅所有主题数据，往该主题发布的数据都会根据根规则链定义交给规则引擎处理。
可以通过`-topics`修改订阅主题，多个以`,`号隔开   

## server编译

go build .

## server启动

```shell
./server -server="127.0.0.1:1883" -rule_file="/home/pi/rulego/tests/chain_call_rest_api.json"
```

或者后台启动
```shell
nohup ./server -server="127.0.0.1:1883" -rule_file="/home/pi/rulego/tests/chain_call_rest_api.json" >> console.log &
```

启动参数    
server: 连接mqtt broker       
username：连接mqtt broker 用户名    
password：连接mqtt broker 密码    
topics：连接mqtt broker 订阅消息主题，多个主题与`,`号隔开    
rule_file: 根规则链    
port: http服务器端口  
log_file: 日志存储文件路径     
