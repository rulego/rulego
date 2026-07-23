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

// Package base provides foundational components and utilities for the RuleGo rule engine.
package base

import (
	"errors"
	"github.com/rulego/rulego/utils/json"
	"reflect"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
)

var (
	ErrNodePoolNil   = errors.New("node pool is nil")
	ErrClientNotInit = errors.New("client not init")
)

var NodeUtils = &nodeUtils{}

type nodeUtils struct {
}

func (n *nodeUtils) GetChainCtx(configuration types.Configuration) types.ChainCtx {
	if v, ok := configuration[types.NodeConfigurationKeyChainCtx]; ok {
		if chainCtx, ok := v.(types.ChainCtx); ok {
			return chainCtx
		}
	}
	return nil
}
func (n *nodeUtils) GetSelfDefinition(configuration types.Configuration) types.RuleNode {
	if v, ok := configuration[types.NodeConfigurationKeySelfDefinition]; ok {
		if ruleNode, ok := v.(types.RuleNode); ok {
			return ruleNode
		}
	}
	return types.RuleNode{}
}

func (n *nodeUtils) GetVars(configuration types.Configuration) map[string]interface{} {
	if v, ok := configuration[types.Vars]; ok {
		fromVars := make(map[string]interface{})
		fromVars[types.Vars] = v
		return fromVars
	} else {
		return nil
	}
}

func (n *nodeUtils) GetEvn(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return n.getEvnAndMetadata(ctx, msg, false)
}

func (n *nodeUtils) GetEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return ctx.GetEnv(msg, true)
}

func (n *nodeUtils) IsNodePool(config types.Config, server string) bool {
	return strings.HasPrefix(server, types.NodeConfigurationPrefixInstanceId)
}

func (n *nodeUtils) GetInstanceId(config types.Config, server string) string {
	if n.IsNodePool(config, server) {
		//Intercept resource ID
		return server[len(types.NodeConfigurationPrefixInstanceId):]
	}
	return ""
}

func (n *nodeUtils) IsInitNetResource(_ types.Config, configuration types.Configuration) bool {
	_, ok := configuration[types.NodeConfigurationKeyIsInitNetResource]
	return ok
}

func (n *nodeUtils) getEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	// Directly call ctx's GetEvnAndMetadata method
	return ctx.GetEnv(msg, useMetadata)
}

// GetDataByType prepares data to be passed to JavaScript scripts
// Different processing is performed depending on the data type of the message:
// - JSON type: parses into map for JavaScript processing
// - BINARY type: converts to a byte array, which JavaScript treats as Uint8Array
// - Other types: Use raw string data
func (n *nodeUtils) GetDataByType(msg types.RuleMsg, readOnly bool) interface{} {
	var data interface{}
	// Different processing is performed depending on the data type
	switch msg.DataType {
	case types.JSON:
		if readOnly {
			if dataMap, err := msg.GetJsonData(); err == nil {
				data = dataMap
			} else {
				data = msg.GetData()
			}
		} else {
			// JSON type: JS modifies data, so re-parse is needed here
			var dataMap interface{}
			if err := json.Unmarshal(msg.GetBytes(), &dataMap); err == nil {
				data = dataMap
			} else {
				data = msg.GetData()
			}
		}
	case types.BINARY:
		if readOnly {
			data = msg.GetBytes()
		} else {
			// Binary type: Creates a copy of the byte array to avoid concurrent modification issues; JavaScript treats it as a Uint8Array
			originalBytes := msg.GetBytes()
			if originalBytes != nil {
				// Create replicas to ensure concurrency security
				copyBytes := make([]byte, len(originalBytes))
				copy(copyBytes, originalBytes)
				data = copyBytes
			} else {
				data = originalBytes
			}
		}

	default:
		// Other types: Use raw string data
		data = msg.GetData()
	}

	return data
}

// TrimStrings removes all preceding and following spaces from the configuration of string values
// Traverses all values in Configuration; if the string type is used, remove the preceding and following spaces
func (n *nodeUtils) TrimStrings(config types.Configuration) {
	for key, value := range config {
		if strValue, ok := value.(string); ok {
			config[key] = strings.TrimSpace(strValue)
		}
	}
}

// SharedNode is a shared resource component. By obtaining a shared instance through Get, multiple nodes can obtain the same instance in the shared pool
// For example: MQTT client, database client, HTTP server, and reusable nodes.
type SharedNode[T any] struct {
	//Node type
	NodeType string
	//Configuration
	RuleConfig types.Config
	//Resource ID
	InstanceId string
	//Initialize the instance resource function
	InitInstanceFunc func() (T, error)
	//Cleanup resource callback function
	CloseFunc func(T) error
	//Initialize resources to prevent concurrent initialization
	//lock int32
	//Whether to obtain from the resource pool
	isFromPool bool
	Locker     sync.RWMutex

	// Local client caching (using new APIs)
	localClient       T
	clientInitialized bool
}

// Init initialization. If resourcePath starts with ref://, it is obtained from the network resource pool; otherwise, initInstanceFunc is called for initialization
// initNow=true, initialization will occur immediately; otherwise, initialization occurs at GetInstance().
func (x *SharedNode[T]) Init(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error)) error {
	return x.InitWithClose(ruleConfig, nodeType, resourcePath, initNow, initInstanceFunc, func(T) error {
		return nil
	})
}

// InitWithClose initialization, supports custom cleanup functions
func (x *SharedNode[T]) InitWithClose(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error), closeFunc func(T) error) error {
	x.RuleConfig = ruleConfig
	x.NodeType = nodeType
	x.CloseFunc = closeFunc

	if instanceId := NodeUtils.GetInstanceId(ruleConfig, resourcePath); instanceId == "" {
		x.InitInstanceFunc = initInstanceFunc
		if initNow {
			//Non-resource pool method, initialization
			client, err := x.InitInstanceFunc()
			if err != nil {
				return err
			}
			x.Locker.Lock()
			defer x.Locker.Unlock()
			// Initialization successful, cache the client
			x.localClient = client
			x.clientInitialized = true
			return nil
		}
	} else {
		x.isFromPool = true
		x.InstanceId = instanceId
	}
	return nil
}

// Has IsInit been initialized?
func (x *SharedNode[T]) IsInit() bool {
	return x.NodeType != ""
}

// GetInstance retrieves the shared instance
func (x *SharedNode[T]) GetInstance() (interface{}, error) {
	return x.GetSafely()
}

// Get the shared instance and return the specific type
// Deprecated: It is recommended to use the GetSafely() method, which offers better concurrency performance and resource management.
// When using GetSafely(), you need to use InitWithClose() and Close() methods for comprehensive resource management.
//func (x *SharedNode[T]) Get() (T, error) {
//	if x.InstanceId != "" {
//		Obtained from the network resource pool
//		if x.RuleConfig.NodePool == nil {
//			return zeroValue[T](), ErrNodePoolNil
//		}
//		if p, err := x.RuleConfig.NodePool.GetInstance(x.InstanceId); err == nil {
//			return p.(T), nil
//		} else {
//			return zeroValue[T](), err
//		}
//	} else if x.InitInstanceFunc != nil {
//		Initialize a client based on the current component configuration
//		return x.InitInstanceFunc()
//	} else {
//		return zeroValue[T](), ErrClientNotInit
//	}
//}

// GetSafely securely retrieves a shared instance; if there are no instances, it initializes one
// Recommend new components using this method for resource management.
//
// Instructions for use:
// 1. Use the InitWithClose() method during initialization and provide a cleanup function
// 2. Use the GetSafely() method when retrieving instances
// 3. When components are destroyed, the Close() method is called to clean up resources
func (x *SharedNode[T]) GetSafely() (T, error) {
	if x.InstanceId != "" {
		//Obtained from the network resource pool
		if x.RuleConfig.NodePool == nil {
			return zeroValue[T](), ErrNodePoolNil
		}
		if p, err := x.RuleConfig.NodePool.GetInstance(x.InstanceId); err == nil {
			return p.(T), nil
		} else {
			return zeroValue[T](), err
		}
	} else if x.InitInstanceFunc != nil {
		// First, use lock reading to check whether the client already exists
		x.Locker.RLock()
		if x.clientInitialized {
			client := x.localClient
			x.Locker.RUnlock()
			return client, nil
		}
		x.Locker.RUnlock()

		// The client does not exist and is created using a write lock
		x.Locker.Lock()
		defer x.Locker.Unlock()

		// Double-check: While waiting for the write lock, another goroutine may have already created a client
		if x.clientInitialized {
			return x.localClient, nil
		}

		// Initialize the client
		client, err := x.InitInstanceFunc()
		if err != nil {
			// Initialization failed. If a partially initialized client is returned, try cleaning up
			if !isZeroValue(client) && x.CloseFunc != nil {
				_ = x.CloseFunc(client)
			}
			return zeroValue[T](), err
		}

		// Initialization successful, cache the client
		x.localClient = client
		x.clientInitialized = true
		return client, nil
	} else {
		return zeroValue[T](), ErrClientNotInit
	}
}

// isZeroValue checks whether the value is zero
// Use reflection to safely compare values and avoid runtime panic on non-comparable types
func isZeroValue[T any](v T) bool {
	// Use reflection to safely check the zero value
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return true
	}
	return rv.IsZero()
}

// Close: Clean client resources from the local cache
// Used together with GetSafely() and InitWithClose() to provide complete resource lifecycle management
// Note: This method does not affect clients obtained from the resource pool
func (x *SharedNode[T]) Close() error {
	// Only clean clients in the local cache, without affecting clients in the resource pool
	if x.InstanceId != "" {
		// Resource pool mode, no need to clean up local clients
		return nil
	}

	x.Locker.Lock()
	defer x.Locker.Unlock()

	if x.clientInitialized {
		client := x.localClient

		// Use user-provided cleanup functions or the default Close method
		var err error
		if x.CloseFunc != nil {
			err = x.CloseFunc(client)
		} else {
			// Try calling the client's Close method (if available)
			if closer, ok := any(client).(interface{ Close() error }); ok {
				err = closer.Close()
			}
		}

		// Reset the local client state
		x.clientInitialized = false
		x.localClient = zeroValue[T]()

		return err
	}

	return nil
}

// IsFromPool is obtained from the resource pool
func (x *SharedNode[T]) IsFromPool() bool {
	return x.isFromPool
}

func (x *SharedNode[T]) Initialized() bool {
	x.Locker.RLock()
	defer x.Locker.RUnlock()
	return x.clientInitialized
}

// The zeroValue function is used to return a zero value of type T
func zeroValue[T any]() T {
	var zero T
	return zero
}
