# CHANGELOG

# [v1.0.0] 2026/10/14
> 首个 1.0 大版本，包含 v0.37.2、v0.37.3、v0.38.0 三个中间版本的累积更新
### rulego-core
- refactor!: 移除全部废弃 API——`WithEndFunc`/`OnMsgWithEndFunc`/`OnMsgWithOptions`（改用 `WithOnEnd`/`OnMsg`）、`SubmitTack`、iterator 迭代节点组件（由 for 节点取代）、`utils/str` 字符串模板（`NewTemplate`/`ExecuteTemplate`/`Template` 接口，统一迁移到 `utils/el` 表达式模板）
- feat(api): 新增 `types.Version` 版本常量与哨兵错误（规则链/节点不存在等）；日志全面迁移到分级方法，`Logger.Printf` 移除计划公示
- feat: 新增业务日志事件——内置 BusinessLog 切面，节点日志模板上报进调试通道与运行记录
- feat(engine): 消息 hop 预算防失控循环（`msg_max_hops` 可配），worker 池打满同步降级兜底
- feat(engine): join 语义重做——等待直接入边分支完成，不再等待整个 fork 子树；修复分支竞态导致的提前完成与误放行
- feat(flow,ref): flow 节点支持 `chainId:nodeId` 跨链起点与 `${}` 动态 targetId（msg/metadata/global/vars），跨链调用带上 fromChainId/fromNodeId 溯源
- feat(node_pool): 新增分布式原语 Locker/OnceGuard/ActiveGuard——schedule endpoint 多副本仅触发一次、endpoint 主备选主；共享节点支持别名查找 (#96)；SetConfig 锁内热更换配置
- feat(endpoint): 链内 `ref://` 引用从 endpoint 解析到同链节点/端点（两阶段部署 + 连接池成环守卫）；mqtt 消息数据类型可配置（留空自动探测/JSON/TEXT/BINARY）；schedule 空 msgType 归一 JSON；mqtt/net/ws/ssh 连接态字段补 ref 标签；EventConnect 支持关闭 websocket 连接
- feat(engine): 运行记录事件补 msgType/msgData/耗时并按开始时间排序，全局运行日志级别与规则链完成回调；端点回调错误携带链/节点定位；debug 模式跨链继承；日志节点输出进调试通道；节点/endpoint 实时连接状态；endpoint 触发消息带 `_triggerSource` 元数据
- feat(log): 日志输出位置 outputTo（both/console/logger）可配；服务端日志带链/节点前缀
- feat(el): 新增一次性 ExecuteTemplate 帮助函数，语义兼容旧 str 模板
- perf(engine): 同步等待 hop 内联、节点切面解析缓存、worker 池分片，降低每消息分配
- fix(engine): 修复组节点计数竞态与 panic 吞没、for/while 取消传播、Join/Group 节点 goroutine 泄漏
- fix(js): 每次执行独立超时定时器并清理池化 VM 残留中断；编译错误行号对齐用户脚本
- fix(endpoint): 修复重叠分隔符匹配 (#128)、延迟路由清理与部署失败回滚、池重载目标与并发创建竞争
- fix(filter): 组过滤到期边界恢复已有结果，不再误报超时
- fix: 路由缓存重载安全；pool/ssh/cache/mqtt/exec 一轮加固；AES 解密对非法密文加固
- test: 聚合覆盖率提升到 88.4%
### rulego-components
- feat(streamsql): 流-流 JOIN（WITHIN 窗口）经 streamKey 路由支持，streamsql 升级 v1.3.0；columns 输入格式支持透视
- feat(endpoint): fasthttp 流式性能重构——非 SSE 请求零开销惰性门控、非流式路由同步路径、惰性请求快照；fasthttp 升级 v1.73.0
- feat(endpoint): kafka 共享连接 (#9)；redis/rabbitmq 多副本单活消费者选主；Redis Locker 支撑跨副本仅一次执行
- feat: 消息组件重连循环与实时连接状态（redis/redis_stream/rabbitmq/mongodb/nsq 等 endpoint 与客户端）
- feat(rabbitmq): 消费端 queueName/exclusive 可配置，客户端节点消息持久化发布
- feat(redis): outputTo 选项把命令结果写入 metadata；内联命令引号感知切分（`SET key ""` 正确存空串）
- feat: 构造期字段标 `ref:shared`（redis/kafka/otel/wukongim/email），借用模式自动隐藏；endpoint 与外部客户端节点支持借用同链连接
- refactor(components)!: SSH 认证字段合并——私钥统一 `certKeyFile`，password/key 互斥；支持私钥内容、文件路径与 passphrase
- perf(restApiCall): 默认连接池 200 → 2000
- fix: beanstalkd 死锁与失败任务无限重投、kafka/rabbitmq 重连复活已删路由、nsq/nats/pulsar/grpc_stream 数据竞争与生命周期幂等、lua nil 处理、otel 模板值解析、redis psubscribe 等待确认
### rulego-components-ai
- feat: 新增轻量 ReAct 组件 ai/agent（纯标准库，32 位可编译）；多轮工具调用 token 用量元数据累加、透传链上下文
- feat: 新增 ai/jev 决策节点与 ai/jevFilter 过滤节点
- feat: lite 工具配置复用 config.Tool，承接 mcp/builtin skill 并支持远程 MCP
- feat: 图片处理注册 BMP/WebP 解码器；流式 delta 按帧合批转发，LLM 客户端补传输层超时
- feat(agent): MCP 工具返回的图片不再降级占位符，透传给视觉模型识别
- fix(agent): 流式工具调用误判为纯文本时自动升级 drain 并重跑本轮，检测放宽到流中任意 chunk
- fix(mcp): structuredContent 透传到工具文本，跨轮句柄不丢失
- fix: 丢弃空 assistant 消息避免 LLM 端点 400
- chore: eino 升级 v0.9.19；browseruse 改用 encoding/json 去除 sonic 直接依赖
### rulego-components-iot
- feat: 新增 BACnet/IP 协议采集，工业协议族扩展到 11 个
- feat(modbus)!: 校验位取值支持 N/O/E 字母并以其为规范形态，修复校验位解码
- feat(iot): 共享重试/重连 runner 与连接状态上报；重连退避；tsdb 初始化与操作锁有界化
- feat(snmp,iec104): 读点位应用缩放/偏移系数，ScaleValue 全协议补齐
- feat(tsdb): 扁平 map 输入保留 timestamp/window_id 保留键
- perf: 批量读——modbus 连续寄存器合并、S7 按 area/DB 合并区间读、SNMP 批量 GET、FINS 0x0104 批量读（STRING 类型与点位级字节序）
- fix: 客户端泄漏、串口请求串扰、OPC UA 名称批量化、snmp walk 上限；静默设备超时跳过逐点回退与重试；iec104 内嵌 go-iecp5 修复竞态；旧版 x/modbus 读输出补 JSON dataType；paho.mqtt/prometheus CVE 升级
### rulego-components-discovery
- feat: nacos 组件连接状态上报与初始化软失败
- feat: nacos SDK 日志经 WithLogger 路由进 rulego 日志，服务不可达时抑制重复刷屏
### rulego-server
- feat: 多租户用户管理（角色、API Key）；管理面审计日志——可插拔存储、拒绝请求采样、MCP 规则链变更工具 RBAC 与审计
- feat: 规则链版本快照与回滚；保存双轨——保存记版本（滚动保留上限）、保存草稿去重快照；新建链只落库不上线
- feat: 全局配置热更新（精准重载）、环境变量引用展开与运行时覆盖/恢复；敏感项掩码持久化
- feat: 可嵌入 bridge 模式——auth SPI、信封包装、无独立监听，嵌入模式注册调试 websocket
- feat: 概览/版本/规则链分类端点；规则列表索引化摘要（含分类计数与 ai/agent 标记）与磁盘对账；分类过滤 `none` 匹配未分类
- feat: 运行日志分级（off/summary/detail）异步写；运行记录事件记录失败节点 ID
- feat: 规则链市场支持本地目录模式
- feat: 用户引擎注入 Locker/owner——schedule 多副本去重、ActiveGuard 选主
- feat: chat completions 按用户限流；登录限流可配置；消息 hop 上限与 worker 池上限可配置
- feat: 静态资源与 JSON API gzip 压缩与缓存头
- fix(security): 一轮加固——拒绝已删除用户认证、config 资源仅管理员、iotpoint/builtin 独立资源、MCP 资源 id 校验、请求体上限与市场拉取超时、调试数据与 ws 广播按用户隔离、JWT 算法白名单、模型 key 掩码、拒绝删除 config.conf 内置账号、内置智能体防 Undeploy 与标记丢失
- fix: 冷启动按序预加载共享节点池与动态组件；已停链/未部署链保存保持状态；动态组件安装 dsl id 与路径不一致拒绝并回滚注册
- chore(server): 依赖工具链要求 Go 1.25+
### rulego-editor
- feat: 保存双轨与版本管理——保存=记版本+上线（新建不上线）、保存草稿；历史版本按来源分档、回滚、超 50 条加载更早、顶栏版本号
- feat: 调试体验大版本——运行耗时瀑布图、运行记录对比（消息 diff/差异定位）、调试表单记忆、右键插入日志节点、错误定位、未部署链发送守卫
- feat: 全局配置面板与环境变量引用/运行时覆盖标记；`${vars.}`/`${global.}` 变量联想补全
- feat: 业务日志事件——节点日志模板、日志上报设置与角标
- feat: endpoint 连线绑定多条路由；子规则链连线自定义路由标签
- feat: 循环分组边界守卫（组内外连线互禁、入组带回 parentGroupId、分组框尺寸入 DSL）；循环起点伪节点与悬浮快捷加号；while 死循环检测
- feat: ref/flow 引用开放同链节点/endpoint 与跨链两步选择器，支持 `${}` 表达式手填；kafka 共享连接池下拉与跨类型引用
- feat: AI 集成——ai/jev 节点、零依赖 AIChat 对话组件、openChat API、flashNodes 落图高亮、extractRuleChainFromToolCalls 导出、AI 设置 extra-panes 插槽
- feat: 宿主集成 API——registerPropertyComponent 自定义属性控件、exportDataTransform 导出钩子、addRequestInterceptor 请求拦截器、resolveApiPrefix 子路径挂载、动态下拉数据源与字段联动、连线端点拖拽重连、SharedNodeManager 页面级复用；readonly 档升级完整预览形态、capabilities 深合并
- feat: 组件配置体验优化——密码掩码、枚举下拉化、高级字段折叠、认证字段条件显隐、placeholder 示例回退
- feat: 定时调度可视化 Cron 选择器与消息内容双模式编辑、频率中文可读化
- feat: 流组件表单内联元数据表、JOIN 关键词联想；mqtt 消息数据类型、rabbitmq 队列字段、redis outputTo 表单支持
- feat: dock 面板体系——「面板」子菜单三形态可用、资源树展开记忆、组件库分组默认折叠、面板空态统一；连接状态轮询自适应降频
- feat: 深色主题全量令牌化，LogicFlow 画布文字随主题切换；内置用户使用手册（PDF 随产物分发）
- fix: LogicFlow 升级 2.1.11/2.1.15 修复框选后画布无法拖动；401 令牌竞态修复退出重登录卡死；保存按钮脏态语义统一

# [v0.37.0] 2026/08/02
### rulego-core
- feat: 增加同链连接池——连接型组件可在链内按源节点 ID（`ref://节点ID`）复用连接，零引用计数；db/mqtt/net/ws 等组件完成适配
- feat(endpoint): endpoint 支持会话寻址推送；`ref://` 共享连接解析优先同链节点，未命中回退共享节点池
- feat(engine): Failure 分支把 TellFailure 错误暴露到 metadata.errorMsg (#26)
- feat: 增加组件别名机制，兼容旧组件类型名；规则引擎池支持按别名查找节点
- feat: 子链调试日志转发到根链控制台；TellFlow 传播父链 debug 模式到子规则链
- feat: RouterForm 增加 Params 字段，组件文档输出 endpoint 路由参数
- fix(switch): 单个 case 求值出错时跳过该分支，而不是终止整个实例
- fix(endpoint): 路由目标规则链不存在时输出错误日志
- fix(dbClient): opType 改为可选字段
- fix: 修复 tellOrElse 循环变量闭包捕获导致的数据竞争
### rulego-components
- feat(streamsql): streamTransform 支持流-表 JOIN 富化和分析函数；被过滤数据改走 False 链，Failure 仅保留给真正的处理错误
- feat(streamsql)!: streamAggregator 支持 CEP（MATCH_RECOGNIZE）模式识别；聚合结果关系类型由 window_event 更名为 stream_event
- fix(endpoint): 注册 kafka 和 nats endpoint 的旧类型别名，保持向后兼容
- fix(python): 空闲进程池缩容到 keepAlive 下限，避免全部回收
- fix: 修正 endpoint 组件文档和默认值
- chore: 移除 opengemini 组件（迁移到 rulego-components-iot 作为存储驱动）
### rulego-components-ai
- feat: 增加 AG-UI 流式模式，支持 OpenAI 工具调用协议
- feat: 流式响应返回 token 用量统计；支持思考过程流式透传
- feat: 支持会话级模型切换和思考强度扩展参数
- feat: 增强流式重试、故障转移与事件有序输出；failover 支持参数覆盖并覆盖 401 认证错误触发条件
- feat(agent): 增加 doom-loop 检测，自动识别并拦截死循环/重复工具调用；增加 panic 恢复
- feat(tool): 增加 grep/glob 工具，行为对齐 ripgrep（跳过二进制、感知 gitignore）
- feat(tool): 工具输出清洗与截断、多根路径安全解析；诊断 provider 改注册制，预置 ruff/tsc/eslint/shellcheck 模板
- feat(skill): 动态技能列表注入，支持禁用技能列表和注入宿主技能目录
- fix(tool/bash): 空白名单回退默认白名单；修复超时未终止进程导致 Windows 挂起
- fix: MemoryStorage 消息深拷贝并加数量上限防 OOM；文件工具启用安全路径解析防越界
- fix: full 模式流式重放改用阻塞反压，避免误判中断
### rulego-components-iot
- feat: 新增 8 个工业协议采集族——西门子 S7（x/s7Read/Write）、EtherNet/IP（x/eipRead/Write）、SNMP（x/snmpRead/Write 及 endpoint/snmp Trap 监听）、三菱 MC（x/mcRead/Write）、欧姆龙 FINS（x/finsRead/Write）、DL/T645 电表（x/dlt645Read/Write）、HJ212 环保（endpoint/hj212）、IEC 60870-5-104 电力（x/iec104Read/Write，协议栈合并进主 module 发布，无需 replace），加上已有 Modbus/OPC UA 共 10 个协议族；采集点位表支持缩放/偏移系数（ScaleValue）
- feat: 新增 5 个时序数据库存储驱动——openGemini、InfluxDB、TDengine、TimescaleDB（x/*Write、x/*Query）和 Prometheus Remote Write（x/promremoteWrite）；TDengine 支持超级表自动建子表、标识符转义与批量分块写入，TimescaleDB 支持多行 VALUES 批量写入
- feat: 新增协议/存储无关的通用组件 x/iotRead、x/iotWrite、x/tsdbWrite、x/tsdbQuery，仅配置 driver 字段即可切换具体协议/数据库；x/tsdbWrite 采集映射直接接受扁平 map 输入
- feat(control): 新增软 PLC 控制节点 x/control/timer（TON/TOF 延时动作）和 x/control/watchdog（心跳看门狗，失联触发告警）
- feat(modbus): 新增 x/modbusRead/x/modbusWrite 读写专用组件；新增 modbus_server 端点（设备上报模式），写回调携带 Modicon 地址和时间戳
- feat(opcua): 增加请求 Timeout；ToPointsData 支持缩放；在线浏览包含层次引用子类型；连接字段支持 ref:shared
- refactor!: modbus TLS 证书配置键 certPath/keyPath/caPath 更名为 certFile/certKeyFile/caFile
- fix(modbus): 修正串口校验位描述（1=Even，2=Odd）
### rulego-components-discovery（新组件库）
- feat: 新增 Nacos 微服务发现组件库——x/nacosServiceCall（服务发现 + HTTP 调用，轮询选取健康实例转发）、x/nacosConfigGet/x/nacosConfigSet（配置读/写）、endpoint/nacos（订阅配置变更自动触发规则链）
- feat: 提供服务注册 Server Hook（Register，临时实例 + 心跳 + 优雅反注册）
- feat: 同地址连接经 SharedNode 自动复用；支持 TLS 配置
### rulego-server
- feat: 集成 rulego-components-discovery，提供 Nacos 服务调用/配置中心能力；默认注册全部 IoT 采集、存储和转换组件
- feat: 新增 iotpoint 点位模板后端模块，内置 S7、IEC104、SNMP 等协议点位模板；新增 OPC UA 在线浏览处理器
- feat: 共享 HTTP server 支持注入用户引擎池复用，由 isSystemNode 保护（默认关闭）(#119)
- feat: 新增 AllChains 存储方法和规则链生命周期监听器；新增 AI 工具安全拦截切面
- feat: 内置组件支持动态查询（DynamicBuiltin）；app 支持 WithConfig 编程式注入配置
- feat(api): 组件表单增加 Order 排序字段
- fix(security): 修复分类路径穿越；跨用户规则链执行限制为系统智能体；SaveAndLoad 剥除 systemAgent 属性防生成不可删除规则链
- fix(skill): 文件上传从缓存请求体解析，修复流被读空导致失败
- chore: 移除废弃的 examples/server
- ci: release 打包内嵌 editor 前端界面
### rulego-editor
- feat: 画布交互全面增强——命令面板、右键菜单（多选对齐子菜单、endpoint 路由注入）、快捷键、节点对齐分布、节点搜索、网格吸附、Shift 溯源选择、调试时连线流动动画
- feat: 新增链路诊断（拓扑告警和必填校验，诊断规则注册表化）
- feat: 新增组件文档浏览器（设置菜单入口、分类列表、内嵌文档、悬浮查看）
- feat: IoT 点位表支持 Excel/CSV 导入导出、覆盖导入、批量删除、重名检查、批量地址生成
- feat: 新增 OPC UA 在线浏览对话框、Nacos 服务发现组件支持（含 TLS 配置）、x/tsdbQuery 组件支持
- feat: endpoint/schedule 的 cron 输入增强（croner 校验、下次执行预览、cronstrue 中文描述、常用预设）
- feat: 组件面板/节点选择器/节点管理全部折叠-展开开关，分类图标双层配色，侧边栏折叠把手重做，组件管理优化
- feat: 未注册组件支持属性面板编辑（JSON 通用编辑模式）
- feat: 库化——ESM 单文件构建（build:lib）、exports/peerDependencies 配置、.d.ts 类型定义
- feat: 聊天流式渲染增强（Markdown 流式、代码块复制/全屏、HTML/SVG 预览）；引入 DOMPurify 修复 XSS
- fix: 修复搜索框 TDZ 崩溃、编辑器保存覆盖宿主应用路由、初始化双重重建闪烁、el-table-column 缺 key 渲染崩溃等问题

# [v0.36.0] 2026/06/01
### rulego-core
- feat(engine): 增加 Stream 关系类型，支持同步执行流 (#63)
- feat(endpoint): 增加 SSE 流式推送支持、ScopedMessage 代理和 NetClient/WsClient 端点组件
- feat(endpoint): rest endpoint 增加 Restart 事件
- feat(node): 支持递归变量替换和规则链环境变量注入
- feat(schedule): 支持自定义消息体、类型和元数据参数
- feat(template): 增加 include 和 fileExists 内置函数
- feat(maps): 支持 struct 字段访问和回退键查找
- feat(logger): 重构 Logger 接口，支持日志级别
- feat: 增加 MCP 工具提供者接口，支持 AI 工具集成；增加 GetUdf/GetUdfs 方法和 AiTool 脚本类型
- feat(dsl): 增强节点引用提取和字段支持
- feat(components): 增加 while 节点组件，支持 mode 配置
- feat(components): cacheGet 节点增加 whenKeyNotFound 配置；cachePut 组件在 outputMode=2 且缓存 key 不存在时路由到 Failure 链 (#104)
- feat: 完善组件配置表单元数据（JSON 标签、label、ref 标签、RouterForm 和共享节点支持）
- feat: 支持 per-message debugMode 和 skip-tell-next 控制；预计算链路级别 debugMode
- feat: 默认字段名标签改为 'json'；支持嵌套结构体扁平化在组件表单配置中
- feat: GetEnv 支持访问全局 vars 变量
- feat(dbClient): 增加 IN 子句切片参数展开
- feat: NodePool.LoadFromRuleChain 跳过已加载条目
- fix: 修复 JoinNode 超时失败、LCA 算法导致死锁、回调提前触发和元数据未合并等问题
- fix: 修复 endpoint 节点重复写入
- fix: 修复全局变量在 endpoint 组件 router 字段不生效 (#93)
- fix: 修复 metadataToHeaders 处理器在 fasthttp 下不生效 (#95)
- fix: 修复 ${ } 占位符在 inclusive/switch 节点中的支持
- fix: 确保 ruleChainPool 传递到 rootRuleContext；传播父 ruleConfig 到动态组件子规则引擎
- fix: 修复并发分支数据竞争、websocket 并发写入和结果切片竞争问题
- refactor(cache): 重构 Cache.Get 签名，返回 (interface{}, error)
- refactor: 不序列化函数体
- perf: 优化文件操作函数
- chore: 升级 expr-lang/expr 到 v1.17.8
### rulego-components
- feat(nsq): 实现 NSQD 多节点轮询发布与容错机制
- feat: 增加 python 节点组件
- feat: 增加文件操作节点组件（读、写、删除、列表）
- feat: fasthttp endpoint 增加 restart 事件
- feat: 完善 SharedNode 组件配置的 ref 标签，支持共享连接
- opt: mongodb 客户端组件支持 ObjectId 自动转换
- refactor(lua): 重构 lua 组件到 transform/lua 目录，适配 Cache.Get 签名变更
- fix: 修复流式模式下 ctx.Response 数据竞争
- chore: 升级依赖
### rulego-components-ai
- feat: 转型为全功能 AI Agent 框架，增加 ReAct Agent 循环、Tool Agent 和 Agent 工厂
- feat: 增加 10 个 AOP 切面（日志、会话、可视化等），支持 Agent 执行生命周期拦截
- feat: 增加统一工具抽象层：bash、read、write、edit、browseruse、mcp、skill 等内置工具
- feat: 增加 MCP 双向协议支持（客户端 + 服务端）
- feat: 增加意图识别模块和多维度会话管理系统
- feat: 增加 OpenAI 流式处理器、Embedding 客户端和相似度计算
- feat: 增加动态模型选择、重试模型和多模态视觉支持
- feat: 增加 Skill 技能系统，支持通过规则链编排 AI 工具调用
### rulego-components-iot
- feat: 增加串口通信组件
- feat(modbus): 增加运行时配置持久化和分级日志；完善组件表单字段标签配置
- fix(modbus): 修复重连失败、惊群问题、多寄存器地址步进错误和共享节点池模式下的连接管理问题
- chore: 升级依赖
### rulego-server
- feat: server 从 examples/server 提升为架构级独立模块，作为 RuleGo 自动化工作流平台独立部署
- feat: 增加文件操作和串口通信组件支持
- feat(ci): 增加 32 位 Linux 构建目标和 server 模块 CI 工作流
- fix: 启动失败后保持规则链启用状态 (#97 #98)
- fix: 修复 32 位系统编译失败和空指针问题
- fix: 修复组件市场列表 API 缺少 installed 字段
- chore: 升级依赖；标记旧 examples/server 为废弃

# [v0.35.0] 2025/12/18
### rulego-core
- feat(components): join组件支持把错误传递到下一个节点
- feat(components): end组件支持把上一个节点错误传递到回调函数
- feat(components): 增加break组件
- feat(components): for节点组件支持中断
- feat(components): 延迟节点组件支持通过元数据获取时间偏移
- feat(components): join/groupAction组件支持把执行结果合并到同一个map
- feat(components): 函数组件支持参数配置
- feat(components): 函数组件函数注册支持添加显示名称和描述
- feat: 执行规则链支持执行实例取消
- feat: 引擎支持多节点执行恢复 
- feat: 增加Config.OnEndWithFailure 配置，出现错误时，是否调用结束回调函数
- fix: 引擎onEnd回调某些情况无法触发
- fix: 执行快照数据竞争问题
- perf: 优化引擎执行超时上下文性能
- perf: 优化引擎结束回调执行顺序
- chore(ci):压缩编译后文件
### rulego-components
- feat: opcua write组件支持int、double、数组等类型
- chore(ci):升级依赖

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
### rulego-components
- feat: nats endpoint组件支持 QueueSubscribe 模式

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
- feat:增加规则链全局配置Properties。[文档](https://rulego.cc/pages/config/#properties)
- feat:增加规则链全局配置和自定义函数到js运行时，js脚本可以调用golang自定义函数。[文档](https://rulego.cc/pages/config/#udf)
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