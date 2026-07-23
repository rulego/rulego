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
	// functions: node components
	builtins["functions"] = map[string]interface{}{
		//Function name options
		"functionName": action.Functions.Names(),
	}
}

// Builtins get built-in component configuration options
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

// RegisterBuiltin Builtin Built-in Component Configuration Options
// value can be a static value or a func() interface{} function used to obtain data in real time
func RegisterBuiltin(name string, value interface{}) {
	lock.Lock()
	defer lock.Unlock()
	builtins[name] = value
}
