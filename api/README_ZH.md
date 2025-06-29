# Types API

------

该目录包含RuleGo规则引擎的核心接口和类型定义，提供支撑模块化、可扩展、高性能规则处理的基础API。

## 概述

RuleGo types包定义了构成规则引擎架构基础的契约接口和数据结构。这些接口实现了：

- **模块化设计**：引擎核心与组件之间的清晰分离
- **可扩展性**：便于集成自定义组件和处理器
- **高性能**：针对高吞吐量处理优化的数据结构
- **灵活性**：可配置的行为和处理模式

## 核心接口

### 引擎接口

- **`RuleEngine`** - 主要规则引擎接口，用于规则链执行
- **`RuleContext`** - 执行上下文，提供运行时信息和操作
- **`Pool`** - 规则引擎池接口，用于管理多个引擎实例

### 组件接口

- **`Node`** - 所有规则链组件的基础接口
- **`NodeCtx`** - 节点执行上下文接口
- **`ComponentRegistry`** - 组件注册和发现接口

### 消息处理

- **`RuleMsg`** - 核心消息接口，承载数据流经规则链
- **`Metadata`** - 消息元数据接口，提供键值对操作
- **`SharedData`** - 共享数据接口，具有写时复制语义

### 配置与DSL

- **`Config`** - 全局规则引擎配置接口
- **`Parser`** - 规则链DSL解析器接口
- **`RuleChain`** - 规则链定义接口
- **`RuleNode`** - 单个规则节点定义接口

### AOP与切面

- **`Aspect`** - 面向切面编程接口，用于横切关注点
- **`AspectManager`** - 切面生命周期管理接口
- **`BeforeAdvice`** - 前置执行通知接口
- **`AfterAdvice`** - 后置执行通知接口

### 端点系统

- **`Endpoint`** - 外部集成的输入/输出端点接口
- **`Router`** - 端点处理的消息路由接口
- **`Exchange`** - 端点与规则链之间的消息交换接口

### 监控与可观测性

- **`Logger`** - 结构化日志记录接口
- **`Cache`** - 性能优化的缓存接口
- **`Metrics`** - 监控的指标收集接口

## 关键类型定义

### 数据类型

```go
// 核心消息数据类型
type DataType string
const (
    JSON   DataType = "JSON"
    TEXT   DataType = "TEXT"
    BINARY DataType = "BINARY"
)

// 路由的消息类型
type MsgType string

// 节点连接的关系类型
type RelationType string
const (
    Success RelationType = "Success"
    Failure RelationType = "Failure"
    True    RelationType = "True"
    False   RelationType = "False"
)
```

### 配置类型

- **`Configuration`** - 通用配置映射
- **`ComponentConfiguration`** - 组件特定配置
- **`EngineOption`** - 引擎初始化选项
- **`EndpointOption`** - 端点配置选项

## 架构设计

### 分层架构

```
┌─────────────────────────────────────────┐
│              应用层                      │
├─────────────────────────────────────────┤
│              引擎层                      │
│  ┌─────────────┐  ┌─────────────────┐   │
│  │   引擎      │  │   规则链        │   │
│  └─────────────┘  └─────────────────┘   │
├─────────────────────────────────────────┤
│              组件层                      │
│  ┌─────────┐ ┌─────────┐ ┌─────────────┐│
│  │ 过滤器  │ │ 动作    │ │ 转换器      ││
│  └─────────┘ └─────────┘ └─────────────┘│
├─────────────────────────────────────────┤
│              类型层                      │
│         (接口与契约)                     │
└─────────────────────────────────────────┘
```

### 接口隔离

types包遵循接口隔离原则：

- **小而专注的接口** - 每个接口都有单一职责
- **可组合的契约** - 复杂行为由简单接口构建
- **可选功能** - 功能可以选择性实现

### 依赖倒置

- **引擎依赖接口，而非实现**
- **组件实现types中定义的接口**
- **配置驱动实现选择**

## 使用示例

### 实现自定义组件

```go
import "github.com/rulego/rulego/api/types"

type MyCustomNode struct {
    config MyConfiguration
}

func (n *MyCustomNode) Type() string {
    return "myCustom"
}

func (n *MyCustomNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
    return maps.Map2Struct(configuration, &n.config)
}

func (n *MyCustomNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
    // 处理消息
    processedData := n.processData(msg.GetData())
    
    // 更新消息
    msg.SetData(processedData)
    
    // 转发到下一个节点
    ctx.TellNext(msg, types.Success)
}

func (n *MyCustomNode) Destroy() {
    // 清理资源
}
```

### 创建自定义切面

```go
type LoggingAspect struct{}

func (a *LoggingAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
    return true // 应用于所有节点
}

func (a *LoggingAspect) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
    log.Printf("处理前: %s", msg.GetMsgType())
    return msg
}

func (a *LoggingAspect) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
    log.Printf("处理后: %s, 错误: %v", msg.GetMsgType(), err)
    return msg
}
```

## 设计原则

### 1. **接口优先设计**
所有主要功能都定义为接口，实现：
- 便于测试的模拟
- 多种实现方式
- 运行时行为切换

### 2. **零拷贝优化**
关键数据结构支持零拷贝操作：
- 具有写时复制语义的 `SharedData`
- 具有只读访问模式的 `Metadata`
- 无不必要拷贝的消息传递

### 3. **并发安全**
所有接口都假设并发使用：
- 线程安全的方法签名
- 适当使用原子操作
- 明确的所有权语义

### 4. **资源管理**
显式的生命周期管理：
- `Init()` 用于设置
- `Destroy()` 用于清理
- 基于上下文的取消

## 性能考虑

### 内存管理
- 高频分配的**对象池**接口
- 大数据传输的**零拷贝**语义
- 可选功能的**延迟初始化**模式

### 并发性
- 尽可能的**无锁**接口
- 计数器和标志的**原子操作**
- **基于通道**的通信模式

### 可扩展性
- 通过引擎池的**水平扩展**
- 通过优化数据结构的**垂直扩展**
- 通过配置接口的**资源限制**

## 扩展点

types包提供多个扩展点：

1. **自定义组件** - 实现 `Node` 接口
2. **自定义端点** - 实现 `Endpoint` 接口
3. **自定义切面** - 实现 `Aspect` 接口
4. **自定义解析器** - 实现 `Parser` 接口
5. **自定义缓存** - 实现 `Cache` 接口
6. **自定义日志器** - 实现 `Logger` 接口

---

有关实现示例和详细API文档，请查看此目录中的各个源文件。