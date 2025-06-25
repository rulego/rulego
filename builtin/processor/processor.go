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

// Package processor provides built-in processor implementations for the RuleGo rule engine.
//
// This package implements various processors that can be used within rule chains,
// allowing for custom logic to be integrated into rule chains. It includes pre-defined
// processors such as HTTP request and response processing, as well as the ability to
// create custom processors.
//
// Key components:
// - InBuiltins: A collection of built-in in processors that can be called by name through endpoint DSL.
// - OutBuiltins: A collection of built-in out processors that can be called by name through endpoint DSL.
//
// The package supports features such as:
// - Registering and accessing processors by name
// - Retrieving all registered processors
// - Unregistering processors
// - Retrieving processor names
package processor

import (
	"encoding/hex"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

const (
	// HeaderKeyContentType Content-Type header key
	HeaderKeyContentType = "Content-Type"
	// HeaderValueApplicationJson Content-Type header value
	HeaderValueApplicationJson = "application/json"
	KeyTopic                   = "topic"
)

// InBuiltins is a collection of built-in in processors that can be called by name through endpoint DSL.
var InBuiltins = builtins{}

// OutBuiltins is a collection of built-in out processors that can be called by name through endpoint DSL.
var OutBuiltins = builtins{}

func init() {
	// Register a processor to add HTTP headers to message metadata.
	InBuiltins.Register("headersToMetadata", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
		return true
	})
	// Register a processor to set the message data type to JSON.
	InBuiltins.Register("setJsonDataType", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		msg.DataType = types.JSON
		exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationJson)
		return true
	})

	// Register a processor to convert the binary bytes message data to hexadecimal.
	InBuiltins.Register("toHex", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		from := exchange.In.From()
		ruleMsg := types.NewMsg(0, from, types.TEXT, types.NewMetadata(), strings.ToUpper(hex.EncodeToString(exchange.In.Body())))
		ruleMsg.Metadata.PutValue(KeyTopic, from)
		exchange.In.SetMsg(&ruleMsg)
		return true
	})

	// Register a processor to respond to the HTTP client with the message.
	OutBuiltins.Register("responseToBody", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Lock()
		defer exchange.Unlock()
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			// Set the response body with the message data.
			if exchange.Out.GetMsg().DataType == types.JSON && exchange.Out.Headers().Get(HeaderKeyContentType) == "" {
				exchange.Out.Headers().Set(HeaderKeyContentType, HeaderValueApplicationJson)
			}
			exchange.Out.SetBody([]byte(exchange.Out.GetMsg().GetData()))
		}
		return true
	})
	// Register a processor to add HTTP headers to message metadata.
	OutBuiltins.Register("metadataToHeaders", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Lock()
		defer exchange.Unlock()
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			msg := exchange.Out.GetMsg()
			msg.Metadata.ForEach(func(k, v string) bool {
				exchange.Out.Headers().Set(k, v)
				return true // continue iteration
			})
		}
		return true
	})
}

// builtins struct holds a map of processor functions that can be registered and called by name.
type builtins struct {
	processors map[string]endpoint.Process // Map of processor functions.
	lock       sync.RWMutex                // Read/Write mutex lock for concurrent access.
}

// Register 注册内置处理器
func (b *builtins) Register(name string, processor endpoint.Process) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.processors == nil {
		b.processors = make(map[string]endpoint.Process)
	}
	b.processors[name] = processor
}

// RegisterAll adds multiple built-in processor functions at once.
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

// Unregister removes one or more built-in processor functions by their names.
func (b *builtins) Unregister(names ...string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, name := range names {
		delete(b.processors, name)
	}
}

// Get retrieves a built-in processor function by its name.
func (b *builtins) Get(name string) (endpoint.Process, bool) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	p, ok := b.processors[name]
	return p, ok
}

// Names Get a list of built-in processor names
func (b *builtins) Names() []string {
	b.lock.RLock()
	defer b.lock.RUnlock()
	var keys = make([]string, 0, len(b.processors))
	for k := range b.processors {
		keys = append(keys, k)
	}
	return keys
}
