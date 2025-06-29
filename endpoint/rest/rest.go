/*
 * Copyright 2023 The RuleGo Authors.
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

// Package rest provides an HTTP/REST endpoint implementation for the RuleGo framework.
// It enables creating HTTP servers that can receive, process, and respond to HTTP requests,
// routing them to appropriate rule chains or components for business logic processing.
//
// Package rest 为 RuleGo 框架提供 HTTP/REST 端点实现。
// 它支持创建 HTTP 服务器，可以接收、处理和响应 HTTP 请求，
// 将它们路由到适当的规则链或组件进行业务逻辑处理。
//
// Key Features / 主要特性：
//
// • HTTP Server Management: Complete HTTP server lifecycle management  HTTP 服务器管理：完整的 HTTP 服务器生命周期管理
// • Dynamic Routing: Runtime addition/removal of HTTP routes  动态路由：运行时添加/删除 HTTP 路由
// • Method Support: All standard HTTP methods (GET, POST, PUT, DELETE, etc.)  方法支持：所有标准 HTTP 方法
// • Path Parameters: URL path parameter extraction and processing  路径参数：URL 路径参数提取和处理
// • CORS Support: Cross-Origin Resource Sharing configuration  CORS 支持：跨域资源共享配置
// • SSL/TLS Support: HTTPS server with certificate configuration  SSL/TLS 支持：带证书配置的 HTTPS 服务器
// • Static File Serving: Built-in static file serving capabilities  静态文件服务：内置静态文件服务功能
// • Shared Server: Multiple endpoint instances can share the same server  共享服务器：多个端点实例可以共享同一服务器
//
// Architecture / 架构：
//
// The REST endpoint follows a message-based processing model:
// REST 端点遵循基于消息的处理模型：
//
// 1. HTTP Request → RequestMessage conversion  HTTP 请求 → RequestMessage 转换
// 2. RequestMessage → Rule Chain/Component processing  RequestMessage → 规则链/组件处理
// 3. Processing Result → ResponseMessage  处理结果 → ResponseMessage
// 4. ResponseMessage → HTTP Response  ResponseMessage → HTTP 响应
//
// Initialization Methods / 初始化方法：
//
// The REST endpoint supports three initialization approaches:
// REST 端点支持三种初始化方法：
//
// 1. Registry-based Initialization / 基于注册表的初始化：
//
//	import "github.com/rulego/rulego/endpoint"
//
//	config := types.Configuration{
//	    "server": ":8080",
//	    "allowCors": true,
//	    "readTimeout": 10,
//	    "writeTimeout": 10,
//	}
//
//	// Create endpoint through registry
//	// 通过注册表创建端点
//	endpoint, err := endpoint.Registry.New(rest.Type, ruleConfig, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add router and start
//	// 添加路由器并启动
//	router := endpoint.NewRouter().
//	    From("/api/device/{deviceId}").
//	    To("chain:deviceProcessing")
//
//	endpoint.AddRouter(router, "POST")
//	endpoint.Start()
//
// 2. Dynamic DSL Initialization / 动态 DSL 初始化：
//
//	dslConfig := `{
//	  "id": "http-endpoint",
//	  "type": "endpoint/http",
//	  "name": "HTTP API Server",
//	  "configuration": {
//	    "server": ":8080",
//	    "allowCors": true,
//	    "readTimeout": 10,
//	    "writeTimeout": 10
//	  },
//	  "routers": [
//	    {
//	      "id": "device-api",
//	      "params": ["POST"],
//	      "from": {
//	        "path": "/api/device/{deviceId}"
//	      },
//	      "to": {
//	        "path": "chain:deviceProcessing"
//	      }
//	    }
//	  ]
//	}`
//
//	// Create endpoint from DSL
//	// 从 DSL 创建端点
//	endpoint, err := endpoint.NewFromDsl([]byte(dslConfig))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	endpoint.Start()
//
// 3. Direct Instantiation with Fluent API / 直接实例化和流畅 API：
//
//	config := &rest.Config{
//	    Server: ":8080",
//	    AllowCors: true,
//	}
//
//	endpoint := &rest.Rest{}
//	err := endpoint.Init(ruleConfig, config)
//
//	// Using fluent API for different HTTP methods
//	// 使用流畅 API 处理不同的 HTTP 方法
//	endpoint.POST(
//	    endpoint.NewRouter().From("/api/users").To("chain:createUser"),
//	).GET(
//	    endpoint.NewRouter().From("/api/users/{id}").To("chain:getUser"),
//	).PUT(
//	    endpoint.NewRouter().From("/api/users/{id}").To("chain:updateUser"),
//	).DELETE(
//	    endpoint.NewRouter().From("/api/users/{id}").To("chain:deleteUser"),
//	)
//
//	endpoint.Start()
//
// Route Path Patterns / 路由路径模式：
//
// The endpoint supports httprouter-style path patterns:
// 端点支持 httprouter 风格的路径模式：
//
// • Static paths: "/api/users"  静态路径
// • Named parameters: "/api/users/{id}"  命名参数
// • Catch-all parameters: "/api/files/*filepath"  通配符参数
package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	nodeBase "github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/runtime"
	"github.com/rulego/rulego/utils/str"
)

// Constants for HTTP headers and content types used throughout the REST endpoint.
// These constants ensure consistency and reduce magic strings in the codebase.
// 用于 REST 端点的 HTTP 头和内容类型常量。
// 这些常量确保一致性并减少代码中的魔法字符串。
const (
	ContentTypeKey                      = "Content-Type"
	JsonContextType                     = "application/json"
	HeaderKeyAccessControlRequestMethod = "Access-Control-Request-Method"
	HeaderKeyAccessControlAllowMethods  = "Access-Control-Allow-Methods"
	HeaderKeyAccessControlAllowHeaders  = "Access-Control-Allow-Headers"
	HeaderKeyAccessControlAllowOrigin   = "Access-Control-Allow-Origin"
	HeaderValueAll                      = "*"
)

// Type defines the component type identifier for the REST endpoint.
// This identifier is used for component registration and DSL configuration.
// Type 定义 REST 端点的组件类型标识符。
// 此标识符用于组件注册和 DSL 配置。
const Type = types.EndpointTypePrefix + "http"

// Endpoint is an alias for Rest to provide backward compatibility.
// This allows users to reference the component using either name.
// Endpoint 是 Rest 的别名，提供向后兼容性。
// 这允许用户使用任一名称引用组件。
type Endpoint = Rest

var _ endpoint.Endpoint = (*Endpoint)(nil)
var _ endpoint.HttpEndpoint = (*Endpoint)(nil)

// RequestMessage represents an incoming HTTP request message in the RuleGo processing pipeline.
// It encapsulates all the necessary information from an HTTP request and provides methods
// to access request data, headers, parameters, and convert the request into a RuleMsg.
//
// RequestMessage 表示 RuleGo 处理管道中的传入 HTTP 请求消息。
// 它封装了 HTTP 请求的所有必要信息，并提供方法来访问请求数据、头部、参数，
// 并将请求转换为 RuleMsg。
//
// Key Features / 主要特性：
// • HTTP Request Wrapping: Provides a unified interface for HTTP request data  HTTP 请求包装：为 HTTP 请求数据提供统一接口
// • Lazy Body Reading: Body is read only when accessed to optimize performance  延迟体读取：仅在访问时读取体以优化性能
// • Parameter Extraction: Supports both path and query parameters  参数提取：支持路径和查询参数
// • Automatic Content Type Detection: Determines data type based on Content-Type header  自动内容类型检测：基于 Content-Type 头确定数据类型
// • Metadata Integration: Seamlessly integrates with RuleGo's metadata system  元数据集成：与 RuleGo 元数据系统无缝集成
//
// Message Flow / 消息流：
// 1. HTTP request received by server  服务器接收 HTTP 请求
// 2. RequestMessage created with request context  使用请求上下文创建 RequestMessage
// 3. Body read and cached on first access  首次访问时读取并缓存正文
// 4. Converted to RuleMsg for rule chain processing  转换为 RuleMsg 进行规则链处理
type RequestMessage struct {
	//HTTP 请求对象，包含所有请求信息  HTTP request object containing all request information  HTTP 请求对象
	request *http.Request
	//HTTP 响应写入器，用于写入响应数据  HTTP response writer for writing response data  HTTP 响应写入器
	response http.ResponseWriter
	//请求体数据，延迟读取以优化性能  Request body data, lazily loaded for performance  请求体数据
	body []byte
	//路径参数，从 URL 路径中提取的命名参数  Path parameters extracted from URL path  路径参数
	Params httprouter.Params
	//转换后的规则消息，缓存以避免重复转换  Converted rule message, cached to avoid re-conversion  转换后的规则消息
	msg *types.RuleMsg
	//处理过程中的错误信息  Error information during processing  处理错误信息
	err error
	//消息元数据，用于存储额外的键值对信息  Message metadata for storing additional key-value information  消息元数据
	Metadata *types.Metadata
}

// Body returns the HTTP request body as a byte slice.
// The body is read lazily on the first call and cached for subsequent calls.
// This approach optimizes performance by avoiding unnecessary I/O operations.
//
// Body 返回 HTTP 请求体作为字节切片。
// 首次调用时延迟读取正文并缓存以供后续调用。
// 这种方法通过避免不必要的 I/O 操作来优化性能。
//
// Returns / 返回：
// • []byte: The request body content, empty slice if no body or error  请求体内容，如果没有正文或错误则为空切片
//
// Note: The request body stream is automatically closed after reading
// 注意：读取后请求体流会自动关闭
func (r *RequestMessage) Body() []byte {
	if r.body == nil && r.request != nil {
		defer func() {
			if r.request.Body != nil {
				_ = r.request.Body.Close()
			}
		}()
		entry, _ := io.ReadAll(r.request.Body)
		r.body = entry
	}
	return r.body
}

// Headers returns the HTTP request headers as a textproto.MIMEHeader.
// This provides access to all HTTP headers in a standardized format.
//
// Headers 返回 HTTP 请求头作为 textproto.MIMEHeader。
// 这提供了以标准化格式访问所有 HTTP 头的能力。
//
// Returns / 返回：
// • textproto.MIMEHeader: HTTP headers map, nil if no request  HTTP 头映射，如果没有请求则为 nil
func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.request == nil {
		return nil
	}
	return textproto.MIMEHeader(r.request.Header)
}

// From returns the complete request URL as a string.
// This is used for routing and logging purposes.
//
// From 返回完整的请求 URL 作为字符串。
// 用于路由和日志记录目的。
//
// Returns / 返回：
// • string: Complete request URL, empty string if no request  完整请求 URL，如果没有请求则为空字符串
func (r RequestMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.URL.String()
}

// GetParam retrieves a parameter value by key from path parameters or query parameters.
// It first checks path parameters (URL segments), then falls back to query parameters.
// This provides a unified way to access all types of HTTP parameters.
//
// GetParam 通过键从路径参数或查询参数中检索参数值。
// 它首先检查路径参数（URL 段），然后回退到查询参数。
// 这提供了访问所有类型 HTTP 参数的统一方式。
//
// Parameters / 参数：
// • key: Parameter name to retrieve  要检索的参数名称
//
// Returns / 返回：
// • string: Parameter value, empty string if not found  参数值，如果未找到则为空字符串
//
// Priority Order / 优先级顺序：
// 1. Path parameters (e.g., /users/{id})  路径参数
// 2. Query parameters (e.g., ?name=value)  查询参数
func (r *RequestMessage) GetParam(key string) string {
	if r.request == nil {
		return ""
	}
	if v := r.Params.ByName(key); v == "" {
		return r.request.FormValue(key)
	} else {
		return v
	}
}

// SetMsg sets the RuleMsg for this request message.
// This is typically used during message processing to cache the converted message.
//
// SetMsg 为此请求消息设置 RuleMsg。
// 这通常在消息处理期间用于缓存转换后的消息。
func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

// GetMsg converts the HTTP request to a RuleMsg for rule chain processing.
// The conversion includes automatic data type detection and metadata population.
//
// GetMsg 将 HTTP 请求转换为 RuleMsg 以进行规则链处理。
// 转换包括自动数据类型检测和元数据填充。
//
// Returns / 返回：
// • *types.RuleMsg: Converted rule message ready for processing  转换后的规则消息，可供处理
//
// Conversion Logic / 转换逻辑：
// • GET requests: Query parameters as JSON data  GET 请求：查询参数作为 JSON 数据
// • Other methods: Request body as data  其他方法：请求体作为数据
// • Content-Type detection: JSON vs TEXT based on Content-Type header  内容类型检测：基于 Content-Type 头的 JSON vs TEXT
// • Metadata: Additional request information  元数据：额外的请求信息
func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		dataType := types.TEXT
		var data string
		if r.request != nil && r.request.Method == http.MethodGet {
			dataType = types.JSON
			data = str.ToString(r.request.URL.Query())
		} else {
			if contentType := r.Headers().Get(ContentTypeKey); strings.HasPrefix(contentType, JsonContextType) {
				dataType = types.JSON
			}
			data = string(r.Body())
		}
		if r.Metadata == nil {
			r.Metadata = types.NewMetadata()
		}
		ruleMsg := types.NewMsg(0, r.From(), dataType, r.Metadata, data)
		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode is a no-op for request messages as status codes are set on responses.
// This method exists to satisfy the Message interface.
//
// SetStatusCode 对于请求消息是无操作，因为状态码在响应上设置。
// 此方法存在是为了满足 Message 接口。
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

// SetBody sets the request body content.
// This is typically used for testing or message transformation scenarios.
//
// SetBody 设置请求体内容。
// 这通常用于测试或消息转换场景。
func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

// SetError sets an error associated with this request message.
// This is used to track errors during request processing.
//
// SetError 设置与此请求消息关联的错误。
// 用于跟踪请求处理期间的错误。
func (r *RequestMessage) SetError(err error) {
	r.err = err
}

// GetError returns any error associated with this request message.
//
// GetError 返回与此请求消息关联的任何错误。
func (r *RequestMessage) GetError() error {
	return r.err
}

// Request returns the underlying HTTP request object.
// This provides direct access to the original HTTP request for advanced scenarios.
//
// Request 返回底层的 HTTP 请求对象。
// 这为高级场景提供对原始 HTTP 请求的直接访问。
func (r *RequestMessage) Request() *http.Request {
	return r.request
}

// Response returns the HTTP response writer.
// This allows direct writing to the HTTP response if needed.
//
// Response 返回 HTTP 响应写入器。
// 如果需要，这允许直接写入 HTTP 响应。
func (r *RequestMessage) Response() http.ResponseWriter {
	return r.response
}

// ResponseMessage represents an outgoing HTTP response message in the RuleGo processing pipeline.
// It handles the conversion of rule processing results back into HTTP responses,
// including status codes, headers, and response body content.
//
// ResponseMessage 表示 RuleGo 处理管道中的传出 HTTP 响应消息。
// 它处理规则处理结果转换回 HTTP 响应，包括状态码、头部和响应体内容。
//
// Thread Safety / 线程安全：
// ResponseMessage is thread-safe and can be safely accessed from multiple goroutines.
// All write operations are protected by a mutex to prevent race conditions.
// ResponseMessage 是线程安全的，可以安全地从多个协程访问。
// 所有写操作都受互斥锁保护以防止竞态条件。
//
// Key Features / 主要特性：
// • Thread-Safe Operations: All methods are protected by mutex for concurrent access  线程安全操作：所有方法都受互斥锁保护以支持并发访问
// • Automatic Response Writing: Body content is automatically written to HTTP response  自动响应写入：正文内容自动写入 HTTP 响应
// • Status Code Management: Support for HTTP status code setting  状态码管理：支持 HTTP 状态码设置
// • Header Management: Access to HTTP response headers  头部管理：访问 HTTP 响应头
// • Error Handling: Built-in error tracking and reporting  错误处理：内置错误跟踪和报告
type ResponseMessage struct {
	//原始 HTTP 请求对象  Original HTTP request object  原始 HTTP 请求对象
	request *http.Request
	//HTTP 响应写入器  HTTP response writer  HTTP 响应写入器
	response http.ResponseWriter
	//响应体数据  Response body data  响应体数据
	body []byte
	//目标路径或标识符  Target path or identifier  目标路径
	to string
	//处理结果的规则消息  Rule message with processing results  处理结果的规则消息
	msg *types.RuleMsg
	//响应处理过程中的错误  Error during response processing  响应处理错误
	err error
	//保护并发访问的互斥锁  Mutex protecting concurrent access  并发访问保护锁
	mu sync.RWMutex
}

// Body returns the response body content in a thread-safe manner.
// This method is used to retrieve the current response body data.
//
// Body 以线程安全的方式返回响应体内容。
// 此方法用于检索当前响应体数据。
//
// Returns / 返回：
// • []byte: Current response body content  当前响应体内容
func (r *ResponseMessage) Body() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.body
}

// Headers returns the HTTP response headers in a thread-safe manner.
// This provides access to the response headers for reading or modification.
//
// Headers 以线程安全的方式返回 HTTP 响应头。
// 这提供对响应头的访问以进行读取或修改。
//
// Returns / 返回：
// • textproto.MIMEHeader: Response headers map, nil if no response writer  响应头映射，如果没有响应写入器则为 nil
func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.response == nil {
		return nil
	}
	return textproto.MIMEHeader(r.response.Header())
}

// From returns the original request URL for context.
// This is useful for logging and debugging purposes.
//
// From 返回原始请求 URL 作为上下文。
// 这对于日志记录和调试很有用。
//
// Returns / 返回：
// • string: Original request URL, empty if no request  原始请求 URL，如果没有请求则为空
func (r *ResponseMessage) From() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.request == nil {
		return ""
	}
	return r.request.URL.String()
}

// GetParam retrieves a parameter from the original request.
// This provides access to request parameters for response processing.
//
// GetParam 从原始请求中检索参数。
// 这为响应处理提供对请求参数的访问。
//
// Parameters / 参数：
// • key: Parameter name to retrieve  要检索的参数名称
//
// Returns / 返回：
// • string: Parameter value, empty if not found or no request  参数值，如果未找到或没有请求则为空
func (r *ResponseMessage) GetParam(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.request == nil {
		return ""
	}
	return r.request.FormValue(key)
}

// SetMsg sets the rule message for this response in a thread-safe manner.
// This is typically called during rule processing to set the processing result.
//
// SetMsg 以线程安全的方式为此响应设置规则消息。
// 这通常在规则处理期间调用以设置处理结果。
//
// Parameters / 参数：
// • msg: The rule message containing processing results  包含处理结果的规则消息
func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}

// GetMsg returns the rule message associated with this response.
// This provides access to the processing results for response generation.
//
// GetMsg 返回与此响应关联的规则消息。
// 这为响应生成提供对处理结果的访问。
//
// Returns / 返回：
// • *types.RuleMsg: The rule message with processing results  包含处理结果的规则消息
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

// SetStatusCode sets the HTTP response status code.
// The status code is immediately written to the HTTP response.
//
// SetStatusCode 设置 HTTP 响应状态码。
// 状态码立即写入 HTTP 响应。
//
// Parameters / 参数：
// • statusCode: HTTP status code to set (e.g., 200, 404, 500)  要设置的 HTTP 状态码
//
// Note: This should be called before SetBody to ensure proper HTTP response format
// 注意：应在 SetBody 之前调用以确保正确的 HTTP 响应格式
func (r *ResponseMessage) SetStatusCode(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.response != nil {
		r.response.WriteHeader(statusCode)
	}
}

// SetBody sets the response body content and immediately writes it to the HTTP response.
// This method combines both storing the body content and sending it to the client.
//
// SetBody 设置响应体内容并立即将其写入 HTTP 响应。
// 此方法结合了存储正文内容和将其发送给客户端。
//
// Parameters / 参数：
// • body: Response body content to set and send  要设置和发送的响应体内容
//
// Behavior / 行为：
// 1. Store body content internally  内部存储正文内容
// 2. Write body to HTTP response writer  将正文写入 HTTP 响应写入器
// 3. Handle any write errors  处理任何写入错误
//
// Thread Safety / 线程安全：
// This method is thread-safe and can be called concurrently
// 此方法是线程安全的，可以并发调用
func (r *ResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.body = body
	if r.response != nil {
		_, _ = r.response.Write(body)
	}
}

// SetError sets an error associated with this response message.
// This is used for error tracking and debugging purposes.
//
// SetError 设置与此响应消息关联的错误。
// 用于错误跟踪和调试目的。
//
// Parameters / 参数：
// • err: Error to associate with this response  要与此响应关联的错误
func (r *ResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// GetError returns any error associated with this response message.
// This is useful for error handling and debugging.
//
// GetError 返回与此响应消息关联的任何错误。
// 这对于错误处理和调试很有用。
//
// Returns / 返回：
// • error: Associated error, nil if no error  关联的错误，如果没有错误则为 nil
func (r *ResponseMessage) GetError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

func (r *ResponseMessage) Request() *http.Request {
	return r.request
}

func (r *ResponseMessage) Response() http.ResponseWriter {
	return r.response
}

// Config defines the configuration structure for the REST endpoint server.
// It contains all necessary settings for HTTP server initialization and behavior control.
//
// Config 定义 REST 端点服务器的配置结构。
// 它包含 HTTP 服务器初始化和行为控制的所有必要设置。
//
// Configuration Categories / 配置类别：
//
// Server Settings / 服务器设置：
// • Server address and port binding  服务器地址和端口绑定
// • SSL/TLS certificate configuration  SSL/TLS 证书配置
// • Cross-Origin Resource Sharing (CORS) settings  跨域资源共享设置
//
// Performance Tuning / 性能调优：
// • Connection timeout configurations  连接超时配置
// • Keep-alive connection management  Keep-alive 连接管理
// • Resource optimization settings  资源优化设置
//
// Security Features / 安全功能：
// • HTTPS support with certificate files  使用证书文件的 HTTPS 支持
// • CORS policy enforcement  CORS 策略执行
// • Connection timeout protection  连接超时保护
type Config struct {
	// Server specifies the server address and port to bind to.
	// Format: "host:port" or ":port" for all interfaces.
	// Examples: ":8080", "localhost:9090", "0.0.0.0:3000"
	// Server 指定要绑定的服务器地址和端口。
	// 格式："host:port" 或 ":port" 用于所有接口。
	// 示例：":8080"、"localhost:9090"、"0.0.0.0:3000"
	Server string `json:"server"`

	// CertFile specifies the path to the SSL/TLS certificate file for HTTPS.
	// When both CertFile and CertKeyFile are provided, the server runs in HTTPS mode.
	// CertFile 指定 HTTPS 的 SSL/TLS 证书文件路径。
	// 当提供 CertFile 和 CertKeyFile 时，服务器以 HTTPS 模式运行。
	CertFile string `json:"certFile"`

	// CertKeyFile specifies the path to the SSL/TLS private key file for HTTPS.
	// This file must correspond to the certificate specified in CertFile.
	// CertKeyFile 指定 HTTPS 的 SSL/TLS 私钥文件路径。
	// 此文件必须与 CertFile 中指定的证书对应。
	CertKeyFile string `json:"certKeyFile"`

	// AllowCors enables Cross-Origin Resource Sharing (CORS) support.
	// When true, the server allows cross-origin requests from web browsers.
	// This is useful for API servers that need to be accessed from web applications.
	// AllowCors 启用跨域资源共享（CORS）支持。
	// 当为 true 时，服务器允许来自 Web 浏览器的跨域请求。
	// 这对于需要从 Web 应用程序访问的 API 服务器很有用。
	AllowCors bool

	// ReadTimeout sets the maximum duration for reading the entire request, including the body.
	// Specified in seconds. A value of 0 uses the default timeout of 10 seconds.
	// This prevents slow or malicious clients from holding connections indefinitely.
	// ReadTimeout 设置读取整个请求（包括正文）的最大持续时间。
	// 以秒为单位指定。值为 0 时使用默认超时 10 秒。
	// 这防止缓慢或恶意客户端无限期保持连接。
	ReadTimeout int `json:"readTimeout"`

	// WriteTimeout sets the maximum duration before timing out writes of the response.
	// Specified in seconds. A value of 0 uses the default timeout of 10 seconds.
	// This ensures timely response delivery and prevents resource exhaustion.
	// WriteTimeout 设置响应写入超时前的最大持续时间。
	// 以秒为单位指定。值为 0 时使用默认超时 10 秒。
	// 这确保及时的响应传递并防止资源耗尽。
	WriteTimeout int `json:"writeTimeout"`

	// IdleTimeout sets the maximum amount of time to wait for the next request
	// when keep-alives are enabled. Specified in seconds.
	// A value of 0 uses the default timeout of 60 seconds.
	// IdleTimeout 设置启用 keep-alive 时等待下一个请求的最大时间。
	// 以秒为单位指定。值为 0 时使用默认超时 60 秒。
	IdleTimeout int `json:"idleTimeout"`

	// DisableKeepalive disables HTTP keep-alive connections.
	// When true, each request uses a new connection, which may impact performance
	// but can be useful for certain deployment scenarios or debugging.
	// DisableKeepalive 禁用 HTTP keep-alive 连接。
	// 当为 true 时，每个请求使用新连接，这可能影响性能，
	// 但对于某些部署场景或调试可能有用。
	DisableKeepalive bool `json:"disableKeepalive"`
}

// Rest represents an HTTP/REST endpoint implementation for the RuleGo framework.
// It provides a complete HTTP server solution with dynamic routing, request processing,
// and integration with RuleGo's rule chains and components.
//
// Rest 表示 RuleGo 框架的 HTTP/REST 端点实现。
// 它提供完整的 HTTP 服务器解决方案，具有动态路由、请求处理以及与 RuleGo 规则链和组件的集成。
//
// Architecture / 架构：
//
// The Rest endpoint follows a layered architecture:
// Rest 端点遵循分层架构：
//
// 1. HTTP Server Layer: Handles low-level HTTP operations  HTTP 服务器层：处理低级 HTTP 操作
// 2. Routing Layer: Maps HTTP requests to rule chains  路由层：将 HTTP 请求映射到规则链
// 3. Message Processing Layer: Converts HTTP to RuleMsg format  消息处理层：将 HTTP 转换为 RuleMsg 格式
// 4. Rule Engine Integration: Executes business logic  规则引擎集成：执行业务逻辑
//
// Key Features / 主要特性：
//
// • HTTP Server Management: Complete server lifecycle with start/stop/restart  HTTP 服务器管理：完整的服务器生命周期
// • Dynamic Routing: Runtime route addition and removal  动态路由：运行时路由添加和删除
// • Shared Server Support: Multiple endpoint instances can share a server  共享服务器支持：多个端点实例可以共享服务器
// • High-Performance Routing: Uses httprouter for fast HTTP routing  高性能路由：使用 httprouter 进行快速 HTTP 路由
// • Method Support: All HTTP methods (GET, POST, PUT, DELETE, etc.)  方法支持：所有 HTTP 方法
// • Path Parameters: Automatic extraction of URL path parameters  路径参数：自动提取 URL 路径参数
// • Static File Serving: Built-in static file server capabilities  静态文件服务：内置静态文件服务器功能
// • CORS Support: Cross-origin request handling  CORS 支持：跨域请求处理
// • SSL/TLS Support: HTTPS server with certificate configuration  SSL/TLS 支持：带证书配置的 HTTPS 服务器
//
// Thread Safety / 线程安全：
//
// The Rest endpoint is designed to be thread-safe for concurrent operations:
// Rest 端点设计为线程安全的并发操作：
//
// • Route management operations are protected by mutex  路由管理操作受互斥锁保护
// • Server operations are safe for concurrent access  服务器操作对并发访问是安全的
// • Message processing supports multiple concurrent requests  消息处理支持多个并发请求
//
// Performance Considerations / 性能考虑：
//
// • Uses httprouter for O(1) routing performance  使用 httprouter 实现 O(1) 路由性能
// • Connection pooling and keep-alive support  连接池和 keep-alive 支持
// • Configurable timeouts to prevent resource exhaustion  可配置超时以防止资源耗尽
// • Shared server instances to reduce memory usage  共享服务器实例以减少内存使用
//
// Usage Patterns / 使用模式：
//
// 1. Simple API Server: Single endpoint with multiple routes  简单 API 服务器：单端点多路由
// 2. Microservice Gateway: Multiple endpoints with shared server  微服务网关：共享服务器的多端点
// 3. REST API with Rule Processing: HTTP requests routed to rule chains  带规则处理的 REST API：HTTP 请求路由到规则链
// 4. Static File Server: Serving static content with dynamic API routes  静态文件服务器：提供静态内容和动态 API 路由
type Rest struct {
	// BaseEndpoint provides common endpoint functionality
	// BaseEndpoint 提供通用端点功能
	impl.BaseEndpoint

	// SharedNode enables server sharing between multiple endpoint instances
	// SharedNode 启用多个端点实例之间的服务器共享
	nodeBase.SharedNode[*Rest]

	// Config contains the HTTP server configuration settings
	// Config 包含 HTTP 服务器配置设置
	Config Config

	// RuleConfig provides access to the rule engine configuration
	// RuleConfig 提供对规则引擎配置的访问
	RuleConfig types.Config

	// Server is the underlying HTTP server instance
	// Server 是底层的 HTTP 服务器实例
	Server *http.Server

	// router handles HTTP request routing using httprouter for performance
	// router 使用 httprouter 处理 HTTP 请求路由以提高性能
	router *httprouter.Router

	// started indicates whether the HTTP server has been started
	// started 指示 HTTP 服务器是否已启动
	started bool
}

// Type 组件类型
func (rest *Rest) Type() string {
	return Type
}

func (rest *Rest) New() types.Node {
	return &Rest{
		Config: Config{
			Server:       ":6333",
			ReadTimeout:  10,
			WriteTimeout: 10,
			IdleTimeout:  60,
		},
	}
}

// Init 初始化
func (rest *Rest) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &rest.Config)
	if err != nil {
		return err
	}
	rest.RuleConfig = ruleConfig
	return rest.SharedNode.Init(rest.RuleConfig, rest.Type(), rest.Config.Server, false, func() (*Rest, error) {
		return rest.initServer()
	})
}

// Destroy 销毁
func (rest *Rest) Destroy() {
	_ = rest.Close()
}

func (rest *Rest) Restart() error {
	if rest.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = rest.Server.Shutdown(ctx)
	}

	if rest.SharedNode.InstanceId != "" {
		if shared, err := rest.SharedNode.Get(); err == nil {
			return shared.Restart()
		} else {
			return err
		}
	}
	if rest.router != nil {
		rest.newRouter()
	}
	var oldRouter = make(map[string]endpoint.Router)

	rest.Lock()
	for id, router := range rest.RouterStorage {
		if !router.IsDisable() {
			oldRouter[id] = router
		}
	}
	rest.Unlock()

	rest.RouterStorage = make(map[string]endpoint.Router)
	rest.started = false

	if err := rest.Start(); err != nil {
		return err
	}
	for _, router := range oldRouter {
		if len(router.GetParams()) == 0 {
			router.SetParams("GET")
		}
		if !rest.HasRouter(router.GetId()) {
			if _, err := rest.AddRouter(router, router.GetParams()...); err != nil {
				rest.Printf("rest add router path:=%s error:%v", router.FromToString(), err)
				continue
			}
		}

	}
	return nil
}

func (rest *Rest) Close() error {
	if rest.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := rest.Server.Shutdown(ctx); err != nil {
			return err
		}
	}
	if rest.router != nil {
		rest.newRouter()
	}
	if rest.SharedNode.InstanceId != "" {
		if shared, err := rest.SharedNode.Get(); err == nil {
			rest.RLock()
			defer rest.RUnlock()
			for key := range rest.RouterStorage {
				shared.deleteRouter(key)
			}
			//重启共享服务
			return shared.Restart()
		}
	}

	rest.started = false
	rest.BaseEndpoint.Destroy()
	return nil
}

func (rest *Rest) Id() string {
	return rest.Config.Server
}

func (rest *Rest) AddRouter(router endpoint.Router, params ...interface{}) (id string, err error) {
	if len(params) <= 0 {
		return "", errors.New("need to specify HTTP method")
	} else if router == nil {
		return "", errors.New("router can not nil")
	} else {
		defer func() {
			if e := recover(); e != nil {
				err = fmt.Errorf("addRouter err :%v", e)
			}
		}()
		err2 := rest.addRouter(strings.ToUpper(str.ToString(params[0])), router)
		return router.GetId(), err2
	}
}

func (rest *Rest) RemoveRouter(routerId string, params ...interface{}) error {
	routerId = strings.TrimSpace(routerId)
	rest.Lock()
	defer rest.Unlock()
	if rest.RouterStorage != nil {
		if router, ok := rest.RouterStorage[routerId]; ok && !router.IsDisable() {
			router.Disable(true)
			return nil
		} else {
			return fmt.Errorf("router: %s not found", routerId)
		}
	}
	return nil
}

func (rest *Rest) deleteRouter(routerId string) {
	routerId = strings.TrimSpace(routerId)
	rest.Lock()
	defer rest.Unlock()
	if rest.RouterStorage != nil {
		delete(rest.RouterStorage, routerId)
	}
}

func (rest *Rest) Start() error {
	if err := rest.checkIsInitSharedNode(); err != nil {
		return err
	}
	if netResource, err := rest.SharedNode.Get(); err == nil {
		return netResource.startServer()
	} else {
		return err
	}
}

func (rest *Rest) Listen() (net.Listener, error) {
	addr := rest.Server.Addr
	if addr == "" {
		if rest.Config.CertKeyFile != "" && rest.Config.CertFile != "" {
			addr = ":https"
		} else {
			addr = ":http"
		}
	}
	return net.Listen("tcp", addr)
}

// addRouter 注册1个或者多个路由
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
func (rest *Rest) addRouter(method string, routers ...endpoint.Router) error {
	method = strings.ToUpper(method)

	rest.Lock()
	defer rest.Unlock()

	if rest.RouterStorage == nil {
		rest.RouterStorage = make(map[string]endpoint.Router)
	}
	for _, item := range routers {
		path := strings.TrimSpace(item.FromToString())
		if id := item.GetId(); id == "" {
			item.SetId(rest.RouterKey(method, path))
		}
		//存储路由
		item.SetParams(method)
		rest.RouterStorage[item.GetId()] = item
		if rest.SharedNode.InstanceId != "" {
			if shared, err := rest.SharedNode.Get(); err == nil {
				return shared.addRouter(method, item)
			} else {
				return err
			}
		} else {
			if rest.router == nil {
				rest.newRouter()
			}
			isWait := false
			if from := item.GetFrom(); from != nil {
				if to := from.GetTo(); to != nil {
					isWait = to.IsWait()
				}
			}
			// 转换路径参数格式：将 {id} 格式转换为 :id 格式
			path = rest.convertPathParams(path)
			rest.router.Handle(method, path, rest.handler(item, isWait))
		}

	}
	return nil
}

func (rest *Rest) GET(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodGet, routers...)
	return rest
}

func (rest *Rest) HEAD(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodHead, routers...)
	return rest
}

func (rest *Rest) OPTIONS(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodOptions, routers...)
	return rest
}

func (rest *Rest) POST(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodPost, routers...)
	return rest
}

func (rest *Rest) PUT(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodPut, routers...)
	return rest
}

func (rest *Rest) PATCH(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodPatch, routers...)
	return rest
}

func (rest *Rest) DELETE(routers ...endpoint.Router) endpoint.HttpEndpoint {
	rest.addRouter(http.MethodDelete, routers...)
	return rest
}

func (rest *Rest) GlobalOPTIONS(handler http.Handler) endpoint.HttpEndpoint {
	rest.Router().GlobalOPTIONS = handler
	return rest
}

func (rest *Rest) RegisterStaticFiles(resourceMapping string) endpoint.HttpEndpoint {
	if resourceMapping != "" {
		mapping := strings.Split(resourceMapping, ",")
		for _, item := range mapping {
			files := strings.Split(item, "=")
			if len(files) == 2 {
				urlPath := strings.TrimSpace(files[0])
				localDir := strings.TrimSpace(files[1])

				// 移除 /*filepath 后缀以获取基础路径
				basePath := urlPath
				if strings.HasSuffix(urlPath, "/*filepath") {
					basePath = urlPath[:len(urlPath)-10]
				}

				// 确保路径以 /{filepath:*} 结尾，这是 fasthttp router 的要求
				if !strings.HasSuffix(urlPath, "/*filepath") {
					if strings.HasSuffix(basePath, "/") {
						urlPath = basePath + "*filepath"
					} else {
						urlPath = basePath + "/*filepath"
					}
				}
				rest.Router().ServeFiles(strings.TrimSpace(urlPath), http.Dir(strings.TrimSpace(localDir)))
			}
		}
	}
	return rest
}

func (rest *Rest) checkIsInitSharedNode() error {
	if !rest.SharedNode.IsInit() {
		err := rest.SharedNode.Init(rest.RuleConfig, rest.Type(), rest.Config.Server, false, func() (*Rest, error) {
			return rest.initServer()
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (rest *Rest) Router() *httprouter.Router {
	rest.checkIsInitSharedNode()

	if fromPool, err := rest.SharedNode.Get(); err != nil {
		rest.Printf("get router err :%v", err)
		return rest.newRouter()
	} else {
		return fromPool.router
	}
}

func (rest *Rest) RouterKey(method string, from string) string {
	return method + ":" + from
}

func (rest *Rest) handler(router endpoint.Router, isWait bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				rest.Printf("http endpoint handler err :\n%v", runtime.Stack())
			}
		}()
		if router.IsDisable() {
			http.NotFound(w, r)
			return
		}
		metadata := types.NewMetadata()
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request:  r,
				response: w,
				Params:   params,
				Metadata: metadata,
			},
			Out: &ResponseMessage{
				request:  r,
				response: w,
			},
		}

		//把路径参数放到msg元数据中
		for _, param := range params {
			metadata.PutValue(param.Key, param.Value)
		}

		//把url?参数放到msg元数据中
		for key, value := range r.URL.Query() {
			if len(value) > 1 {
				metadata.PutValue(key, str.ToString(value))
			} else {
				metadata.PutValue(key, value[0])
			}

		}
		var ctx = r.Context()
		if !isWait {
			//异步不能使用request context，否则后续执行会取消
			ctx = context.Background()
		}
		rest.DoProcess(ctx, router, exchange)
	}
}

func (rest *Rest) Printf(format string, v ...interface{}) {
	if rest.RuleConfig.Logger != nil {
		rest.RuleConfig.Logger.Printf(format, v...)
	}
}

// Started 返回服务是否已经启动
func (rest *Rest) Started() bool {
	return rest.started
}

// GetServer 获取HTTP服务
func (rest *Rest) GetServer() *http.Server {
	if rest.Server != nil {
		return rest.Server
	} else if rest.SharedNode.InstanceId != "" {
		if shared, err := rest.SharedNode.Get(); err == nil {
			return shared.Server
		}
	}
	return nil
}

func (rest *Rest) newRouter() *httprouter.Router {
	rest.router = httprouter.New()
	//设置跨域
	if rest.Config.AllowCors {
		rest.GlobalOPTIONS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(HeaderKeyAccessControlRequestMethod) != "" {
				// 设置 CORS 相关的响应头
				header := w.Header()
				header.Set(HeaderKeyAccessControlAllowMethods, HeaderValueAll)
				header.Set(HeaderKeyAccessControlAllowHeaders, HeaderValueAll)
				header.Set(HeaderKeyAccessControlAllowOrigin, HeaderValueAll)
			}
			// 返回 204 状态码
			w.WriteHeader(http.StatusNoContent)
		}))
		// 直接操作 Interceptors 字段，避免调用 AddInterceptors 造成递归锁
		corsInterceptor := func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			exchange.Out.Headers().Set(HeaderKeyAccessControlAllowOrigin, HeaderValueAll)
			return true
		}
		rest.Interceptors = append(rest.Interceptors, corsInterceptor)
	}
	return rest.router
}

func (rest *Rest) initServer() (*Rest, error) {
	if rest.router == nil {
		rest.newRouter()
	}
	return rest, nil
}

func (rest *Rest) startServer() error {
	if rest.started {
		return nil
	}
	var err error

	// 创建HTTP服务器并应用超时配置
	rest.Server = &http.Server{
		Addr:    rest.Config.Server,
		Handler: rest.router,
	}

	// 应用读取超时配置
	if rest.Config.ReadTimeout > 0 {
		rest.Server.ReadTimeout = time.Duration(rest.Config.ReadTimeout) * time.Second
	} else {
		rest.Server.ReadTimeout = 10 * time.Second // 默认10秒
	}

	// 应用写入超时配置
	if rest.Config.WriteTimeout > 0 {
		rest.Server.WriteTimeout = time.Duration(rest.Config.WriteTimeout) * time.Second
	} else {
		rest.Server.WriteTimeout = 10 * time.Second // 默认10秒
	}

	// 应用空闲超时配置
	if rest.Config.IdleTimeout > 0 {
		rest.Server.IdleTimeout = time.Duration(rest.Config.IdleTimeout) * time.Second
	} else {
		rest.Server.IdleTimeout = 60 * time.Second // 默认60秒
	}

	// 应用禁用keepalive配置
	if rest.Config.DisableKeepalive {
		rest.Server.SetKeepAlivesEnabled(false)
	}
	ln, err := rest.Listen()
	if err != nil {
		return err
	}
	//标记已经启动
	rest.started = true

	isTls := rest.Config.CertKeyFile != "" && rest.Config.CertFile != ""
	if rest.OnEvent != nil {
		rest.OnEvent(endpoint.EventInitServer, rest)
	}
	if isTls {
		rest.Printf("started rest server with TLS on %s", rest.Config.Server)
		go func() {
			defer ln.Close()
			err = rest.Server.ServeTLS(ln, rest.Config.CertFile, rest.Config.CertKeyFile)
			if rest.OnEvent != nil {
				rest.OnEvent(endpoint.EventCompletedServer, err)
			}
		}()
	} else {
		rest.Printf("started rest server on %s", rest.Config.Server)
		go func() {
			defer ln.Close()
			err = rest.Server.Serve(ln)
			if rest.OnEvent != nil {
				rest.OnEvent(endpoint.EventCompletedServer, err)
			}
		}()
	}
	return err
}

// convertPathParams 转换路径参数格式：将 {id}格式转换为 :id  格式
func (rest *Rest) convertPathParams(path string) string {
	// 使用正则表达式匹配 :参数名 格式并转换为 {参数名} 格式
	re := regexp.MustCompile(`{([a-zA-Z_][a-zA-Z0-9_]*)}`)
	return re.ReplaceAllString(path, ":$1")
}
