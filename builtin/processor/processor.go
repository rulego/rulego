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

package processor

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"sync"
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
	// Register a processor to respond to the HTTP client with the message.
	OutBuiltins.Register("responseToBody", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			// Set the response body with the message data.
			if exchange.Out.GetMsg().DataType == types.JSON {
				exchange.Out.Headers().Set("Content-Type", "application/json")
			}
			exchange.Out.SetBody([]byte(exchange.Out.GetMsg().Data))
		}
		return true
	})
	// Register a processor to add HTTP headers to message metadata.
	OutBuiltins.Register("metadataToHeaders", func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		if err := exchange.Out.GetError(); err != nil {
			// Set error status and body in the response.
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else if exchange.Out.GetMsg() != nil {
			msg := exchange.Out.GetMsg()
			for k, v := range msg.Metadata {
				exchange.Out.Headers().Set(k, v)
			}
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
