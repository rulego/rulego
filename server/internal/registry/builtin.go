package registry

import (
	"errors"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
)

var (
	builtins        = make(map[string]interface{})
	globalUdfs      = make(map[string]interface{})
	dynamicBuiltins = make(map[string]DynamicBuiltin)
	lock            sync.RWMutex
	AiToolsProvider func(c types.Config) []interface{}
)

// DynamicBuiltin 动态组件配置查询处理器（运行时需要参数，如 OPC UA 在线浏览地址空间）
type DynamicBuiltin func(params map[string]interface{}) (interface{}, error)

// RegisterDynamicBuiltin 注册动态组件配置查询处理器
func RegisterDynamicBuiltin(name string, fn DynamicBuiltin) {
	lock.Lock()
	defer lock.Unlock()
	dynamicBuiltins[name] = fn
}

// QueryDynamicBuiltin 执行动态组件配置查询，未注册返回错误
func QueryDynamicBuiltin(name string, params map[string]interface{}) (interface{}, error) {
	lock.RLock()
	fn := dynamicBuiltins[name]
	lock.RUnlock()
	if fn == nil {
		return nil, errors.New("dynamic builtin not found: " + name)
	}
	return fn(params)
}

func init() {
	builtins["functions"] = map[string]interface{}{
		"functionName": action.Functions.Names(),
	}
}

// Builtins 获取内置组件配置选项
func Builtins() map[string]interface{} {
	lock.RLock()
	defer lock.RUnlock()
	data := make(map[string]interface{})
	for k, v := range builtins {
		if f, ok := v.(func() interface{}); ok {
			data[k] = f()
		} else {
			data[k] = v
		}
	}
	return data
}

// RegisterBuiltin 注册内置组件配置选项
func RegisterBuiltin(name string, value interface{}) {
	lock.Lock()
	defer lock.Unlock()
	builtins[name] = value
}

// RegisterGlobalUdf 注册全局 UDF
func RegisterGlobalUdf(name string, value interface{}) {
	lock.Lock()
	defer lock.Unlock()
	globalUdfs[name] = value
}

// GlobalUdfs 获取全局 UDF
func GlobalUdfs() map[string]interface{} {
	lock.RLock()
	defer lock.RUnlock()
	data := make(map[string]interface{})
	for k, v := range globalUdfs {
		data[k] = v
	}
	return data
}
