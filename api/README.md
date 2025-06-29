# Types API

------

This directory contains the core interfaces and type definitions for the RuleGo rule engine. It provides the foundational APIs that enable modular, extensible, and high-performance rule processing.

## Overview

The RuleGo types package defines the contract interfaces and data structures that form the backbone of the rule engine architecture. These interfaces enable:

- **Modular Design**: Clean separation between engine core and components
- **Extensibility**: Easy integration of custom components and processors
- **Performance**: Optimized data structures for high-throughput processing
- **Flexibility**: Configurable behaviors and processing patterns

## Core Interfaces

### Engine Interfaces

- **`RuleEngine`** - Main rule engine interface for rule chain execution
- **`RuleContext`** - Execution context providing runtime information and operations
- **`Pool`** - Rule engine pool interface for managing multiple engine instances

### Component Interfaces

- **`Node`** - Base interface for all rule chain components
- **`NodeCtx`** - Node execution context interface
- **`ComponentRegistry`** - Component registration and discovery interface

### Message Processing

- **`RuleMsg`** - Core message interface carrying data through rule chains
- **`Metadata`** - Message metadata interface with key-value operations
- **`SharedData`** - Shared data interface with copy-on-write semantics

### Configuration & DSL

- **`Config`** - Global rule engine configuration interface
- **`Parser`** - Rule chain DSL parser interface
- **`RuleChain`** - Rule chain definition interface
- **`RuleNode`** - Individual rule node definition interface

### AOP & Aspects

- **`Aspect`** - Aspect-oriented programming interface for cross-cutting concerns
- **`AspectManager`** - Aspect lifecycle management interface
- **`BeforeAdvice`** - Before execution advice interface
- **`AfterAdvice`** - After execution advice interface

### Endpoint System

- **`Endpoint`** - Input/output endpoint interface for external integrations
- **`Router`** - Message routing interface for endpoint processing
- **`Exchange`** - Message exchange interface between endpoints and rule chains

### Monitoring & Observability

- **`Logger`** - Logging interface for structured logging
- **`Cache`** - Caching interface for performance optimization
- **`Metrics`** - Metrics collection interface for monitoring

## Key Type Definitions

### Data Types

```go
// Core message data types
type DataType string
const (
    JSON   DataType = "JSON"
    TEXT   DataType = "TEXT"
    BINARY DataType = "BINARY"
)

// Message types for routing
type MsgType string

// Relation types for node connections
type RelationType string
const (
    Success RelationType = "Success"
    Failure RelationType = "Failure"
    True    RelationType = "True"
    False   RelationType = "False"
)
```

### Configuration Types

- **`Configuration`** - Generic configuration map
- **`ComponentConfiguration`** - Component-specific configuration
- **`EngineOption`** - Engine initialization options
- **`EndpointOption`** - Endpoint configuration options

## Architecture Design

### Layered Architecture

```
┌─────────────────────────────────────────┐
│            Application Layer            │
├─────────────────────────────────────────┤
│              Engine Layer               │
│  ┌─────────────┐  ┌─────────────────┐   │
│  │   Engine    │  │   Rule Chains   │   │
│  └─────────────┘  └─────────────────┘   │
├─────────────────────────────────────────┤
│             Component Layer             │
│  ┌─────────┐ ┌─────────┐ ┌─────────────┐│
│  │ Filters │ │ Actions │ │ Transforms  ││
│  └─────────┘ └─────────┘ └─────────────┘│
├─────────────────────────────────────────┤
│              Types Layer                │
│        (Interfaces & Contracts)         │
└─────────────────────────────────────────┘
```

### Interface Segregation

The types package follows the Interface Segregation Principle:

- **Small, focused interfaces** - Each interface has a single responsibility
- **Composable contracts** - Complex behaviors built from simple interfaces
- **Optional capabilities** - Features can be optionally implemented

### Dependency Inversion

- **Engine depends on interfaces, not implementations**
- **Components implement interfaces defined in types**
- **Configuration drives implementation selection**

## Usage Examples

### Implementing a Custom Component

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
    // Process message
    processedData := n.processData(msg.GetData())
    
    // Update message
    msg.SetData(processedData)
    
    // Forward to next node
    ctx.TellNext(msg, types.Success)
}

func (n *MyCustomNode) Destroy() {
    // Cleanup resources
}
```

### Creating Custom Aspects

```go
type LoggingAspect struct{}

func (a *LoggingAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
    return true // Apply to all nodes
}

func (a *LoggingAspect) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
    log.Printf("Before processing: %s", msg.GetMsgType())
    return msg
}

func (a *LoggingAspect) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
    log.Printf("After processing: %s, error: %v", msg.GetMsgType(), err)
    return msg
}
```

## Design Principles

### 1. **Interface-First Design**
All major functionalities are defined as interfaces, enabling:
- Easy mocking for testing
- Multiple implementations
- Runtime behavior switching

### 2. **Zero-Copy Optimizations**
Key data structures support zero-copy operations:
- `SharedData` with copy-on-write semantics
- `Metadata` with read-only access patterns
- Message passing without unnecessary copying

### 3. **Concurrent Safety**
All interfaces assume concurrent usage:
- Thread-safe method signatures
- Atomic operations where appropriate
- Clear ownership semantics

### 4. **Resource Management**
Explicit lifecycle management:
- `Init()` for setup
- `Destroy()` for cleanup
- Context-based cancellation

## Performance Considerations

### Memory Management
- **Object pooling** interfaces for high-frequency allocations
- **Zero-copy** semantics for large data transfers
- **Lazy initialization** patterns for optional features

### Concurrency
- **Lock-free** interfaces where possible
- **Atomic operations** for counters and flags
- **Channel-based** communication patterns

### Scalability
- **Horizontal scaling** through engine pooling
- **Vertical scaling** through optimized data structures
- **Resource limits** through configuration interfaces

## Extension Points

The types package provides multiple extension points:

1. **Custom Components** - Implement `Node` interface
2. **Custom Endpoints** - Implement `Endpoint` interface
3. **Custom Aspects** - Implement `Aspect` interface
4. **Custom Parsers** - Implement `Parser` interface
5. **Custom Caches** - Implement `Cache` interface
6. **Custom Loggers** - Implement `Logger` interface

---

For implementation examples and detailed API documentation, see the individual source files in this directory.