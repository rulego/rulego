# CHANGELOG

## [v0.16.0] 2023/10/30

- feat:提供规则链可视化编辑器RuleGo-Editor [在线使用](https://editor.rulego.cc/)
- feat:增加ssh节点组件  [文档](https://rulego.cc/pages/fa62c1/)
- feat:增加延迟节点组件 [文档](https://rulego.cc/pages/5f5612/)
- feat:增加functions节点组件 [文档](https://rulego.cc/pages/b7edde/)
- feat:dbClient节点组件支持手动导入数据库驱动，例如：TDengine
- feat:增加schedule endpoint组件 [文档](https://rulego.cc/pages/4c4e4c/)
- feat:http endpoint增加global options handler
- feat:增加作为中间件独立运行的规则引擎示例工程，并提供二进制文件 [examples/server](https://github.com/rulego/rulego/tree/main/examples/server)
- feat:endpoint.AddRouterWithParams 返回 routerId
- feat:可视化相关api返回的json，字段首字母改成小写
- feat:onDebug回调函数，可以得到规则链id
- feat:完善ctx.TellSelf逻辑
- fix:规则链JSON文件，节点Id字段改成首字母小写：id
- opt:upgraded github.com/dop251/goja v0.0.0-20230605162241-28ee0ee714f3 => v0.0.0-20231024180952-594410467bc6
- opt:组件包结构调整
- opt:dbClient节点dbType改成driverName
- opt:完善文档

## [v0.15.0] 2023/10/7

- feat:增加文档官网: [rulego.cc](https://rulego.cc/)
- feat:增加可视化相关API。[文档](https://rulego.cc/pages/cf0193/)
- feat:增加规则链全局配置Properties。[文档](https://rulego.cc/pages/d59341/#properties)
- feat:增加规则链全局配置和自定义函数到js运行时，js脚本可以调用golang自定义函数。[文档](https://rulego.cc/pages/d59341/#udf)
- feat:增加同步调用规则链方式:`OnMsgAndWait`。
- feat:http Endpoint支持把规则链处理结果响应给前端。
- feat:Endpoint模块，路由增加Wait()语义,表示同步等待规则链执行结果。
- feat:增加批量触发规则引擎实例池所有规则链处理消息方法。
- feat:DefaultRuleContext增加onAllNodeCompleted回调。
- feat:DefaultRuleContext增加parentRuleCtx,支持更加灵活的规则链嵌套。
- fix:修复log组件，metadata参数丢失问题。
- fix:examples/server getDsl响应头不是`application/json`。
- opt:所有组件`config`改成大写`Config`变成公有。
- opt:优化子规则链的调用方式。
- opt:restApiCall组件ReadTimeoutMs 参数默认设置成2000ms。
- opt:所有测试规则链json文件，添加ruleId。
- opt:优化文档。

## [v0.14.0] 2023/9/6

### 新功能

- 【examples】增加大量使用示例：[详情](https://gitee.com/rulego/rulego/tree/main/examples)
- 【标准组件】增加数据库客户端节点组件(dbClient)，支持mysql和postgres数据库，可以在规则链通过配置方式对数据库进行增删修改查：[使用示例](https://gitee.com/rulego/rulego/tree/main/examples/db_client)
- 【[扩展组件](https://gitee.com/rulego/rulego-components) 】增加redis客户端节点组件(x/redisClient):[使用示例](https://gitee.com/rulego/rulego-components/tree/main/examples/redis)
- 【规则链引擎】增加加载指定路径文件夹所有规则链功能
- 【HTTP Endpoint组件】URL Query参数自动存放到msg.Metadata
- 【msg】 msg.Metadata value允许为空
- 【节点组件】节点配置，支持字符串映射成time.Duration类型
- 规则链配置文件支持配置规则链id

### 修复

- 修复mqttClient节点组件，随机clientId不生效问题

### 改进

- [Endpoint](https://gitee.com/rulego/rulego/blob/main/endpoint/README_ZH.md) 接口抽象，实现types.Node 接口，上层可以根据Endpoint”类型“统一调用
- js脚本相关节点，处理msg支持数组和map方式
- 【HTTP Endpoint组件】配置 Addr改成Server

### 其他信息

- 欢迎在 [Gitee](https://gitee.com/rulego/rulego) 或者 [Github](https://github.com/rulego/rulego) 上提交反馈或建议
- 扩展组件rulego-components：[Gitee](https://gitee.com/rulego/rulego-components)  [Github](https://github.com/rulego/rulego-components)
- 欢迎加入社区讨论QQ群：720103251


## [v0.13.0] 2023/8/23

### 新功能

- 新增数据集成模块(**Endpoint**)，使用文档和介绍点击：[Gitee](https://gitee.com/rulego/rulego/blob/main/endpoint/README_ZH.md) 或者 [Github](https://github.com/rulego/rulego/blob/main/endpoint/README_ZH.md)
    - 提供统一的数据处理抽象，方便异构系统数据集成，目前支持HTTP和MQTT协议
    - 支持其他协议集成扩展，例如：kafka数据等
    - 支持统一的数据路由和数据响应
- 新增字段过滤器组件(**fieldFilter**)
- 新增RuleEngine.OnMsgWithOptions方法，支持传递context和共享数据
- 组件支持ctx.GetContext().Value(shareKey)获取共享数据


### 修复

- 修复RuleEngine rootCtx不安全问题

### 改进

- jsFilter、jsSwitch、jsTransform、log组件，在dataType=JSON数据类型下，支持js脚本使用msg.xx方式操作msg payload
- 重命名mqttClient组件tls相关字段
- 优化Metadata使用
- 优化testcases
- 优化README

### 其他信息

- 新增RuleGo扩展组件库项目，欢迎贡献组件
    - 详情点击：[Gitee](https://gitee.com/rulego/rulego-components) 或者 [Github](https://github.com/rulego/rulego-components)

- 欢迎在 [Gitee](https://gitee.com/rulego/rulego) 或者 [Github](https://github.com/rulego/rulego) 上提交反馈或建议    