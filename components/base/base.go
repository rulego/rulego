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
	"fmt"
	"github.com/rulego/rulego/utils/json"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
)

var (
	ErrNodePoolNil   = errors.New("node pool is nil")
	ErrClientNotInit = errors.New("client not init")
)

// DefaultInitFailRetryInterval is the default cooldown before retrying a failed init.
const DefaultInitFailRetryInterval = 30 * time.Second

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
		//截取资源ID
		return server[len(types.NodeConfigurationPrefixInstanceId):]
	}
	return ""
}

func (n *nodeUtils) IsInitNetResource(_ types.Config, configuration types.Configuration) bool {
	_, ok := configuration[types.NodeConfigurationKeyIsInitNetResource]
	return ok
}

func (n *nodeUtils) getEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	// 直接调用ctx的GetEvnAndMetadata方法
	return ctx.GetEnv(msg, useMetadata)
}

// GetDataByType 准备传递给JavaScript脚本的数据
// 根据消息的数据类型进行不同的处理：
// - JSON类型：解析为map以便JavaScript处理
// - BINARY类型：转换为字节数组，JavaScript将其视为Uint8Array
// - 其他类型：使用原始字符串数据
func (n *nodeUtils) GetDataByType(msg types.RuleMsg, readOnly bool) interface{} {
	var data interface{}
	// 根据数据类型进行不同的处理
	switch msg.DataType {
	case types.JSON:
		if readOnly {
			if dataMap, err := msg.GetJsonData(); err == nil {
				data = dataMap
			} else {
				data = msg.GetData()
			}
		} else {
			// JSON类型：js会修改数据，所以这里需要重新解析
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
			// 二进制类型：创建字节数组副本以避免并发修改问题，JavaScript会将其视为Uint8Array
			originalBytes := msg.GetBytes()
			if originalBytes != nil {
				// 创建副本以确保并发安全
				copyBytes := make([]byte, len(originalBytes))
				copy(copyBytes, originalBytes)
				data = copyBytes
			} else {
				data = originalBytes
			}
		}

	default:
		// 其他类型：使用原始字符串数据
		data = msg.GetData()
	}

	return data
}

// TrimStrings 去除配置中所有字符串值的前后空格
// 遍历 Configuration 中的所有值，如果是字符串类型则去除前后空格
func (n *nodeUtils) TrimStrings(config types.Configuration) {
	for key, value := range config {
		if strValue, ok := value.(string); ok {
			config[key] = strings.TrimSpace(strValue)
		}
	}
}

// connHolder 是同链连接池的稳定间接层：注册到链资源目录的是 *connHolder[T]，
// 而非裸连接 T。重连时只更新 holder 内部值、目录条目恒定——借用方永拿最新连接，
// 且 Close 时可按 holder 指针做 CAS 注销（防 id 碰撞误删别人的条目）。
type connHolder[T any] struct {
	mu sync.RWMutex
	v  T
	// status snapshot exposed to chain-scoped borrowers
	status types.NodeStatus
	msg    string
}

func (h *connHolder[T]) load() T {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.v
}

func (h *connHolder[T]) store(c T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.v = c
}

func (h *connHolder[T]) storeStatus(s types.NodeStatus, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status = s
	h.msg = msg
}

func (h *connHolder[T]) loadStatus() types.StatusInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return types.StatusInfo{Status: h.status, Message: h.msg}
}

// SharedNode 共享资源组件，通过 Get 获取共享实例，多个节点可以在共享池中获取相同的实例
// 例如：mqtt 客户端、数据库客户端，也可以http server以及是可复用的节点。
type SharedNode[T any] struct {
	//节点类型
	NodeType string
	//配置
	RuleConfig types.Config
	//资源ID
	InstanceId string
	//初始化实例资源函数
	InitInstanceFunc func() (T, error)
	//清理资源的回调函数
	CloseFunc func(T) error
	// InitFailRetryInterval is the cooldown after a failed init; <=0 falls back to DefaultInitFailRetryInterval.
	InitFailRetryInterval time.Duration
	////初始化资源资源，防止并发初始化
	//lock int32
	//是否从资源池获取
	isFromPool bool
	Locker     sync.RWMutex

	// 本地客户端缓存（新API使用）
	localClient       T
	clientInitialized bool

	// recorded init failure: within the cooldown, the last error is returned without retrying
	initFailErr  error
	initFailTime time.Time

	// 同链连接池（Chain-Scoped Connection Pool）相关字段：
	// chainCtx 为本节点部署所属链（固定，不随消息流漂移）；nil 表示未启用同链能力（退化为旧行为）。
	chainCtx types.ChainCtx
	// nodeId 作为同链目录注册 key（= 节点 ID）。
	nodeId string
	// holder is the chain-scoped registry entry; holds *connHolder[T].
	holder atomic.Value
	// isRegistered 是否已注册到同链目录（防重复注册）。
	isRegistered bool

	// Connection status. status is atomic (types.NodeStatus); statusMsg is
	// guarded by statusMu, decoupled from x.Locker so SetStatus is safe to
	// call from within GetSafely (which holds x.Locker), e.g. inside an
	// InitInstanceFunc callback.
	status    int32
	statusMsg string
	statusMu  sync.RWMutex
}

// Init 初始化，如果 resourcePath 为 ref:// 开头，则从网络资源池获取，否则调用 initInstanceFunc 初始化
// initNow=true，会在立刻初始化，否则在 GetInstance() 时候初始化
func (x *SharedNode[T]) Init(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error)) error {
	return x.InitWithClose(ruleConfig, nodeType, resourcePath, initNow, initInstanceFunc, func(T) error {
		return nil
	})
}

// InitWithClose initializes with a custom cleanup function.
// When initNow is true, an init failure is returned as-is so callers can
// gate startup on connectivity (e.g. node components honoring NodeClientInitNow).
// For tolerant startup where the dependency may not be ready yet, use InitWithCloseSoftFail.
func (x *SharedNode[T]) InitWithClose(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error), closeFunc func(T) error) error {
	return x.initShared(ruleConfig, nodeType, resourcePath, initNow, initInstanceFunc, closeFunc, false)
}

// InitWithCloseSoftFail is like InitWithClose, but when initNow is true an init
// failure does not return an error: the status is set to Reconnecting and the
// next GetSafely retries after the cooldown. Use when the dependency may become
// ready after this service starts (e.g. an MQTT broker not yet up).
func (x *SharedNode[T]) InitWithCloseSoftFail(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error), closeFunc func(T) error) error {
	return x.initShared(ruleConfig, nodeType, resourcePath, initNow, initInstanceFunc, closeFunc, true)
}

func (x *SharedNode[T]) initShared(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error), closeFunc func(T) error, softFail bool) error {
	x.RuleConfig = ruleConfig
	x.NodeType = nodeType
	x.CloseFunc = closeFunc

	if instanceId := NodeUtils.GetInstanceId(ruleConfig, resourcePath); instanceId == "" {
		x.InitInstanceFunc = initInstanceFunc
		if initNow {
			client, err := x.InitInstanceFunc()
			if err != nil {
				x.Locker.Lock()
				x.initFailErr = err
				x.initFailTime = time.Now()
				x.Locker.Unlock()
				x.setStatusLocked(types.StatusReconnecting, err.Error())
				if softFail {
					return nil
				}
				return err
			}
			x.Locker.Lock()
			defer x.Locker.Unlock()
			x.localClient = client
			x.clientInitialized = true
			x.setStatusLocked(types.StatusConnected, "")
			return nil
		}
	} else {
		x.isFromPool = true
		x.InstanceId = instanceId
	}
	return nil
}

// IsInit 是否初始化过
func (x *SharedNode[T]) IsInit() bool {
	return x.NodeType != ""
}

// BindChain 绑定节点部署所属链，启用同链连接池能力。
// 由组件 Init 在 InitWithClose 之后调用：从 configuration 取 chainCtx 与节点 ID。
// 未调用则 chainCtx=nil，GetSafely/Close 退化为旧行为（仅 NodePool / 本地）。
func (x *SharedNode[T]) BindChain(configuration types.Configuration) {
	ctx := NodeUtils.GetChainCtx(configuration)
	// nil 指针清洗：types.ChainCtx 是接口，防御"非 nil 接口包 nil 指针"陷阱。
	if ctx != nil {
		if rv := reflect.ValueOf(ctx); rv.Kind() == reflect.Ptr && rv.IsNil() {
			ctx = nil
		}
	}
	x.chainCtx = ctx
	x.nodeId = NodeUtils.GetSelfDefinition(configuration).Id
	// 若 InitWithClose(initNow=true) 已提前建连，补注册到同链目录。
	if ctx != nil && x.nodeId != "" {
		x.Locker.Lock()
		if x.clientInitialized && !x.isRegistered {
			x.registerUnderLock(x.localClient)
		}
		x.Locker.Unlock()
	}
}

// Refresh 重连后更新本地与同链目录中的连接：目录条目不变，仅更新 holder 内部值。
// 供连接型组件（modbus/net/ws）重连重建 client 后调用，借用方下次取连接即拿最新。
func (x *SharedNode[T]) Refresh(newClient T) {
	x.Locker.Lock()
	defer x.Locker.Unlock()
	x.localClient = newClient
	x.clientInitialized = true
	x.initFailErr = nil
	if h, _ := x.holder.Load().(*connHolder[T]); h != nil {
		h.store(newClient)
	}
	x.setStatusLocked(types.StatusConnected, "")
}

// registerUnderLock 在已持有 x.Locker 写锁的前提下，将 client 注册为同链源。
// 幂等：未启用同链能力或已注册时直接返回。
func (x *SharedNode[T]) registerUnderLock(client T) {
	if x.chainCtx == nil || x.nodeId == "" || x.isRegistered {
		return
	}
	h, _ := x.holder.Load().(*connHolder[T])
	if h == nil {
		h = &connHolder[T]{}
		x.holder.Store(h)
	}
	h.store(client)
	x.chainCtx.ResourceRegistry().Register(x.nodeId, h)
	x.isRegistered = true
}

// unpackHolder 将同链目录命中的实例解包为连接（断言 connHolder + load）。
// 类型不符或连接为 nil 返回错误（不静默回退，暴露跨类型 ref / id 碰撞等配置错误）。
func (x *SharedNode[T]) unpackHolder(inst any) (T, error) {
	h, ok := inst.(*connHolder[T])
	if !ok {
		return zeroValue[T](), fmt.Errorf("chain resource %s type %T is incompatible", x.InstanceId, inst)
	}
	c := h.load()
	if isZeroValue(c) {
		return zeroValue[T](), fmt.Errorf("chain resource %s connection is nil", x.InstanceId)
	}
	return c, nil
}

// GetInstance 获取共享实例
func (x *SharedNode[T]) GetInstance() (interface{}, error) {
	return x.GetSafely()
}

// Get 获取共享实例，并返回具体类型
// Deprecated: 建议使用 GetSafely() 方法，该方法提供更好的并发性能和资源管理。
// 使用 GetSafely() 时需要配合 InitWithClose() 和 Close() 方法进行完整的资源管理。
//func (x *SharedNode[T]) Get() (T, error) {
//	if x.InstanceId != "" {
//		//从网络资源池获取
//		if x.RuleConfig.NodePool == nil {
//			return zeroValue[T](), ErrNodePoolNil
//		}
//		if p, err := x.RuleConfig.NodePool.GetInstance(x.InstanceId); err == nil {
//			return p.(T), nil
//		} else {
//			return zeroValue[T](), err
//		}
//	} else if x.InitInstanceFunc != nil {
//		//根据当前组件配置初始化一个客户端
//		return x.InitInstanceFunc()
//	} else {
//		return zeroValue[T](), ErrClientNotInit
//	}
//}

// GetSafely 安全获取共享实例，如果没有实例则初始化一个
// 推荐新组件使用此方法进行资源管理。
//
// 使用说明：
// 1. 初始化时使用 InitWithClose() 方法并提供清理函数
// 2. 获取实例时使用 GetSafely() 方法
// 3. 组件销毁时调用 Close() 方法清理资源
func (x *SharedNode[T]) GetSafely() (T, error) {
	if x.InstanceId != "" {
		// ref:// 借用模式
		// ① 同链目录优先（部署链 chainCtx，固定不随消息流漂移）
		if x.chainCtx != nil {
			if inst, found := x.chainCtx.Resources().Lookup(x.InstanceId); found {
				return x.unpackHolder(inst)
			}
		}
		// ② NodePool 回退（comma-ok，类型不符报错而非 panic）
		if x.RuleConfig.NodePool == nil {
			return zeroValue[T](), ErrNodePoolNil
		}
		p, err := x.RuleConfig.NodePool.GetInstance(x.InstanceId)
		if err != nil {
			return zeroValue[T](), err
		}
		t, ok := p.(T)
		if !ok {
			return zeroValue[T](), fmt.Errorf("node pool resource %s type %T is incompatible", x.InstanceId, p)
		}
		return t, nil
	} else if x.InitInstanceFunc != nil {
		// 本地模式：懒建连（双重检查锁）
		// 首先使用读锁检查客户端是否已存在
		x.Locker.RLock()
		if x.clientInitialized {
			client := x.localClient
			x.Locker.RUnlock()
			return client, nil
		}
		x.Locker.RUnlock()

		// 客户端不存在，使用写锁进行创建
		x.Locker.Lock()
		defer x.Locker.Unlock()

		// 双重检查：可能在等待写锁期间其他goroutine已经创建了客户端
		if x.clientInitialized {
			return x.localClient, nil
		}

		// Fast-fail within the cooldown window: return the last error without retrying init.
		if x.initFailErr != nil && time.Since(x.initFailTime) < x.initRetryInterval() {
			return zeroValue[T](), x.initFailErr
		}

		// 初始化客户端
		client, err := x.InitInstanceFunc()
		if err != nil {
			// record failure for cooldown fast-fail
			x.initFailErr = err
			x.initFailTime = time.Now()
			x.setStatusLocked(types.StatusReconnecting, err.Error())
			// 初始化失败，如果返回了部分初始化的客户端，尝试清理
			if !isZeroValue(client) && x.CloseFunc != nil {
				_ = x.CloseFunc(client)
			}
			return zeroValue[T](), err
		}

		// init succeeded: clear failure record and cache the client
		x.initFailErr = nil
		x.localClient = client
		x.clientInitialized = true
		// 首次建连成功，注册为同链源（key=nodeId），供链内 ref:// 节点借用
		x.registerUnderLock(client)
		x.setStatusLocked(types.StatusConnected, "")
		return client, nil
	} else {
		return zeroValue[T](), ErrClientNotInit
	}
}

// initRetryInterval returns the cooldown applied after a failed init.
func (x *SharedNode[T]) initRetryInterval() time.Duration {
	if x.InitFailRetryInterval > 0 {
		return x.InitFailRetryInterval
	}
	return DefaultInitFailRetryInterval
}

// isZeroValue 检查值是否为零值
// 使用反射来安全地比较值，避免在不可比较类型上出现运行时恐慌
func isZeroValue[T any](v T) bool {
	// 使用反射来安全地检查零值
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return true
	}
	return rv.IsZero()
}

// Close 清理本地缓存的客户端资源
// 与 GetSafely() 和 InitWithClose() 配合使用，提供完整的资源生命周期管理
// 注意：此方法不会影响从资源池获取的客户端
func (x *SharedNode[T]) Close() error {
	// ref:// 借用方不管连接生命周期（连接归源节点所有）
	if x.InstanceId != "" {
		return nil
	}

	x.Locker.Lock()
	defer x.Locker.Unlock()

	if !x.clientInitialized {
		// never connected: clear the failure record to allow re-init
		x.initFailErr = nil
		x.setStatusLocked(types.StatusDisconnected, "")
		return nil
	}
	client := x.localClient

	// 先从同链目录摘除可见性（软 CAS：仅当仍是自己的 holder 才删，防 id 碰撞误删别人）。
	// 先摘可见性再关连接，缩小借用方拿到正在关闭连接的窗口。
	if h, _ := x.holder.Load().(*connHolder[T]); x.isRegistered && x.chainCtx != nil && h != nil {
		reg := x.chainCtx.ResourceRegistry()
		if cur, found := reg.Lookup(x.nodeId); found && cur == h {
			reg.Unregister(x.nodeId)
		}
		x.isRegistered = false
	}

	// 再关闭连接
	var err error
	if x.CloseFunc != nil {
		err = x.CloseFunc(client)
	} else {
		// 尝试调用客户端的Close方法（如果有的话）
		if closer, ok := any(client).(interface{ Close() error }); ok {
			err = closer.Close()
		}
	}

	// 重置本地客户端状态
	x.clientInitialized = false
	x.localClient = zeroValue[T]()
	x.setStatusLocked(types.StatusDisconnected, "")
	x.holder.Store((*connHolder[T])(nil))
	x.initFailErr = nil

	return err
}

// IsFromPool 是否从资源池获取
func (x *SharedNode[T]) IsFromPool() bool {
	return x.isFromPool
}

func (x *SharedNode[T]) Initialized() bool {
	x.Locker.RLock()
	defer x.Locker.RUnlock()
	return x.clientInitialized
}

// setStatusLocked updates status and syncs it to the chain-scoped holder.
func (x *SharedNode[T]) setStatusLocked(s types.NodeStatus, msg string) {
	atomic.StoreInt32(&x.status, int32(s))
	x.statusMu.Lock()
	x.statusMsg = msg
	h, _ := x.holder.Load().(*connHolder[T])
	x.statusMu.Unlock()
	if h != nil {
		h.storeStatus(s, msg)
	}
}

// SetStatus updates the connection status from disconnect/reconnect events
// (e.g. net/ws self-reconnect). It does not acquire x.Locker, so it is safe to
// call from within an InitInstanceFunc callback running under GetSafely.
func (x *SharedNode[T]) SetStatus(s types.NodeStatus, msg string) {
	x.setStatusLocked(s, msg)
}

// ConnectionStatus implements types.ConnectionStatusGetter.
// A ref:// borrower delegates to the holder: chain-scoped holder snapshot first, then the shared-pool source node. It never triggers a connection.
func (x *SharedNode[T]) ConnectionStatus() types.StatusInfo {
	if x.InstanceId != "" {
		if x.chainCtx != nil {
			if inst, found := x.chainCtx.Resources().Lookup(x.InstanceId); found {
				if h, ok := inst.(*connHolder[T]); ok {
					return h.loadStatus()
				}
			}
		}
		if x.RuleConfig.NodePool != nil {
			if ctx, ok := x.RuleConfig.NodePool.Get(x.InstanceId); ok {
				if s, ok := ctx.GetNode().(types.ConnectionStatusGetter); ok {
					return s.ConnectionStatus()
				}
			}
		}
	}
	x.statusMu.RLock()
	msg := x.statusMsg
	x.statusMu.RUnlock()
	return types.StatusInfo{Status: types.NodeStatus(atomic.LoadInt32(&x.status)), Message: msg}
}

// Instance returns the initialized client without triggering lazy init; zero value and false if not initialized.
func (x *SharedNode[T]) Instance() (T, bool) {
	x.Locker.RLock()
	defer x.Locker.RUnlock()
	if x.clientInitialized {
		return x.localClient, true
	}
	return zeroValue[T](), false
}

// zeroValue 函数用于返回 T 类型的零值
func zeroValue[T any]() T {
	var zero T
	return zero
}
