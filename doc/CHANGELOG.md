# CHANGELOG

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