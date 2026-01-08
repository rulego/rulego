/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package processor provides built-in processor implementations for the RuleGo endpoint system.
// Processors are functions that can be applied to message exchanges during endpoint processing,
// enabling data transformation, validation, and protocol-specific handling.
//
// Package processor 为 RuleGo 端点系统提供内置处理器实现。
// 处理器是在端点处理期间可以应用于消息交换的函数，支持数据转换、验证和协议特定处理。
//
// Processing Pipeline Integration:
// 处理管道集成：
//
// Processors are integrated into the endpoint processing pipeline at multiple levels:
// 处理器在多个级别集成到端点处理管道中：
//
//  1. Global Interceptors: Applied to all messages before routing (BaseEndpoint.Interceptors)
//     全局拦截器：在路由之前应用于所有消息（BaseEndpoint.Interceptors）
//
//  2. From Processing: Transform incoming data before target execution (From.processList)
//     From 处理：在目标执行前转换传入数据（From.processList）
//
//  3. To Processing: Handle results after target execution (To.processList)
//     To 处理：在目标执行后处理结果（To.processList）
//
// Built-in Processor Collections:
// 内置处理器集合：
//
//   - InBuiltins: Input processors for message preparation and transformation
//     InBuiltins：用于消息准备和转换的输入处理器
//
//   - OutBuiltins: Output processors for response formatting and delivery
//     OutBuiltins：用于响应格式化和传递的输出处理器
//
// Available Input Processors:
// 可用的输入处理器：
//
//   - headersToMetadata: Extracts HTTP headers into message metadata
//     headersToMetadata：将 HTTP 头提取到消息元数据
//
//   - setJsonDataType: Sets message data type to JSON and Content-Type header
//     setJsonDataType：设置消息数据类型为 JSON 并设置 Content-Type 头
//
//   - toHex: Converts binary data to hexadecimal string representation
//     toHex：将二进制数据转换为十六进制字符串表示
//
// Available Output Processors:
// 可用的输出处理器：
//
//   - responseToBody: Formats message data as HTTP response body
//     responseToBody：将消息数据格式化为 HTTP 响应正文
//
//   - metadataToHeaders: Maps message metadata to HTTP response headers
//     metadataToHeaders：将消息元数据映射到 HTTP 响应头
//
// Usage in Endpoint DSL:
// 在端点 DSL 中的使用：
//
// Processors can be referenced by name in endpoint DSL configuration:
// 处理器可以在端点 DSL 配置中通过名称引用：
//
//	{
//	  "routers": [{
//	    "from": {
//	      "path": "/api/data",
//	      "processors": ["headersToMetadata", "setJsonDataType"]
//	    },
//	    "to": {
//	      "path": "chain:dataProcessor",
//	      "processors": ["responseToBody", "metadataToHeaders"]
//	    }
//	  }]
//	}
//
// Custom Processor Development:
// 自定义处理器开发：
//
// Custom processors can be registered for specific use cases:
// 可以为特定用例注册自定义处理器：
//
//	InBuiltins.Register("customValidator", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
//		// Custom validation logic
//		msg := exchange.In.GetMsg()
//		if len(msg.GetData()) == 0 {
//			exchange.Out.SetError(errors.New("empty data"))
//			return false
//		}
//		return true
//	})
//
// Processor Function Signature:
// 处理器函数签名：
//
// All processors implement the endpoint.Process function signature:
// 所有处理器都实现 endpoint.Process 函数签名：
//
//	type Process func(router endpoint.Router, exchange *endpoint.Exchange) bool
//
// Return Value Semantics:
// 返回值语义：
//   - true: Continue processing pipeline  继续处理管道
//   - false: Stop processing pipeline  停止处理管道
package processor

import (
	"encoding/hex"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

const (
	// HeaderKeyContentType is the standard HTTP Content-Type header key.
	// Used for setting and reading the content type of HTTP messages.
	//
	// HeaderKeyContentType 是标准的 HTTP Content-Type 头键。
	// 用于设置和读取 HTTP 消息的内容类型。
	HeaderKeyContentType = "Content-Type"

	// HeaderValueApplicationJson is the MIME type for JSON content.
	// Used when setting Content-Type header for JSON responses.
	//
	// HeaderValueApplicationJson 是 JSON 内容的 MIME 类型。
	// 在为 JSON 响应设置 Content-Type 头时使用。
	HeaderValueApplicationJson        = "application/json"
	HeaderValueTextPlain              = "text/plain"
	HeaderValueApplicationOctetStream = "application/octet-stream"
	// KeyTopic is a metadata key used for storing message topic information.
	// Commonly used in messaging scenarios to identify the source topic.
	//
	// KeyTopic 是用于存储消息主题信息的元数据键。
	// 在消息传递场景中常用于标识源主题。
	KeyTopic = "topic"
)

// InBuiltins is a thread-safe collection of built-in input processors.
// These processors are designed to handle incoming message preparation,
// data transformation, and protocol-specific processing before target execution.
//
// InBuiltins 是内置输入处理器的线程安全集合。
// 这些处理器用于处理传入消息准备、数据转换和目标执行前的协议特定处理。
//
// Input processors are typically used in the From processing pipeline to:
// 输入处理器通常在 From 处理管道中用于：
//   - Extract and normalize protocol headers  提取和标准化协议头
//   - Set appropriate data types and content types  设置适当的数据类型和内容类型
//   - Convert data formats for rule engine consumption  转换数据格式供规则引擎使用
//   - Validate incoming message structure  验证传入消息结构
//
// Built-in Input Processors:
// 内置输入处理器：
//   - headersToMetadata: HTTP headers → message metadata  HTTP 头 → 消息元数据
//   - setJsonDataType: Set JSON data type and Content-Type  设置 JSON 数据类型和 Content-Type
//   - toHex: Binary data → hexadecimal string  二进制数据 → 十六进制字符串
var InBuiltins = builtins{}

// OutBuiltins is a thread-safe collection of built-in output processors.
// These processors are designed to handle response formatting, protocol-specific
// output preparation, and message delivery after rule chain execution.
//
// OutBuiltins 是内置输出处理器的线程安全集合。
// 这些处理器用于处理响应格式化、协议特定输出准备和规则链执行后的消息传递。
//
// Output processors are typically used in the To processing pipeline to:
// 输出处理器通常在 To 处理管道中用于：
//   - Format rule engine results for protocol responses  为协议响应格式化规则引擎结果
//   - Map message metadata to protocol headers  将消息元数据映射到协议头
//   - Handle error conditions and status codes  处理错误条件和状态码
//   - Prepare final response payload  准备最终响应负载
//
// Built-in Output Processors:
// 内置输出处理器：
//   - responseToBody: Message data → HTTP response body  消息数据 → HTTP 响应正文
//   - metadataToHeaders: Message metadata → HTTP headers  消息元数据 → HTTP 头
var OutBuiltins = builtins{}

// init registers all built-in processors during package initialization.
// This ensures that common processing functions are available immediately
// for use in endpoint configurations and DSL definitions.
//
// init 在包初始化期间注册所有内置处理器。
// 这确保通用处理函数立即可用于端点配置和 DSL 定义。
func init() {
	// Register input processor to extract HTTP headers into message metadata.
	// This enables rule chains to access HTTP header values as message metadata.
	//
	// 注册输入处理器将 HTTP 头提取到消息元数据。
	// 这使规则链能够将 HTTP 头值作为消息元数据访问。
	InBuiltins.Register("headersToMetadata", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
		return true
	})

	// Register input processor to set JSON data type and Content-Type.After setting, the rule chain component will process data based on that type
	// 注册输入处理器设置 JSON 数据类型和 Content-Type。设置后，规则链组件将根据该类型处理数据
	InBuiltins.Register("setJsonDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.JSON
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationJson)
		return true
	})
	// Register input processor to set text data type and Content-Type.After setting, the rule chain component will process data based on that type
	// 注册输入处理器设置文本数据类型和 Content-Type。设置后，规则链组件将根据该类型处理数据
	InBuiltins.Register("setTextDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.TEXT
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueTextPlain)
		return true
	})
	// Register input processor to set binary data type and Content. After setting, the rule chain component will process data based on that type
	// 注册输入处理器设置二进制数据类型和 Content-Type。设置后，规则链组件将根据该类型处理数据
	InBuiltins.Register("setBinaryDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.BINARY
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationOctetStream)
		return true
	})

	// Register input processor to convert binary message data to hexadecimal string.
	// 注册输入处理器将二进制消息数据转换为十六进制字符串。
	InBuiltins.Register("toHex", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		from := exchange.In.From()
		ruleMsg := types.NewMsg(0, from, types.TEXT, types.NewMetadata(), strings.ToUpper(hex.EncodeToString(exchange.In.Body())))
		ruleMsg.Metadata.PutValue(KeyTopic, from)
		exchange.In.SetMsg(&ruleMsg)
		return true
	})

	// Register output processor to format rule chain results as HTTP response body.
	// Handles both success cases (message data) and error cases (error messages).
	// Automatically sets Content-Type header for JSON responses.
	//
	// 注册输出处理器将规则链结果格式化为 HTTP 响应正文。
	// 处理成功情况（消息数据）和错误情况（错误消息）。
	// 自动为 JSON 响应设置 Content-Type 头。
	OutBuiltins.Register("responseToBody", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Lock()
		defer exchange.Unlock()
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			// 在响应中设置错误状态和正文。
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			// Set the response body with the message data.
			// 使用消息数据设置响应正文。
			if exchange.Out.GetMsg().DataType == types.JSON && exchange.Out.Headers().Get(HeaderKeyContentType) == "" {
				exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationJson)
			}
			exchange.Out.SetBody([]byte(exchange.Out.GetMsg().GetData()))
		}
		return true
	})

	// Register output processor to map message metadata to HTTP response headers.
	// This enables rule chains to set custom HTTP headers through message metadata.
	// Also handles error cases by setting appropriate status code and error body.
	//
	// 注册输出处理器将消息元数据映射到 HTTP 响应头。
	// 这使规则链能够通过消息元数据设置自定义 HTTP 头。
	// 也通过设置适当的状态码和错误正文处理错误情况。
	OutBuiltins.Register("metadataToHeaders", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Lock()
		defer exchange.Unlock()
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			// 在响应中设置错误状态和正文。
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			msg := exchange.Out.GetMsg()
			msg.Metadata.ForEach(func(k, v string) bool {
				if t, ok := exchange.Out.(endpoint.HeaderModifier); ok {
					t.SetHeader(k, v)
				} else {
					exchange.Out.Headers().Set(k, v)
				}
				return true
			})
		}
		return true
	})
}

// builtins is a thread-safe registry for processor functions that can be
// registered and retrieved by name. It provides the foundation for both
// InBuiltins and OutBuiltins collections.
//
// builtins 是可以按名称注册和检索的处理器函数的线程安全注册表。
// 它为 InBuiltins 和 OutBuiltins 集合提供基础。
//
// Registry Operations:
// 注册表操作：
//   - Register: Add single processor  注册：添加单个处理器
//   - RegisterAll: Add multiple processors  注册所有：添加多个处理器
//   - Unregister: Remove processors by name  注销：按名称删除处理器
//   - Get: Retrieve processor by name  获取：按名称检索处理器
//   - Names: List all registered names  名称：列出所有注册的名称
type builtins struct {
	processors map[string]endpoint.Process // Map of processor functions  处理器函数映射
	lock       sync.RWMutex                // Read/Write mutex for concurrent access  用于并发访问的读写互斥锁
}

// Register adds a single processor function to the registry with the specified name.
// If a processor with the same name already exists, it will be replaced.
//
// Register 使用指定名称将单个处理器函数添加到注册表。
// 如果已存在同名处理器，它将被替换。
//
// Parameters:
// 参数：
//   - name: Unique identifier for the processor  处理器的唯一标识符
//   - processor: Function implementing the processor logic  实现处理器逻辑的函数
//
// Usage:
// 使用：
//
//	InBuiltins.Register("myProcessor", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
//		// Custom processing logic
//		return true
//	})
func (b *builtins) Register(name string, processor endpoint.Process) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.processors == nil {
		b.processors = make(map[string]endpoint.Process)
	}
	b.processors[name] = processor
}

// RegisterAll adds multiple processor functions to the registry at once.
// This is more efficient than calling Register multiple times when adding
// many processors simultaneously.
//
// RegisterAll 一次性将多个处理器函数添加到注册表。
// 当同时添加多个处理器时，这比多次调用 Register 更高效。
//
// Parameters:
// 参数：
//   - processors: Map of processor names to their implementations
//     processors：处理器名称到其实现的映射
func (b *builtins) RegisterAll(processors map[string]endpoint.Process) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.processors == nil {
		b.processors = make(map[string]endpoint.Process)
	}
	for k, v := range processors {
		b.processors[k] = v
	}
}

// Unregister removes one or more processor functions from the registry by their names.
// Non-existent processor names are silently ignored.
//
// Unregister 通过名称从注册表中删除一个或多个处理器函数。
// 不存在的处理器名称会被静默忽略。
//
// Parameters:
// 参数：
//   - names: Variable number of processor names to remove
//     names：要删除的处理器名称的可变数量
//
// Thread Safety:
// 线程安全：
// This method is thread-safe and can be called concurrently.
// 此方法是线程安全的，可以并发调用。
//
// Usage:
// 使用：
//
//	InBuiltins.Unregister("processor1", "processor2")
func (b *builtins) Unregister(names ...string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, name := range names {
		delete(b.processors, name)
	}
}

// Get retrieves a processor function by its name from the registry.
// Returns the processor function and a boolean indicating whether it was found.
//
// Get 通过名称从注册表中检索处理器函数。
// 返回处理器函数和一个布尔值，指示是否找到。
//
// Parameters:
// 参数：
//   - name: The name of the processor to retrieve
//     name：要检索的处理器名称
//
// Returns:
// 返回：
//   - endpoint.Process: The processor function if found  如果找到则返回处理器函数
//   - bool: True if processor exists, false otherwise  如果处理器存在则为 true，否则为 false
//
// Usage:
// 使用：
//
//	if processor, exists := InBuiltins.Get("headersToMetadata"); exists {
//		// Use the processor
//	}
func (b *builtins) Get(name string) (endpoint.Process, bool) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	p, ok := b.processors[name]
	return p, ok
}

// Names returns a list of all registered processor names.
// The returned slice is a copy and can be safely modified without affecting the registry.
//
// Names 返回所有注册处理器名称的列表。
// 返回的切片是副本，可以安全修改而不影响注册表。
//
// Returns:
// 返回：
//   - []string: List of all registered processor names  所有注册处理器名称的列表
//
// Usage:
// 使用：
//
//	names := InBuiltins.Names()
//	fmt.Printf("Available processors: %v", names)
func (b *builtins) Names() []string {
	b.lock.RLock()
	defer b.lock.RUnlock()
	var keys = make([]string, 0, len(b.processors))
	for k := range b.processors {
		keys = append(keys, k)
	}
	return keys
}
