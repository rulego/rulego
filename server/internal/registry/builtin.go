package registry

import (
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
)

var (
	builtins        = make(map[string]interface{})
	globalUdfs      = make(map[string]interface{})
	lock            sync.RWMutex
	AiToolsProvider func(c types.Config) []interface{}
)

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
