# CHANGELOG

# [v0.34.0] 2025/11/03
### rulego-core
- feat(components): 延迟组件(delayNode),延迟时间支持毫秒
- feat(components): 引用组件(refNode)支持引用子链
- feat(components): 数据库客户端组件(dbClient)支持执行DDL和数据库方言
- feat: 组件表单生成，支持生成 icon 字段
- feat: 组件表单生成，完善通过tag配置表单
- fix: 修复mqtt endpoint组件会初始化2个客户端
- fix: 修复不支持多层嵌套跨节点取值
- fix: 修复子链不支持跨节点取值
- fix(components): fieldFilter组件CheckAllKeys 模式下逻辑错误
- fix(components): CheckAllKeys 模式下逻辑错误
- fix(components): join组件某些情况下阻塞
- chore(ci):actions workflow增加mysql

# [v0.33.0] 2025/09/03
### rulego-core
- feat: 完善组件配置表达式取值系统，支持跨节点取值，例如:${node1.msg.xx}
- feat: 增加 end 节点组件
- feat: 增加跨节点取值节点组件
- feat: 节点组件配置表单生成，跳过不可导出和 `json:-` 字段
- perf: 完善mqtt客户端重连机制
- perf: 优化js引擎超时处理
- perf: 优化表达式引擎混合字符串场景性能
- perf: 使用 el.NewTemplate 代替 str.NewTemplate
- perf: 完善net endpoint组件数据竞争问题
- fix: js节点组件，dataType 字段类型转换错误
- fix: Reload engine chainCtx lost
- fix: 修复js脚本部分场景会读写错误
- refactor: 重构 ctx.TellFlow 入参
- refactor: 增加公共组件分类，重新调整部分组件分类
- chore: expr 升级到1.17.6

### rulego-components
- feat: 增加 pulsar 发布和订阅节点组件
- feat: 增加 pulsar 发布和订阅节点组件
- feat: 增加流式计算转换节点组件
- feat: 增加流式聚合运算节点组件

### rulego-server
- fix: 完善mqtt客户端重连机制
- fix: add defer resp.Body.Close() for GetComponentsFromMarketplace

### rulego-editor
- feat: 默认【输入】节点可以删除
- feat: 增加最新节点组件支持
- feat: 如果节点没配置，首次添加到画布，不弹出属性配置表单
- feat: 增加新型画布节点，应用于for、节点组等分组组件展示效果
- feat: 增加sql编辑器表单组件
- fix:左边栏高度适配
- chore: 升级最新的依赖

# [v0.32.0] 2025/07/11

### rulego-core
- feat: endpoint/http restApiCall支持无感切换成fasthttp实现
- feat: endpoint配置支持变量替换
- feat: 规则引擎重载增加错误恢复机制
- feat: 规则引擎增加优雅关闭
- feat: 增加消息传递的写时复制(Copy-on-Write)机制
- feat: RuleMsg增加zero-copy API
- feat: RuleMsg消息负荷使用[]byte代替string
- feat: 脚本组件支持处理字节数组输入
- feat(endpoint/http): 增加读写超时配置
- feat(endpoint/ws): 改进事件注册
- feat(endpoint/net): 支持多种拆包配置
- fix: 修复多个组件OnMsg和Destroy方法之间的竞态条件
- fix: 修复表达式引擎`vm.VM`并发故障问题
- fix: 增强ReloadChild和ReloadSelf方法保护
- fix: 修复endpoint Marshal DSL循环依赖问题
- fix: 修复groupAction、groupFilter数据竞争
- fix(endpoint/mqtt): MaxReconnectInterval支持秒数配置方法
- refactor: 组件配置字段名优先从JSON tag获取
- refactor: 优化exprFilter组件初始化错误
- refactor: 改进restApiCall组件代理逻辑
- refactor: 将Config NetPool字段重命名为NodePool
- refactor: 不在支持直接访问msg.Data，使用msg.GetData()和msg.SetData('')代替
- perf: 脚本组件增加智能直通模式
- perf: 简化共享节点组件实现
- perf: 使用对象池优化DefaultRuleContext
- perf: 优化表达式获取变量性能
- perf: 所有组件和测试用例通过`-race`模式测试
- perf: 完善代码注释
- perf: 增加更多的示例和测试用例

### rulego-components
- feat: 增加fasthttp组件
- feat: kafka组件增加SASL和TLS配置
- feat: Lua脚本支持处理字节流
- feat: Lua脚本支持数组转换
- feat: 增加集成测试和CI设置
- feat(ci): 增加全面的GitHub Actions CI/CD流水线和中间件测试
- feat(ci): 为所有拉取请求触发CI
- fix: 修复kafka组件重连问题
- perf: 使用零拷贝优化元数据访问
- perf: 改进生命周期管理和测试

# [v0.31.0] 2025/05/20

### rulego-core
- feat: 增加cacheSet/cacheGet/cacheDelete组件节点
- feat: 增加缓存模块
- feat(restApiCall): 允许自定义body并优化变量取值
- feat: 节点配置支持混合字符串和变量取值
- feat: 节点池添加 AddNode API
- feat: base endpoint添加 HasRouter API
- feat: 添加默认HTTP endpoint到节点池
- feat: endpoint可获取规则链DSL
- feat(rest endpoint): rest endpoint重启增加关闭超时
- feat: 统一js和lua 自定义函数注册方法
- feat: 支持把所有结构体导出函数绑定到js和lua中
- feat: 脚本可以操作缓存
- fix(switch): 修复Switch节点配置不能完全覆盖默认cases参数
- fix(restApiCall): restApiCall节点请求失败无法在元数据拿到请求错误信息 
- fix(rest endpoint): rest endpoint共享节点热更新无法恢复路由
- fix(join): Join节点未收集错误节点信息
- refactor: 优化JS引擎测试用例
- refactor: 删除无用代码
- refactor: 热更新endpoint路由恢复忽略错误
- chore: 优化注释

### rulego-server
- fix: 修复HTTP服务器重启后`/editor`找不到问题
- feat: 注册mcp server endpoint
- feat: 共享系统默认http server

### rulego-components
- feat: Lua脚本支持与JS相同的UDF注册方法
- feat: Lua脚本可调用缓存方法
- feat: 添加mcp server endpoint

### rulego-editor
- feat: 增加缓存组件
- feat: rest节点增加body参数自定义配置
- feat: 添加mcp server endpoint节点
- fix: 解决复制和删除快捷键在某些情况下不生效问题
- opt: 优化集成显示

## [v0.30.0] 2025/04/03
- feat:增加动态组件，支持通过规则链DSL定义组件
- feat:组件注册器增加支持多租户
- feat:引擎池支持规则引擎实例添加、修改、删除回调
- feat:组件增加CategoryGetter DescGetter可选接口
- feat:组件表单增加必填字段
- feat(server):增加组件市场、组件安装、组件卸载API
- feat(server):增加MCP服务器
- feat(server):组件、规则链、rulego-server API支持自动注册成MCP工具
- feat(server):rulego-server分离到独立仓库维护: https://github.com/rulego/rulego-server
- feat(server):rulego-server 开源新版本的UI
- fix:修复共享节点一种类型只能配置一个
- fix:OutBuiltins lock err
- fix:[dbClient]连接不成功导致的错误
- opt:优化组件初始化错误提示
- opt:rest endpoint组件延迟获取body
- chore:build.yaml 支持编译成arm64
- chore:升级github.com/expr-lang/expr至v1.17.2

## [v0.29.0] 2025/03/06
- feat(components):增加wukongIM节点组件 @dimon
- feat(components):增加wukongIM输入端组件 @dimon
- feat(components):增加beanstalkd输入端组件 @dimon
- feat(components):增加beanstalkd节点组件 @dimon
- feat(components):增加modbus读写节点组件 @dimon
- feat(components):完善大模型节点组件
- feat(components):增加获取git日志节点组件
- feat:增加规则链校验拦截器
- feat:校验规则链是否形成环、子规则链不允许探究输入端组件
- feat:DSL NodeConnection 增加Label字段
- opt:网络客户端组件运行延迟初始化
- opt:restApiCall节点组件把响应错误通过err传递到下一个节点
- feat(server):rulego-server支持多租户和权限校验
- feat(server):rulego-server支持apiKey访问api
- refactor:OnNodeBeforeInit 和 OnChainBeforeInitAspect支持获得Config
- refactor(components):弃用旧版的大模型组件
- refactor(components):优化mqtt客户端连接失败错误提示

## [v0.28.0] 2025/01/09
- feat(components):增加opcua endpoint组件 @dimon
- feat(components):增加opcua读节点组件 @dimon
- feat(components):增加opcua写节点组件 @dimon
- feat(components):增加gRPC 流endpoint组件 @付晨阳
- feat(components):增加Mysql CDC endpoint组件
- feat(components):增加OpenTelemetry组件
- feat(components):endpoint/ws 支持配置跨域
- feat:for节点增加异步模式
- feat:js引擎注入RuleContext @Husky
- fix:解决规则链有多个结束点，会导出endpoint异常
- fix:str.ExecuteTemplate 空参数问题
- fix(server):save api无法保存vars
- opt(components):优化 dbClient组件获取参数
- opt:优化节点表单定义
- opt:restApiCall节点读超时默认值改成2000ms
- opt(components):redis endpoint组件接收数据后XDel @Brian B. Williams

## [v0.27.0] 2024/12/08

- feat:允许获得endpoint router错误
- feat:规则链DSL增加Disabled字段
- feat(endpoint/rest):允许设置跨域
- feat(restApiCallNode):允许配置不校验证书
- feat(flow):子规则链允许设置成继承模式
- feat:如果规则链Disabled，则初始化引擎错误
- feat(groupActionNode):节点ID列表允许string和数组格式
- feat(builtin):增加toHex和setJsonDataType内置函数
- feat(netNode):支持不发送心跳包
- fix(endpoint/rest):类型识别错误
- opt(netNode):优化重连机制
- refactor:dsl additionInfo 改成map[string]interface{}类型
- refactor:删除log依赖
- refactor(server):重构rulego-server api
- feat(server):规则链存储增加索引
- feat(server):自动创建默认用户
- feat(server):增加部署、停用规则链API
- feat(server):允许通过Disabled字段搜索规则链
- feat(server):增加默认的前端访问路由
- fix(server):启动错误退出
- ci(server):减少编译包文件大小
- ci(server):提供RuleGo-Editor编辑器离线部署包
### RuleGo-Editor[v0.27.0]
- feat(rulego-editor):规则链列表管理
- feat(rulego-editor):显示规则链状态和标题
- feat(rulego-editor):打开规则链
- feat(rulego-editor):编辑规则链
- feat(rulego-editor):查询规则链集成URL
- feat(rulego-editor):优化导入导出
- feat(rulego-editor):组件管理
- feat(rulego-editor):后台API配置持久化
- feat(rulego-editor):规则链部署/下线操作
- feat(rulego-editor):增加框选、撤销、重做、小地图、全屏操作工具
- feat(rulego-editor):子规则链节点允许通过下拉选取子规则链

## [v0.26.0] 2024/11/07

- feat:增加注释节点
- feat:增加条件分支节点(switch node)
- feat:增加规则引擎指标统计模块
- feat:增加并发限制aspect
- feat:start aspect 提供错误中断机制
- feat:提供 NewRuleGo Api
- feat:net组件允许使用节点池方式
- fix:flow node 并发读写问题
- fix:http endpoint 异步执行会出现context canceled
- refactor:js转换器组件忽略json转换错误
- refactor:重构内置函数注册器
- refactor:路由节点默认关系修改成Default
- chore:完善部分注释
- fix(server):config.conf允许配置js执行操作参数
- feat(rulego-components):增加MongoDB节点组件
- feat(rulego-components):增加redis 发布节点组件

## [v0.25.0] 2024/10/07

- feat:增加并行网关节点组件
- feat:增加合并汇聚节点组件
- feat:for节点组件增加合并遍历结果选项
- feat:节点组和子规则链节点移除合并metadata
- feat:ruleContext允许获得Out Message和error
- feat:websocket endpoint setBody返回错误
- feat:增加js内置函数注册器
- fix:http endpoint无法使用节点池
- chore:增加贡献文档
- chore:升级依赖
- perf(server):优化保存运行日志
- fix(server):实时执行日志需要过滤其他规则链数据
- fix(server):实时日志响应错误，需要移除客户端
- feat(rulego-components):增加gRPC客户端节点组件
- feat(rulego-components):增加git push节点组件
- feat(rulego-components):增加git create tag节点组件
- feat(rulego-components):增加git commit节点组件
- feat(rulego-editor):增加最新版本节点配置
- feat(rulego-editor):允许跨规则链复制节点

## [v0.24.0] 2024/09/09

- feat:增加节点连接资源复用机制
- feat:网络连接类组件支持共享连接池
- feat:增加引用节点的节点
- feat:exec node允许通过stderr获取数据
- feat:http endpoint允许响应html页面
- fix(server):post msg api没有workDir
- feat(server):增加节点复用相关api
- feat(server):加载全局共享组件
- feat(rulego-components):增加rabbitmq endpoint和节点组件
- feat(rulego-components):增加opengemini读和opengemini写组件
- feat(rulego-components):组件支持连接池
- refactor(rulego-components):kafka组件brokers字段改成server
- feat(rulego-editor):规则链ID默认使用nanoid
- feat(rulego-editor):endpoint支持多路由
- feat(rulego-editor):增加连接类型国际化
- feat(rulego-editor):增加连接池下拉选项
- feat(rulego-editor):增加最新版本节点配置

## [v0.23.0] 2024/08/11
- feat(server):动态获取functions节点内置函数列表API
- feat(server):日志分页
- feat(server):config.conf支持自定义的global配置
- feat(rulego-components):增加redis stream endpoint组件
- feat(rulego-components):redis 组件支持配置密码
- feat(rulego-components):redis 组件支持HMSET、HGETALL、HDEL等操作
- feat(rulego-components):redis 组件支持动态参数
- feat(rulego-components-ci):增加gitClone组件
- feat(rulego-components-ci):增加服务器指标监控组件，如：cpu、内存、磁盘、网络等
- feat(builtin/processor):增加metadataToHeaders内置processor函数
- feat(builtin/processor):内置responseToBody函数 支持所有endpoint类型
- feat:rest endpoint GET请求，消息负荷从查询参数读取
- feat:统一所有组件配置变量取值方法。
- fix(server):无法删除规则链
- fix(server):websocket断开连接错误
- fix:for node 修改out数据
- fix:TellNode找不到节点，没触发第二个回调
- fix:dbClient node 在某些go版本下，转换int64错误
- fix:ToString 函数适配 map[interface{}]interface{} 类型
- refactor:打印endpoint详细错误栈
- refactor:builtin/processor 区分 in 和 out类型
- refactor:优化规则链解析器

### RuleGo-Editor[v1.4]
- feat:支持rulego最新版本组件配置
- feat:支持endpoint组件配置
- feat:支持下拉表单
- fix:修复边文本越界问题
- fix:保存规则链失败没提示
- fix:解决0值无法显示问题
- fix:自定义组件无法显示问题
- refactor:Input节点允许移动
- refactor:增加帮助文档链接
- refactor:升级element-plus
- refactor:引入element-plus zhCn lang

## [v0.22.0] 2024/07/08
- feat[rulego-editor]: 接入端(endpoint)允许可视化配置。体验地址：[http://8.134.32.225:9090/ui/](http://8.134.32.225:9090/ui/)
- feat[rulego-components]: 增加redis endpoint组件
- feat[rulego-components]: 增加redis 节点组件允许配置db参数
- feat[rulego-components]: 增加nats endpoint组件
- feat[rulego-components]: 增加nats 节点组件
- feat: 增加for节点组件，用于控制循环节点
- feat: 增加执行本地命令节点组件，用于控制循环节点
- feat: 增加template节点组件
- feat: 增加metadataTransform节点组件
- feat: 增加OnChainBeforeInitAspect和OnNodeBeforeInitAspect增强点
- feat: 增加规则引擎中断恢复相关API
- feat: endpoint允许指定从规则链某节点开始执行
- fix: mqtt client平滑关闭
- refactor: endpoint type名称增加前缀
- refactor: iterator 节点组件标记弃用

## [v0.21.0] 2024/06/06

- feat: rule chain DSL允许动态配置接入端（endpoint）
- feat: 接入端（endpoint）允许通过DSL动态配置和启动
- feat: endpoint通过无阻塞方式启动
- feat: endpoint router允许传递context
- feat: endpoint 组件注册和rule 组件注册合并
- feat: 增加nats 节点组件
- feat: msgTypeSwitch 和jsSwitch 节点如果没任何匹配转发到默认链
- feat: 增加nats endpoint组件
- fix: 子规则链context丢失问题
- fix: examples/server 规则链文件解析失败不保存
- refactor: endpoint 模块优化，调整目录结构
- refactor: engine 模块优化，调整目录结构
- refactor：优化aspect初始化
- chore：examples/server build关闭CGO_ENABLED
- chore：examples/server 加入nats组件

## [v0.20.0] 2024/04/24
- feat: 允许不同脚本相同的函数名
- feat: restApiCall 节点允许空body
- feat: 可以得到规则链执行快照
- feat: 允许在OnMsg上下文添加onDebug回调函数
- feat: endpoint允许添加RuleContextOption
- feat: 规则链DSL文件可以添加vars变量
- feat: 节点配置允许通过规则链vars值替换
- feat: 规则链池增加reload和range方法
- feat: websocket endpoint允许和rest endpoint 共用用一个server
- feat: 节点debugMode 允许被规则链的debugMode参数统一覆盖
- feat: 子规则链允许通过Failure和其他节点连接
- feat: 加载规则链跳过出错的规则链
- feat: 规则链引擎增加初始化标志
- feat: js相关节点运行时允许通过`vars.xx`访问规则链vars
- feat: 重构examples/server 提供基于rulego开发应用的脚手架，前端地址：[example.rulego.cc](https://example.rulego.cc/)
- feat: 增加rulego-components-ai模块，提供AI组件
- feat: 增加rulego-components-ci模块，提供CD/CI组件
- feat: 增加rulego-components-iot模块，提供iot组件
- fix: mqtt client节点如果连接不上mqtt broker允许延迟连接，而不是报错
- fix: 修复groupAction节点，可能并发读写问题
- fix: 规则链没有节点，执行报错问题
- opt: 优化大js文件的执行效率

## [v0.19.0] 2024/02/18

- feat:增加表达式过滤器节点组件。[文档](https://rulego.cc/pages/c8fe75/)
- feat:增加表达式转换节点组件。[文档](https://rulego.cc/pages/3769cc/)
  表达式示例：
  使用函数：upper(msg.name)
  判断：(msg.temperature+10)>50
  三元运算：upper(msg.name==nil?'no':msg.name)
  截取字符串：msg.name[:4]
  替换字符串：replace("Hello World", "World", "Universe") == "Hello Universe"

- feat:增加groupAction节点组件，把多个节点组成一个分组，异步执行所有节点，等待所有节点执行完成后，把所有节点结果合并，发送到下一个节点。[文档](https://rulego.cc/pages/bf06e2/)
- feat:增加迭代器节点组件。遍历msg或者msg中指定字段每一项值到下一个节。[文档](https://rulego.cc/pages/5898a0/)
- fix:修复子规则结果合并，并发问题。
- fix:onEnd某些原因可能会重复调用问题。
- fix:metadata可能会出现并发读写问题。
- fix:js引擎初始化增加并发保护。
- fix:jsTransform 遇到NaN值，流转到TellFailure分支。

## [v0.18.0] 2023/12/27

- feat:增加AOP模块，它允许在不修改规则链或节点的原有逻辑的情况下，对规则链的执行添加额外的行为，或者直接替换原规则链或者节点逻辑。 提供以下增强点：Before Advice、After Advice、Around Advice、Start Advice、End Advice、Completed Advice、OnCreated Advice、OnReload Advice、OnDestroy Advice。[文档](https://rulego.cc/pages/a1ed6c/)
- feat:restApiCall节点组件，增加SSE(Server-Sent Events)流式请求模式，支持对接大模型接口。
- feat:增加CI自动化测试流程。
- feat:增加大量单元测试，覆盖率达到92%。
- feat:增加性能[测试用例](https://rulego.cc/pages/f60381/) 。
- feat:sendEmail节点组件，增加ConnectTimeout配置。
- feat:/examples/server示例工程，增加 -js -plugins -chain_id flags，支持启动加载js原生文件、插件和指定mqtt订阅处理规则链ID。
- fix:/examples/server示例工程，规则链文件夹多层路径无法正常解析。
- fix:/examples/server示例工程，保存规则链，可能会出现旧规则链文件数据无法正确覆盖。
- fix:metadata可能会出现并发读写问题。
- fix:规则引擎同步处理数据，有几率无法正确调用onCompleted回调函数。
- fix:RuleChainPool nil问题。
- fix:mqtt endpoint，无法通过header得到主题。
- refactor:onEnd回调函数允许得到relationType。
- refactor:删除函数Configuration.GetToString。
- opt:部分组件，增强nil检查。
- opt:dsl AdditionalInfo字段 增加omitempty json tag。
- opt:run go fmt。

## [v0.17.0] 2023/11/27

- feat:增加websocket endpoint组件 [文档](https://rulego.cc/pages/e36f41/)
- feat:增加tcp/udp endpoint组件 [文档](https://rulego.cc/pages/b7050c/)
- feat:增加kafka endpoint组件(扩展组件库) [文档](https://rulego.cc/pages/07ad50/)
- feat:增加tcp/udp 节点组件[文档](https://rulego.cc/pages/c1af87/)
- feat:endpoint组件使用统一的创建方式[文档](https://rulego.cc/pages/5a3227/)
- feat:增加过滤器组节点组件[文档](https://rulego.cc/pages/b14e3b/)
- feat:增加子规则链节点组件（原子规则链配置方式废弃）[文档](https://rulego.cc/pages/e27cec/)
- feat:允许子规则链接其它节点
- feat:functions节点组件，支持动态指定函数名
- feat:delay节点组件，增加覆盖模式
- feat:支持加载JavaScript脚本文件
- feat:onEnd回调函数，支持获取ctx
- feat:examples/server 使用独立的go.mod
- feat:examples/server 支持是否引入扩展组件库的build tags
- feat:mqtt client 允许重连被取消
- fix:http endpoint 如果不是application/json无法获取body
- fix:mqtt client 节点组件，没有重试次数限制
- opt:Metadata修改实现方式
- opt:rest node  ReadTimeoutMs 默认值改成 0
- opt:mqtt client config MaxReconnectInterval改成int
- opt:Node接口OnMsg取消返回值error
- opt:config.JsMaxExecutionTime->ScriptMaxExecutionTime
- opt:Endpoint.AddRouterWithParams->Endpoint.AddRouter
- opt:Endpoint.RemoveRouterWithParams->Endpoint.RemoveRouter
- opt:RuleMetadata.RuleChainConnections标记弃用
- opt:config.OnEnd标记弃用
- opt:RuleEngine.OnMsgWithEndFunc标记弃用
- opt:RuleEngine.OnMsgWithOptions标记弃用
- opt:添加doc overview

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