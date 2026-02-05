package service

import (
	"sync"

	"github.com/rulego/rulego/components/action"
)

var (
	builtins = make(map[string]interface{})
	lock     sync.RWMutex
)

func init() {
	// functions节点组件
	builtins["functions"] = map[string]interface{}{
		//函数名选项
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
// value 可以是静态值，也可以是 func() interface{} 类型的函数，用于实时获取数据
func RegisterBuiltin(name string, value interface{}) {
	lock.Lock()
	defer lock.Unlock()
	builtins[name] = value
}
