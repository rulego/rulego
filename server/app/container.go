package app

import (
	"fmt"
	"reflect"
	"sync"
)

// Container 轻量级服务容器，使用名字注册 + 泛型获取模型
// Container is a lightweight service container using name-based registration and generic retrieval
type Container struct {
	services map[string]any
	mu       sync.RWMutex
}

// NewContainer 创建一个新的服务容器
// NewContainer creates a new service container
func NewContainer() *Container {
	return &Container{
		services: make(map[string]any),
	}
}

// Register 注册一个服务到容器中，不允许重复覆盖
// Register registers a service into the container, duplicate names are not allowed
func (c *Container) Register(name string, svc any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.services[name]; exists {
		return fmt.Errorf("service %q already registered", name)
	}
	c.services[name] = svc
	return nil
}

// Replace 替换容器中已注册的服务，仅用于明确的覆盖点
// Replace replaces a registered service in the container, intended for explicit override points
func (c *Container) Replace(name string, svc any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = svc
}

// Get 获取指定名称的服务
// Get retrieves a service by name
func (c *Container) Get(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	svc, ok := c.services[name]
	return svc, ok
}

// GetAs 泛型获取指定名称的服务，如果服务不存在或类型不匹配则返回错误
// GetAs retrieves a service by name with generic type, returns error if not found or type mismatch
func GetAs[T any](c *Container, name string) (T, error) {
	var zero T
	svc, ok := c.Get(name)
	if !ok {
		return zero, fmt.Errorf("service %q not found", name)
	}
	typed, ok := svc.(T)
	if !ok {
		expected := reflect.TypeOf(zero)
		if expected == nil {
			// T 是接口类型时 zero 为 nil，用 reflect.TypeOf(svc) 的接口集判断
			expected = reflect.TypeOf((*T)(nil)).Elem()
		}
		return zero, fmt.Errorf("service %q type mismatch: expected %s, got %T", name, expected, svc)
	}
	return typed, nil
}

// MustGetAs 泛型获取指定名称的服务，如果服务不存在或类型不匹配则 panic
// MustGetAs retrieves a service by name with generic type, panics if not found or type mismatch
func MustGetAs[T any](c *Container, name string) T {
	svc, err := GetAs[T](c, name)
	if err != nil {
		panic(err)
	}
	return svc
}

// Names 返回容器中所有已注册的服务名称
// Names returns all registered service names in the container
func (c *Container) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}
	return names
}
