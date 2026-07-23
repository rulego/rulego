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
// The Package processor provides a built-in processor implementation for the RuleGo endpoint system.
// A processor is a function that can be applied to message exchange during endpoint processing, supporting data transformation, validation, and protocol-specific processing.
//
// Processing Pipeline Integration:
// Integrated Processing Pipelines:
//
// Processors are integrated into the endpoint processing pipeline at multiple levels:
// Processors are integrated into the endpoint processing pipeline at multiple levels:
//
//  1. Global Interceptors: Applied to all messages before routing (BaseEndpoint.Interceptors)
//     Global Interceptors: Applied to all messages before routing (BaseEndpoint.Interceptors)
//
//  2. From Processing: Transform incoming data before target execution (From.processList)
//     From Processing: Transforms incoming data before target execution (From.processList)
//
//  3. To Processing: Handle results after target execution (To.processList)
//     To Process: Process the result after the target execution (To.processList)
//
// Built-in Processor Collections:
// Built-in processor collection:
//
//   - InBuiltins: Input processors for message preparation and transformation
//     InBuiltins: Input processor for message preparation and transformation
//
//   - OutBuiltins: Output processors for response formatting and delivery
//     OutBuiltins: Output processors used to respond to formatting and passing
//
// Available Input Processors:
// Available input processors:
//
//   - headersToMetadata: Extracts HTTP headers into message metadata
//     headersToMetadata: Extracts HTTP headers into message metadata
//
//   - setJsonDataType: Sets message data type to JSON and Content-Type header
//     setJsonDataType: Set the message data type to JSON and set the Content-Type header
//
//   - toHex: Converts binary data to hexadecimal string representation
//     toHex: Converts binary data to hexadecimal string representation
//
// Available Output Processors:
// Available output processors:
//
//   - responseToBody: Formats message data as HTTP response body
//     responseToBody: Format message data into HTTP response body
//
//   - metadataToHeaders: Maps message metadata to HTTP response headers
//     metadataToHeaders: Maps message metadata to HTTP response headers
//
// Usage in Endpoint DSL:
// Use in endpoint DSL:
//
// Processors can be referenced by name in endpoint DSL configuration:
// Processors can be referenced by name in endpoint DSL configurations:
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
// Custom processor development:
//
// Custom processors can be registered for specific use cases:
// You can register custom processors for specific use cases:
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
// Processor function signature:
//
// All processors implement the endpoint.Process function signature:
// All processors implement endpoint.Process function signatures:
//
//	type Process func(router endpoint.Router, exchange *endpoint.Exchange) bool
//
// Return Value Semantics:
// Return value semantics:
//   - true: Continue processing pipeline
//   - false: Stop processing pipeline
package processor

import (
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

const (
	// HeaderKeyContentType is the standard HTTP Content-Type header key.
	// Used for setting and reading the content type of HTTP messages.
	//
	// HeaderKeyContentType is the standard HTTP Content-Type header.
	// Used to set and read the content type of HTTP messages.
	HeaderKeyContentType = "Content-Type"

	// HeaderValueApplicationJson is the MIME type for JSON content.
	// Used when setting Content-Type header for JSON responses.
	//
	// HeaderValueApplicationJson is a MIME type of JSON content.
	// Used when setting the Content-Type header for JSON responses.
	HeaderValueApplicationJson        = "application/json"
	HeaderValueTextPlain              = "text/plain"
	HeaderValueApplicationOctetStream = "application/octet-stream"
	HeaderValueEventStream            = "text/event-stream"
	// HeaderKeyCacheControl is the standard HTTP Cache-Control header key.
	HeaderKeyCacheControl = "Cache-Control"
	// HeaderKeyConnection is the standard HTTP Connection header key.
	HeaderKeyConnection = "Connection"

	// HeaderValueNoCache is the Cache-Control header value for disabling caching.
	HeaderValueNoCache = "no-cache"
	// HeaderValueKeepAlive is the Connection header value for persistent connections.
	HeaderValueKeepAlive = "keep-alive"

	// KeyTopic is a metadata key used for storing message topic information.
	// Commonly used in messaging scenarios to identify the source topic.
	//
	// KeyTopic is a metadata key used to store message topic information.
	// In messaging scenarios, it is often used to identify source topics.
	KeyTopic = "topic"
)

// InBuiltins is a thread-safe collection of built-in input processors.
// These processors are designed to handle incoming message preparation,
// data transformation, and protocol-specific processing before target execution.
//
// InBuiltins is a thread-safe collection of built-in input processors.
// These processors are used to handle protocol-specific processing before incoming message preparation, data transformation, and target execution.
//
// Input processors are typically used in the From processing pipeline to:
// Input processors are typically used in From processing pipelines for:
//   - Extract and normalize protocol headers
//   - Set appropriate data types and content types
//   - Convert data formats for rule engine consumption
//   - Validate incoming message structure
//
// Built-in Input Processors:
// Built-in input processor:
//   - headersToMetadata: HTTP headers → message metadata HTTP headers → message metadata
//   - setJsonDataType: Set JSON data type and Content-Type
//   - toHex: Binary data → hexadecimal string
var InBuiltins = builtins{}

// OutBuiltins is a thread-safe collection of built-in output processors.
// These processors are designed to handle response formatting, protocol-specific
// output preparation, and message delivery after rule chain execution.
//
// OutBuiltins is a thread-safe collection of built-in output processors.
// These processors are used to handle response formatting, protocol-specific output preparation, and message passing after rule chain execution.
//
// Output processors are typically used in the To processing pipeline to:
// Output processors are typically used in To processing pipelines for:
//   - Format rule engine results for protocol responses
//   - Map message metadata to protocol headers
//   - Handle error conditions and status codes
//   - Prepare final response payload
//
// Built-in Output Processors:
// Built-in Output Processor:
//   - responseToBody: Message data → HTTP response body
//   - metadataToHeaders: Message metadata → HTTP headers
var OutBuiltins = builtins{}

// init registers all built-in processors during package initialization.
// This ensures that common processing functions are available immediately
// for use in endpoint configurations and DSL definitions.
//
// init registers all built-in processors during packet initialization.
// This ensures that general-purpose processing functions are immediately available for endpoint configuration and DSL definition.
func init() {
	// Register input processor to extract HTTP headers into message metadata.
	// This enables rule chains to access HTTP header values as message metadata.
	//
	// The registration input processor extracts the HTTP header into message metadata.
	// This allows the rule chain to access HTTP headers as message metadata.
	InBuiltins.Register("headersToMetadata", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
		return true
	})

	// Register input processor to set JSON data type and Content-Type.After setting, the rule chain component will process data based on that type
	// Register the input processor to set the JSON data type and Content-Type. Once set, the rule chain component will process the data according to the type
	InBuiltins.Register("setJsonDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.JSON
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationJson)
		return true
	})
	// Register input processor to set text data type and Content-Type.After setting, the rule chain component will process data based on that type
	// Register the input processor to set the text data type and Content-Type. Once set, the rule chain component will process the data according to the type
	InBuiltins.Register("setTextDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.TEXT
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueTextPlain)
		return true
	})
	// Register input processor to set binary data type and Content. After setting, the rule chain component will process data based on that type
	// Register the input processor to set the binary data type and Content-Type. Once set, the rule chain component will process the data according to the type
	InBuiltins.Register("setBinaryDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.BINARY
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationOctetStream)
		return true
	})

	// Register input processor to convert binary message data to hexadecimal string.
	// The registration input processor converts binary message data into hexadecimal strings.
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
	// The registered output processor formats the result of the rule chain into the HTTP response body.
	// Handle success (message data) and error (error messages).
	// Automatically sets the Content-Type header for JSON responses.
	OutBuiltins.Register("responseToBody", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Lock()
		defer exchange.Unlock()
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			// Set error status and body text in the response.
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			// Set the response body with the message data.
			// Use message data to set response body.
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
	// The registration output processor maps message metadata to the HTTP response header.
	// This allows the rule chain to set custom HTTP headers through message metadata.
	// Errors are also handled by setting appropriate status codes and error body text.
	OutBuiltins.Register("metadataToHeaders", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Lock()
		defer exchange.Unlock()
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body fvin the response.
			// Set error status and body text in the response.
			exchange.Out.SetStatusCode(http.StatusBadRequest)
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
// builtins is a thread-safe registry that allows you to register and retrieve processor functions by name.
// It provides the foundation for the InBuiltins and OutBuiltins collections.
//
// Registry Operations:
// Registry operations:
//   - Register: Add single processor
//   - RegisterAll: Add multiple processors
//   - Unregister: Remove processors by name
//   - Get: Retrieve processor by name
//   - Names: List all registered names
type builtins struct {
	processors map[string]endpoint.Process // Map of processor functions
	lock       sync.RWMutex                // Read/Write mutex for concurrent access
}

// Register adds a single processor function to the registry with the specified name.
// If a processor with the same name already exists, it will be replaced.
//
// Register Adds a single handler function to the registry using a specified name.
// If a processor with the same name already exists, it will be replaced.
//
// Parameters:
// Parameters:
//   - name: Unique identifier for the processor
//   - processor: Function implementing the processor logic
//
// Usage:
// Usage:
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
// RegisterAll adds multiple processor functions to the registry at once.
// When multiple processors are added simultaneously, this is more efficient than multiple Register calls.
//
// Parameters:
// Parameters:
//   - processors: Map of processor names to their implementations
//     processors: The mapping of processor names to their implementations
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
// Unregister removes one or more processor functions from the registry by name.
// Processor names that don't exist will be silently ignored.
//
// Parameters:
// Parameters:
//   - names: Variable number of processor names to remove
//     names: A variable number of processor names to be deleted
//
// Thread Safety:
// Thread safety:
// This method is thread-safe and can be called concurrently.
// This method is thread-safe and can be called concurrently.
//
// Usage:
// Usage:
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
// Get retrieves the processor function from the registry by name.
// Returns the processor function and a boolean value, indicating whether it was found.
//
// Parameters:
// Parameters:
//   - name: The name of the processor to retrieve
//     name: The processor name to be retrieved
//
// Returns:
// Returns:
//   - endpoint.Process: The processor function if found
//   - bool: True if processor exists, false otherwise
//
// Usage:
// Usage:
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
// Names returns a list of all registered processor names.
// The returned slices are copies that can be safely modified without affecting the registry.
//
// Returns:
// Returns:
//   - []string: List of all registered processor names
//
// Usage:
// Usage:
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
