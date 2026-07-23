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
// Package rest provides HTTP/REST endpoint implementations for the RuleGo framework.
// It supports the creation of HTTP servers that can receive, process, and respond to HTTP requests,
// Routing them to appropriate rule chains or components for business logic processing.
//
// # Key Features
//
// • HTTP Server Management: Complete HTTP server lifecycle management
// • Dynamic Routing: Runtime addition/removal of HTTP routes
// • Method Support: All standard HTTP methods (GET, POST, PUT, DELETE, etc.)
// • Path Parameters: URL path parameter extraction and processing
// • CORS Support: Cross-Origin Resource Sharing Configuration
// • SSL/TLS Support: HTTPS server with certificate configuration
// • Static File Serving: Built-in static file serving capabilities
// • Shared Server: Multiple endpoint instances can share the same server
//
// # Architecture
//
// The REST endpoint follows a message-based processing model:
// REST endpoints follow a message-based processing model:
//
// 1. HTTP Request → RequestMessage conversion HTTP request → RequestMessage conversion
// 2. RequestMessage → Rule Chain/Component Processing RequestMessage → Rule Chain/Component Processing
// 3. Processing Result → ResponseMessage
// 4. ResponseMessage → HTTP Response ResponseMessage → HTTP response
//
// # Initialization Methods
//
// The REST endpoint supports three initialization approaches:
// REST endpoints support three initialization methods:
//
// 1. Registry-based Initialization
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
//	Create endpoints through the registry
//	endpoint, err := endpoint.Registry.New(rest.Type, ruleConfig, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add router and start
//	Add the router and start it
//	router := endpoint.NewRouter().
//	    From("/api/device/{deviceId}").
//	    To("chain:deviceProcessing")
//
//	endpoint.AddRouter(router, "POST")
//	endpoint.Start()
//
// 2. Dynamic DSL Initialization
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
//	Create endpoints from DSL
//	endpoint, err := endpoint.NewFromDsl([]byte(dslConfig))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	endpoint.Start()
//
// 3. Direct Instantiation with Fluent API
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
//	Use the Smooth API to handle different HTTP methods
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
// # Route Path Patterns
//
// The endpoint supports httprouter-style path patterns:
// Endpoints support HTTPROUTER-style path patterns:
//
// • Static paths: "/api/users"
// • Named parameters: "/api/users/{id}"
// • Catch-all parameters: "/api/files/*filepath"
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
// HTTP header and content type constants for REST endpoints.
// These constants ensure consistency and reduce the number of magic strings in the code.
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
// Type defines the component type identifier for the REST endpoint.
// This identifier is used for component registration and DSL configuration.
const Type = types.EndpointTypePrefix + "http"

// Endpoint is an alias for Rest to provide backward compatibility.
// This allows users to reference the component using either name.
// Endpoint is an alias for Rest and provides backward compatibility.
// This allows users to reference components using any name.
type Endpoint = Rest

var _ endpoint.Endpoint = (*Endpoint)(nil)
var _ endpoint.HttpEndpoint = (*Endpoint)(nil)

// RequestMessage represents an incoming HTTP request message in the RuleGo processing pipeline.
// It encapsulates all the necessary information from an HTTP request and provides methods
// to access request data, headers, parameters, and convert the request into a RuleMsg.
//
// RequestMessage means RuleGo handles incoming HTTP request messages in the pipeline.
// It encapsulates all the necessary information for HTTP requests and provides methods to access request data, headers, and parameters,
// and convert the request into RuleMsg.
//
// Key Features
// • HTTP Request Wrapping: Provides a unified interface for HTTP request data
// • Lazy Body Reading: Body is read only when accessed to optimize performance
// • Parameter Extraction: Supports both path and query parameters
// • Automatic Content Type Detection: Determines data type based on Content-Type header
// • Metadata Integration: Seamlessly integrates with RuleGo's metadata system
//
// Message Flow
// 1. HTTP request received by server
// 2. RequestMessage created with request context
// 3. Body read and cached on first access
// 4. Converted to RuleMsg for rule chain processing
type RequestMessage struct {
	//HTTP request object containing all request information HTTP request object containing all request information
	request *http.Request
	//HTTP response writer for writing response data HTTP response writer
	response http.ResponseWriter
	//Request body data, lazily loaded for performance
	body []byte
	//Path parameters, named parameters extracted from URL paths
	Params httprouter.Params
	//Converted rule message, cached to avoid re-conversion
	msg *types.RuleMsg
	//Error information during processing
	err error
	//Message metadata for storing additional key-value information
	Metadata *types.Metadata
}

// Body returns the HTTP request body as a byte slice.
// The body is read lazily on the first call and cached for subsequent calls.
// This approach optimizes performance by avoiding unnecessary I/O operations.
//
// Body returns the HTTP request body as a byte slice.
// On the first call, the text is read with delay and cached for subsequent calls.
// This approach optimizes performance by avoiding unnecessary I/O operations.
//
// Returns
// • []byte: The request body content, empty slice if no body or error
//
// Note: The request body stream is automatically closed after reading
// Note: After reading, the request body stream will automatically close
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
// Headers returns the HTTP request header as textproto.MIMEHeader.
// This provides the ability to access all HTTP headers in a standardized format.
//
// Returns
// • textproto.MIMEHeader: HTTP headers map, nil if no request HTTP header map; if no request, nil is used
func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.request == nil {
		return nil
	}
	return textproto.MIMEHeader(r.request.Header)
}

// From returns the complete request URL as a string.
// This is used for routing and logging purposes.
//
// From returns the complete request URL as a string.
// Used for routing and logging purposes.
//
// Returns
// • string: Complete request URL, empty string if no request
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
// GetParam retrieves parameter values from path parameters or query parameters via keys.
// It first checks the path parameter (URL segment), then falls back to the query parameters.
// This provides a unified way to access all types of HTTP parameters.
//
// Parameters
// • key: Parameter name to retrieve
//
// Returns
// • string: Parameter value, empty string if not found
//
// Priority Order
// 1. Path parameters (e.g., /users/{id})
// 2. Query parameters (e.g., ?name=value)
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
// SetMsg sets RuleMsg for this request message.
// This is usually used during message processing to cache the transformed messages.
func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

// GetMsg converts the HTTP request to a RuleMsg for rule chain processing.
// The conversion includes automatic data type detection and metadata population.
//
// GetMsg converts HTTP requests into RuleMsg for rule chain processing.
// Transformation includes automatic data type detection and metadata filling.
//
// Returns
// • *types.RuleMsg: Converted rule message ready for processing
//
// Conversion Logic
// • GET requests: Query parameters as JSON data GET requests: Query parameters as JSON data
// • Other methods: Request body as data
// • Content-Type detection: JSON vs TEXT based on Content-Type header
// • Metadata: Additional request information
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
// SetStatusCode is non-operative for request messages because the status code is set on the response.
// This method exists to satisfy the Message interface.
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

// SetBody sets the request body content.
// This is typically used for testing or message transformation scenarios.
//
// SetBody sets the content of the request body.
// This is usually used for testing or message conversion scenarios.
func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

// SetError sets an error associated with this request message.
// This is used to track errors during request processing.
//
// SetError sets the error associated with this request message.
// Used to track errors during request processing.
func (r *RequestMessage) SetError(err error) {
	r.err = err
}

// GetError returns any error associated with this request message.
//
// GetError returns any errors associated with this request message.
func (r *RequestMessage) GetError() error {
	return r.err
}

// Request returns the underlying HTTP request object.
// This provides direct access to the original HTTP request for advanced scenarios.
//
// Request returns the underlying HTTP request object.
// This provides direct access to the original HTTP request for advanced scenarios.
func (r *RequestMessage) Request() *http.Request {
	return r.request
}

// Response returns the HTTP response writer.
// This allows direct writing to the HTTP response if needed.
//
// Response returns the HTTP response writer.
// If needed, this allows direct writing of HTTP responses.
func (r *RequestMessage) Response() http.ResponseWriter {
	return r.response
}

// ResponseMessage represents an outgoing HTTP response message in the RuleGo processing pipeline.
// It handles the conversion of rule processing results back into HTTP responses,
// including status codes, headers, and response body content.
//
// ResponseMessage means RuleGo handles outgoing HTTP response messages in the pipeline.
// It handles the conversion of rule processing results back into HTTP responses, including the status code, header, and response body content.
//
// Thread Safety
// ResponseMessage is thread-safe and can be safely accessed from multiple goroutines.
// All write operations are protected by a mutex to prevent race conditions.
// ResponseMessage is thread-safe and can be securely accessed from multiple coroutines.
// All write operations are protected by mutexes to prevent race conditions.
//
// Key Features
// • Thread-Safe Operations: All methods are protected by mutex for concurrent access
// • Automatic Response Writing: Body content is automatically written to HTTP response
// • Status Code Management: Support for HTTP status code setting
// • Header Management: Access to HTTP response headers
// • Error Handling: Built-in error tracking and reporting
type ResponseMessage struct {
	//Original HTTP request object
	request *http.Request
	//HTTP response writer HTTP response writer
	response http.ResponseWriter
	//Response metadata
	metadata *types.Metadata
	//HTTP status code HTTP status code HTTP status code
	statusCode int
	//Whether response headers have been written
	headerWritten bool
	//Response body data
	body []byte
	//Target path or identifier
	to string
	//Rule message with processing results
	msg *types.RuleMsg
	//Error during response processing
	err error
	//Mutex protecting concurrent access
	mu sync.RWMutex
}

// Body returns the response body content in a thread-safe manner.
// This method is used to retrieve the current response body data.
//
// Body returns the response body content in a thread-safe manner.
// This method is used to retrieve the current responder data.
//
// Returns
// • []byte: Current response body content
func (r *ResponseMessage) Body() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.body
}

// Headers returns the HTTP response headers in a thread-safe manner.
// This provides access to the response headers for reading or modification.
//
// Headers return HTTP response headers in a thread-safe manner.
// This provides access to the response header for reading or modification.
//
// Returns
// • textproto.MIMEHeader: Response headers map, nil if no response writer
func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.response == nil {
		return nil
	}
	return textproto.MIMEHeader(r.response.Header())
}

// AddHeader appends a response header value in a thread-safe manner.
// It is used by output processors that rely on the HeaderModifier interface.
//
// AddHeader adds response header values in a thread-safe manner.
// It is used for output processors that rely on the HeaderModifier interface.
func (r *ResponseMessage) AddHeader(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.response == nil {
		return
	}
	r.response.Header().Add(key, value)
}

// SetHeader sets a response header value in a thread-safe manner.
// It is used by output processors that rely on the HeaderModifier interface.
//
// SetHeader sets the response header value in a thread-safe manner.
// It is used for output processors that rely on the HeaderModifier interface.
func (r *ResponseMessage) SetHeader(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.response == nil {
		return
	}
	r.response.Header().Set(key, value)
}

// DelHeader removes a response header value in a thread-safe manner.
// It is used by output processors that rely on the HeaderModifier interface.
//
// DelHeader deletes response header values in thread-safe ways.
// It is used for output processors that rely on the HeaderModifier interface.
func (r *ResponseMessage) DelHeader(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.response == nil {
		return
	}
	r.response.Header().Del(key)
}

// GetMetadata returns response-scoped metadata, initializing it lazily when needed.
// This keeps rest.ResponseMessage compatible with the HeaderModifier interface.
//
// GetMetadata returns response scope metadata and delays initialization when needed.
// This makes rest.ResponseMessage remains compatible with the HeaderModifier interface.
func (r *ResponseMessage) GetMetadata() *types.Metadata {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.metadata == nil {
		r.metadata = types.NewMetadata()
	}
	return r.metadata
}

// From returns the original request URL for context.
// This is useful for logging and debugging purposes.
//
// From returns the original request URL as context.
// This is useful for logging and debugging.
//
// Returns
// • string: Original request URL, empty if no request
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
// GetParam retrieves parameters from the original request.
// This provides access to request parameters for response processing.
//
// Parameters
// • key: Parameter name to retrieve
//
// Returns
// • string: Parameter value, empty if not found or no request
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
// SetMsg sets rule messages in this response thread-safely.
// This is usually called during rule processing to set the processing result.
//
// Parameters
// • msg: The rule message containing processing results
func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}

// GetMsg returns the rule message associated with this response.
// This provides access to the processing results for response generation.
//
// GetMsg returns the rule message associated with this response.
// This provides access to the processing results for response generation.
//
// Returns
// • *types.RuleMsg: The rule message with processing results
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

// SetStatusCode sets the HTTP response status code.
// The status code is immediately written to the HTTP response.
//
// SetStatusCode sets the HTTP response status code.
// The status code is immediately written into the HTTP response.
//
// Parameters
// • statusCode: HTTP status code to set (e.g., 200, 404, 500)
//
// Note: This should be called before SetBody to ensure proper HTTP response format
// Note: Call before SetBody to ensure the correct HTTP response formatting
func (r *ResponseMessage) SetStatusCode(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusCode = statusCode
	if r.response != nil {
		if r.headerWritten {
			return
		}
		defer func() {
			if err := recover(); err != nil {
				r.err = fmt.Errorf("write header panic: %v", err)
			}
		}()
		r.response.WriteHeader(statusCode)
		r.headerWritten = true
	}
}

// SetBody sets the response body content and immediately writes it to the HTTP response.
// This method combines both storing the body content and sending it to the client.
//
// SetBody sets the response body content and immediately writes it to the HTTP response.
// This method combines storing the body content with sending it to the client.
//
// Parameters
// • body: Response body content to set and send
//
// Behavior
// 1. Store body content internally
// 2. Write body to HTTP response writer
// 3. Handle any write errors
//
// Thread Safety
// This method is thread-safe and can be called concurrently
// This method is thread-safe and can be called concurrently
func (r *ResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.body = body
	if r.response != nil {
		if len(body) > 0 {
			defer func() {
				if err := recover(); err != nil {
					r.err = fmt.Errorf("write body panic: %v", err)
				}
			}()
			_, err := r.response.Write(body)
			if err != nil {
				r.err = err
				return
			}
			r.headerWritten = true
		}
	}
}

// SetError sets an error associated with this response message.
// This is used for error tracking and debugging purposes.
//
// SetError sets the error associated with this response message.
// Used for error tracking and debugging purposes.
//
// Parameters
// • err: Error to associate with this response
func (r *ResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// GetError returns any error associated with this response message.
// This is useful for error handling and debugging.
//
// GetError returns any errors associated with this response message.
// This is useful for error handling and debugging.
//
// Returns
// • error: Associated error, nil if no error
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

// Flush sends any buffered data to the client by calling Flush on the underlying
// http.ResponseWriter if it implements http.Flusher.
// This is particularly important for streaming responses like SSE.
//
// Flush calls the underlying http.ResponseWriter's Flush method (if http.Flusher)
// Send buffered data to the client.
// This is especially important for stream responses like SSE.
func (r *ResponseMessage) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.response == nil {
		return
	}
	// Using recover to capture panic, the crash occurs when the connection is closed
	defer func() {
		if err := recover(); err != nil {
			// Records errors but does not interrupt execution
		}
	}()
	if flusher, ok := r.response.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Config defines the configuration structure for the REST endpoint server.
// It contains all necessary settings for HTTP server initialization and behavior control.
//
// Config defines the configuration structure of the REST endpoint server.
// It contains all necessary settings for HTTP server initialization and behavior control.
//
// # Configuration Categories
//
// Server Settings
// • Server address and port binding
// • SSL/TLS certificate configuration
// • Cross-Origin Resource Sharing (CORS) settings
//
// Performance Tuning
// • Connection timeout configurations
// • Keep-alive connection management
// • Resource optimization settings
//
// Security Features
// • HTTPS support with certificate files
// • CORS policy enforcement
// • Connection timeout protection
type Config struct {
	// Server specifies the server address and port to bind to.
	// Format: "host:port" or ":port" for all interfaces.
	// Examples: ":8080", "localhost:9090", "0.0.0.0:3000"
	// Server specifies the server address and port to bind.
	// Format: use "host:port", or ":port" to listen on all interfaces.
	// Examples: ":8080", "localhost:9090", "0.0.0.0:3000"
	Server string `json:"server" label:"Server" desc:"HTTP server address and port to bind to, e.g. :8080, 0.0.0.0:9090" required:"true" ref:"primary"`

	// CertFile specifies the path to the SSL/TLS certificate file for HTTPS.
	// When both CertFile and CertKeyFile are provided, the server runs in HTTPS mode.
	// CertFile specifies the path to the HTTPS SSL/TLS certificate file.
	// When CertFile and CertKeyFile are provided, the server runs in HTTPS mode.
	CertFile string `json:"certFile" label:"Cert File" desc:"SSL/TLS certificate file path; provide together with certKeyFile to enable HTTPS" ref:"shared"`

	// CertKeyFile specifies the path to the SSL/TLS private key file for HTTPS.
	// This file must correspond to the certificate specified in CertFile.
	// CertKeyFile specifies the path of the HTTPS SSL/TLS private key file.
	// This file must correspond to the certificate specified in CertFile.
	CertKeyFile string `json:"certKeyFile" label:"Cert Key File" desc:"SSL/TLS private key file path; provide together with certFile to enable HTTPS" ref:"shared"`

	// AllowCors enables Cross-Origin Resource Sharing (CORS) support.
	// When true, the server allows cross-origin requests from web browsers.
	// This is useful for API servers that need to be accessed from web applications.
	// AllowCors enables Cross-Origin Resource Sharing (CORS) support.
	// When true, the server allows cross-origin requests from the web browser.
	// This is useful for API servers that need to be accessed from web applications.
	AllowCors bool `json:"allowCors" label:"Allow CORS" desc:"Enable Cross-Origin Resource Sharing for browser access"`

	// ReadTimeout sets the maximum duration for reading the entire request, including the body.
	// Specified in seconds. A value of 0 uses the default timeout of 10 seconds.
	// This prevents slow or malicious clients from holding connections indefinitely.
	// ReadTimeout sets the maximum duration for reading the entire request (including the body).
	// Specified in seconds. When the value is 0, use the default 10-second timeout.
	// This prevents slow or malicious clients from staying connected indefinitely.
	ReadTimeout int `json:"readTimeout" label:"Read Timeout" desc:"Max duration in seconds for reading the entire request, default 10"`

	// WriteTimeout sets the maximum duration before timing out writes of the response.
	// Specified in seconds. A value of 0 uses the default timeout of 10 seconds.
	// This ensures timely response delivery and prevents resource exhaustion.
	// WriteTimeout sets the maximum duration before a response writes out timeout.
	// Specified in seconds. When the value is 0, use the default 10-second timeout.
	// This ensures timely response delivery and prevents resource depletion.
	WriteTimeout int `json:"writeTimeout" label:"Write Timeout" desc:"Max duration in seconds for writing the response, default 10"`

	// IdleTimeout sets the maximum amount of time to wait for the next request
	// when keep-alives are enabled. Specified in seconds.
	// A value of 0 uses the default timeout of 60 seconds.
	// IdleTimeout sets the maximum waiting time for the next request when keep-alive is enabled.
	// Specified in seconds. When the value is 0, use the default 60-second timeout.
	IdleTimeout int `json:"idleTimeout" label:"Idle Timeout" desc:"Max duration in seconds to wait for the next keep-alive request, default 60"`

	// DisableKeepalive disables HTTP keep-alive connections.
	// When true, each request uses a new connection, which may impact performance
	// but can be useful for certain deployment scenarios or debugging.
	// DisableKeepalive Disables HTTP keep-alive connections.
	// When true, each request uses a new connection, which may affect performance,
	// However, it may be useful for certain deployment scenarios or debugging.
	DisableKeepalive bool `json:"disableKeepalive" label:"Disable Keepalive" desc:"Disable HTTP keep-alive connections, each request uses a new connection"`
}

// Rest represents an HTTP/REST endpoint implementation for the RuleGo framework.
// It provides a complete HTTP server solution with dynamic routing, request processing,
// and integration with RuleGo's rule chains and components.
//
// Rest represents the HTTP/REST endpoint implementation of the RuleGo framework.
// It offers a complete HTTP server solution with dynamic routing, request processing, and integration with RuleGo rule chains and components.
//
// # Architecture
//
// The Rest endpoint follows a layered architecture:
// Rest endpoints follow a layered architecture:
//
// 1. HTTP Server Layer: Handles low-level HTTP operations HTTP Server Layer: Handles low-level HTTP operations
// 2. Routing Layer: Maps HTTP requests to rule chains
// 3. Message Processing Layer: Converts HTTP to RuleMsg format
// 4. Rule Engine Integration: Executes business logic
//
// # Key Features
//
// • HTTP Server Management: Complete server lifecycle with start/stop/restart HTTP Server Management: Complete server lifecycle
// • Dynamic Routing: Runtime route addition and removal
// • Shared Server Support: Multiple endpoint instances can share a server
// • High-Performance Routing: Uses httprouter for fast HTTP routing
// • Method Support: All HTTP methods (GET, POST, PUT, DELETE, etc.)
// • Path Parameters: Automatic extraction of URL path parameters
// • Static File Serving: Built-in static file server capabilities
// • CORS Support: Cross-origin request handling CORS Support: Cross-origin request handling
// • SSL/TLS Support: HTTPS server with certificate configuration
//
// # Thread Safety
//
// The Rest endpoint is designed to be thread-safe for concurrent operations:
// Rest endpoints are designed for thread-safe concurrent operations:
//
// • Route management operations are protected by mutex
// • Server operations are safe for concurrent access
// • Message processing supports multiple concurrent requests
//
// # Performance Considerations
//
// • Uses httprouter for O(1) routing performance
// • Connection pooling and keep-alive support
// • Configurable timeouts to prevent resource exhaustion
// • Shared server instances to reduce memory usage
//
// # Usage Patterns
//
// 1. Simple API Server: Single endpoint with multiple routes
// 2. Microservice Gateway: Multiple endpoints with shared server
// 3. REST API with Rule Processing: HTTP requests routed to rule chains
// 4. Static File Server: Serving static content with dynamic API routes
type Rest struct {
	// BaseEndpoint provides common endpoint functionality
	// BaseEndpoint provides universal endpoint functionality
	impl.BaseEndpoint

	// SharedNode enables server sharing between multiple endpoint instances
	// SharedNode enables server sharing among multiple endpoint instances
	nodeBase.SharedNode[*Rest]

	// Config contains the HTTP server configuration settings
	// Config contains HTTP server configuration settings
	Config Config

	// RuleConfig provides access to the rule engine configuration
	// RuleConfig provides access to the rule engine configuration
	RuleConfig types.Config

	// Server is the underlying HTTP server instance
	// Server is the underlying HTTP server instance
	Server *http.Server

	// router handles HTTP request routing using httprouter for performance
	// The router uses HTTP Router to handle HTTP request routing to improve performance
	router *httprouter.Router

	// started indicates whether the HTTP server has been started
	// 'started' indicates whether the HTTP server has started
	started bool
	// resourceMapping is the resource mapping for static file serving
	resourceMapping string
}

// Type returns the component type
func (rest *Rest) Type() string {
	return Type
}

// Category returns the component category.
func (rest *Rest) Category() string {
	return "endpoint"
}

// Def returns the component definition including description and router form metadata.
func (rest *Rest) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "HTTP/REST server endpoint for receiving and processing HTTP requests",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Path",
					Desc:     "HTTP route path pattern; supports {param} (e.g. /api/device/{deviceId}) and *filepath catch-all. HTTP method is set via router params, not here",
					Required: true,
				},
			},
		},
	}
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

// Init initializes the component
func (rest *Rest) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &rest.Config)
	if err != nil {
		return err
	}
	rest.RuleConfig = ruleConfig
	return rest.SharedNode.InitWithClose(rest.RuleConfig, rest.Type(), rest.Config.Server, false, func() (*Rest, error) {
		return rest.initServer()
	}, func(server *Rest) error {
		if server != nil {
			return server.Close()
		}
		return nil
	})
}

// Destroy releases resources
func (rest *Rest) Destroy() {
	_ = rest.Close()
}

// shutdownServer uses a unified shutdown logic
// Unified server shutdown logic
// A unified server shutdown logic
func (rest *Rest) shutdownServer() error {
	// Use locks to protect concurrent security
	rest.Lock()
	defer rest.Unlock()

	// Check if the server has been shut down (Idempotency guarantee)
	if rest.Server == nil {
		return nil
	}

	// Increase the shutdown timeout to 2 seconds to ensure enough time for elegant closing
	// Increase shutdown timeout to 2 seconds to ensure graceful shutdown completion
	// Increase the shutdown timeout to 2 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var shutdownErr error
	// Gracefully shut down the server
	// Gracefully shutdown the server
	// Gracefully shut down the server
	if err := rest.Server.Shutdown(ctx); err != nil {
		// If the elegant closure fails, the forced closure is enforced
		// Force close if graceful shutdown fails
		// If the elegant closure fails, the forced closure is enforced
		rest.Printf("graceful shutdown failed, forcing close: %v", err)
		if closeErr := rest.Server.Close(); closeErr != nil {
			rest.Printf("force close failed: %v", closeErr)
		}
		shutdownErr = err
	}

	// First mark it as stopped
	rest.started = false
	// Clean up server references to prevent repeated shutdowns
	// Clear server reference to prevent duplicate shutdown
	// Clean up server references
	rest.Server = nil

	// Wait a short while to ensure the port is fully released
	// Wait a moment to ensure port is fully released
	// Wait to ensure the port is fully released
	time.Sleep(100 * time.Millisecond)

	return shutdownErr
}

func (rest *Rest) Restart() error {
	// Use a unified closing method, ignore errors, and continue restarting the process
	_ = rest.shutdownServer()

	if rest.SharedNode.InstanceId != "" {
		if shared, err := rest.SharedNode.GetSafely(); err == nil {
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

	if err := rest.Start(); err != nil {
		return err
	}

	if rest.OnEvent != nil {
		rest.OnEvent(endpoint.EventRestart, oldRouter)
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
	if rest.resourceMapping != "" {
		rest.RegisterStaticFiles(rest.resourceMapping)
	}
	return nil
}

func (rest *Rest) Close() error {
	// Use a unified closing method to retain error handling
	if err := rest.shutdownServer(); err != nil {
		// In the Close() method, we need to continue cleaning up, even if the close fails
		rest.Printf("server shutdown error during close: %v", err)
	}

	if rest.router != nil {
		rest.newRouter()
	}
	if rest.SharedNode.InstanceId != "" {
		if shared, err := rest.SharedNode.GetSafely(); err == nil {
			rest.RLock()
			defer rest.RUnlock()
			for key := range rest.RouterStorage {
				shared.deleteRouter(key)
			}
			//If the shared service has stopped, there is no need to restart
			if !shared.Started() {
				return nil
			}
			//Restart the shared service
			return shared.Restart()
		}
	}

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
	if netResource, err := rest.SharedNode.GetSafely(); err == nil {
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

// addRouter registers one or more routes
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
		//Store the route
		item.SetParams(method)
		rest.RouterStorage[item.GetId()] = item
		if rest.SharedNode.InstanceId != "" {
			if shared, err := rest.SharedNode.GetSafely(); err == nil {
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
			// Convert path parameter format: Convert {id} format to:id format
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
		rest.resourceMapping = resourceMapping
		mapping := strings.Split(resourceMapping, ",")
		for _, item := range mapping {
			files := strings.Split(item, "=")
			if len(files) == 2 {
				urlPath := strings.TrimSpace(files[0])
				localDir := strings.TrimSpace(files[1])

				// Remove the /*filepath suffix to get the base path
				basePath := urlPath
				if strings.HasSuffix(urlPath, "/*filepath") {
					basePath = urlPath[:len(urlPath)-10]
				}

				// Make sure the path ends with /{filepath:*}, which is a requirement for the fastHTTP router
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
		err := rest.SharedNode.InitWithClose(rest.RuleConfig, rest.Type(), rest.Config.Server, false, func() (*Rest, error) {
			return rest.initServer()
		}, func(server *Rest) error {
			if server != nil {
				return server.Close()
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (rest *Rest) Router() *httprouter.Router {
	rest.checkIsInitSharedNode()

	if fromPool, err := rest.SharedNode.GetSafely(); err != nil {
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
			//Capture anomalies
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

		//Put the path parameter into the msg metadata
		for _, param := range params {
			metadata.PutValue(param.Key, param.Value)
		}

		//Place the url? parameter into the msg metadata
		for key, value := range r.URL.Query() {
			if len(value) > 1 {
				metadata.PutValue(key, str.ToString(value))
			} else {
				metadata.PutValue(key, value[0])
			}

		}
		var ctx = r.Context()
		if !isWait {
			//Asynchronous Request Context cannot be used; otherwise, subsequent executions will be canceled
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

// Started returns whether the service has started
func (rest *Rest) Started() bool {
	rest.RLock()
	defer rest.RUnlock()
	return rest.started
}

// GetServer obtains HTTP services
func (rest *Rest) GetServer() *http.Server {
	rest.RLock()
	defer rest.RUnlock()
	if rest.Server != nil {
		return rest.Server
	} else if rest.SharedNode.InstanceId != "" {
		if shared, err := rest.SharedNode.GetSafely(); err == nil {
			return shared.Server
		}
	}
	return nil
}

func (rest *Rest) newRouter() *httprouter.Router {
	rest.router = httprouter.New()
	//Set up cross-domain
	if rest.Config.AllowCors {
		// Set GlobalOPTIONS directly without calling the Router() method to avoid recursive locks
		// Set GlobalOPTIONS directly without calling Router() method to avoid recursive lock
		// Directly set up GlobalOPTIONS to avoid recursive locks
		rest.router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(HeaderKeyAccessControlRequestMethod) != "" {
				// Set CORS-related response headers
				header := w.Header()
				header.Set(HeaderKeyAccessControlAllowMethods, HeaderValueAll)
				header.Set(HeaderKeyAccessControlAllowHeaders, HeaderValueAll)
				header.Set(HeaderKeyAccessControlAllowOrigin, HeaderValueAll)
			}
			// Return the 204 status code
			w.WriteHeader(http.StatusNoContent)
		})
		// Directly operate the Interceptors field to avoid recursive locks caused by calling AddInterceptors
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
	rest.RLock()
	if rest.started {
		rest.RUnlock()
		return nil
	}
	rest.RUnlock()
	var err error

	// Create an HTTP server and apply timeout configuration
	rest.Server = &http.Server{
		Addr:    rest.Config.Server,
		Handler: rest.router,
	}

	// Application reads timeout configurations
	if rest.Config.ReadTimeout > 0 {
		rest.Server.ReadTimeout = time.Duration(rest.Config.ReadTimeout) * time.Second
	} else {
		rest.Server.ReadTimeout = 10 * time.Second // Default is 10 seconds
	}

	// Application writes timeout configuration
	if rest.Config.WriteTimeout > 0 {
		rest.Server.WriteTimeout = time.Duration(rest.Config.WriteTimeout) * time.Second
	} else {
		rest.Server.WriteTimeout = 10 * time.Second // Default is 10 seconds
	}

	// Application idle timeout configuration
	if rest.Config.IdleTimeout > 0 {
		rest.Server.IdleTimeout = time.Duration(rest.Config.IdleTimeout) * time.Second
	} else {
		rest.Server.IdleTimeout = 60 * time.Second // Default is 60 seconds
	}

	// Disable the keepalive configuration in the app
	if rest.Config.DisableKeepalive {
		rest.Server.SetKeepAlivesEnabled(false)
	}
	ln, err := rest.Listen()
	if err != nil {
		return err
	}
	//The marker has already been activated
	rest.Lock()
	rest.started = true
	rest.Unlock()

	// Securely access the Config and Server fields
	rest.RLock()
	isTls := rest.Config.CertKeyFile != "" && rest.Config.CertFile != ""
	certFile := rest.Config.CertFile
	certKeyFile := rest.Config.CertKeyFile
	serverAddr := rest.Config.Server
	onEvent := rest.OnEvent
	server := rest.Server // Save Server references to prevent modifications by other goroutines when accessing in goroutine
	rest.RUnlock()

	// Calls the OnEvent callback outside the lock to avoid deadlocks
	if onEvent != nil {
		onEvent(endpoint.EventInitServer, rest)
	}
	if isTls {
		rest.Printf("started rest server with TLS on %s", serverAddr)
		go func() {
			defer ln.Close()
			err = server.ServeTLS(ln, certFile, certKeyFile)
			// Securely access the OnEvent field
			rest.RLock()
			onEvent := rest.OnEvent
			rest.RUnlock()
			if onEvent != nil {
				onEvent(endpoint.EventCompletedServer, err)
			}
		}()
	} else {
		rest.Printf("started rest server on %s", serverAddr)
		go func() {
			defer ln.Close()
			err = server.Serve(ln)
			// Securely access the OnEvent field
			rest.RLock()
			onEvent := rest.OnEvent
			rest.RUnlock()
			if onEvent != nil {
				onEvent(endpoint.EventCompletedServer, err)
			}
		}()
	}
	return err
}

// convertPathParams Convert path parameter format: Convert {id} format to:id
func (rest *Rest) convertPathParams(path string) string {
	// Use regular expressions to match:parametername_format and convert to {parameter_name}
	re := regexp.MustCompile(`{([a-zA-Z_][a-zA-Z0-9_]*)}`)
	return re.ReplaceAllString(path, ":$1")
}
