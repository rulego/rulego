package app

import (
	"fmt"
	"reflect"
	"sync"
)

// Container: Lightweight service container, registered by name + generic acquisition model
// Container is a lightweight service container using name-based registration and generic retrieval
type Container struct {
	services map[string]any
	mu       sync.RWMutex
}

// NewContainer creates a new service container
// NewContainer creates a new service container
func NewContainer() *Container {
	return &Container{
		services: make(map[string]any),
	}
}

// Register registers a service into the container and does not allow duplicate overriding
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

// Replace: Replaces the registered services in the container, used only for explicit coverage
// Replace replaces a registered service in the container, intended for explicit override points
func (c *Container) Replace(name string, svc any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = svc
}

// Get the service with a specified name
// Get retrieves a service by name
func (c *Container) Get(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	svc, ok := c.services[name]
	return svc, ok
}

// GetAs generics get a service with a specified name; if the service does not exist or the type does not match, an error is returned
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
			// When T is the interface type, zero is nil, and use reflect.TypeOf(svc) interface set check
			expected = reflect.TypeOf((*T)(nil)).Elem()
		}
		return zero, fmt.Errorf("service %q type mismatch: expected %s, got %T", name, expected, svc)
	}
	return typed, nil
}

// MustGetAs generics get a service named with a specified name; if the service does not exist or the type does not match, it panics
// MustGetAs retrieves a service by name with generic type, panics if not found or type mismatch
func MustGetAs[T any](c *Container, name string) T {
	svc, err := GetAs[T](c, name)
	if err != nil {
		panic(err)
	}
	return svc
}

// Names returns the names of all registered services in the container
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
