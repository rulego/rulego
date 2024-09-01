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

package base

import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
	"strings"
	"sync/atomic"
)

var (
	ErrNetPoolNil    = errors.New("net pool is nil")
	ErrClientNotInit = errors.New("client not init")
)

var NodeUtils = &nodeUtils{}

type nodeUtils struct {
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

// GetEvnAndMetadata 和Metadata key合并
func (n *nodeUtils) GetEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return n.getEvnAndMetadata(ctx, msg, true)
}

func (n *nodeUtils) IsNetPool(config types.Config, server string) bool {
	return strings.HasPrefix(server, types.NodeConfigurationPrefixInstanceId)
}

func (n *nodeUtils) GetInstanceId(config types.Config, server string) string {
	if n.IsNetPool(config, server) {
		//截取资源ID
		return server[len(types.NodeConfigurationPrefixInstanceId):]
	}
	return ""
}

func (n *nodeUtils) IsInitNetResource(_ types.Config, configuration types.Configuration) bool {
	_, ok := configuration[types.NodeConfigurationKeyIsInitNetResource]
	return ok
}

func (n *nodeUtils) getEvnAndMetadata(_ types.RuleContext, msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	var data interface{}
	if msg.DataType == types.JSON {
		// 解析 JSON 字符串到 map
		if err := json.Unmarshal([]byte(msg.Data), &data); err != nil {
			// 解析失败，使用原始数据
			data = msg.Data
		}
	} else {
		// 如果不是 JSON 类型，直接使用原始数据
		data = msg.Data
	}
	var evn = make(map[string]interface{})
	evn[types.IdKey] = msg.Id
	evn[types.TsKey] = msg.Ts
	evn[types.DataKey] = msg.Data
	evn[types.MsgKey] = data
	evn[types.MetadataKey] = map[string]string(msg.Metadata)
	evn[types.MsgTypeKey] = msg.Type
	evn[types.TypeKey] = msg.Type
	evn[types.DataTypeKey] = msg.DataType
	if useMetadata {
		for k, v := range msg.Metadata {
			evn[k] = v
		}
	}
	return evn
}

// SharedNode 共享资源组件，通过GetInstance获取共享实例，多个节点可以在共享池中获取相同的实例，例如：mqtt、数据库客户端，也可以http server以及是可复用的节点.
type SharedNode[T any] struct {
	NodeType   string
	RuleConfig types.Config
	//资源ID
	InstanceId string
	//初始化实例资源函数
	InitInstanceFunc func() (T, error)
	//是否正在连接资源
	connecting int32
	//是否从资源池获取
	isFromPool bool
}

// Init 初始化，如果server 为 ref:// 开头，则从网络资源池获取，否则调用initNetResourceFunc初始化
func (x *SharedNode[T]) Init(ruleConfig types.Config, nodeType, server string, initNow bool, initInstanceFunc func() (T, error)) error {
	x.RuleConfig = ruleConfig
	x.NodeType = nodeType

	if instanceId := NodeUtils.GetInstanceId(ruleConfig, server); instanceId == "" {
		x.InitInstanceFunc = initInstanceFunc
		if initNow {
			//非资源池方式，初始化
			_, err := x.InitInstanceFunc()
			return err
		}
	} else {
		x.InstanceId = instanceId
	}
	return nil
}

// IsInit 是否初始化过
func (x *SharedNode[T]) IsInit() bool {
	return x.NodeType != ""
}

func (x *SharedNode[T]) GetInstance() (T, error) {
	if x.InstanceId != "" {
		//从网络资源池获取
		if x.RuleConfig.NetPool == nil {
			return zeroValue[T](), ErrNetPoolNil
		}
		if p, err := x.RuleConfig.NetPool.GetInstance(x.InstanceId); err == nil {
			x.isFromPool = true
			return p.(T), nil
		} else {
			return zeroValue[T](), err
		}
	} else if x.InitInstanceFunc != nil {
		//根据当前组件配置初始化一个客户端
		return x.InitInstanceFunc()
	} else {
		return zeroValue[T](), ErrClientNotInit
	}
}

// TryLock 尝试连接中
func (x *SharedNode[T]) TryLock() bool {
	return atomic.CompareAndSwapInt32(&x.connecting, 0, 1)
}

// ReleaseLock 连接完成
func (x *SharedNode[T]) ReleaseLock() {
	atomic.StoreInt32(&x.connecting, 0)
}

// IsFromPool 是否从资源池获取
func (x *SharedNode[T]) IsFromPool() bool {
	return x.isFromPool
}

// zeroValue 函数用于返回 T 类型的零值
func zeroValue[T any]() T {
	var zero T
	return zero
}
