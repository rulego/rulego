/*
 * Copyright 2023 The RuleGo Authors.
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

package node_pool

import (
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
)

// TestRestSharedNodeBasicOperations 测试REST endpoint的基本SharedNode功能和多实例共享
func TestRestSharedNodeBasicOperations(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 子测试1：基本SharedNode功能
	t.Run("BasicSharedNodeFunctionality", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "shared_rest_endpoint",
		       "type": "endpoint/http",
		       "name": "共享REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9080"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 验证共享节点已创建
		sharedCtx, ok := pool.Get("shared_rest_endpoint")
		assert.True(t, ok)
		assert.NotNil(t, sharedCtx)

		// 获取REST实例
		restInstance, err := pool.GetInstance("shared_rest_endpoint")
		assert.Nil(t, err)
		assert.NotNil(t, restInstance)

		// 验证实例类型和配置
		restEndpoint, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.NotNil(t, restEndpoint)
		assert.Equal(t, ":9080", restEndpoint.Config.Server)

		// 清理
		pool.Del("shared_rest_endpoint")
		assert.Equal(t, 0, len(pool.GetAll()))
	})

	// 子测试2：多实例共享验证
	t.Run("MultipleInstancesSharing", func(t *testing.T) {
		// 创建共享的REST服务器节点
		var sharedServerDsl = []byte(`
			{
		       "id": "shared_rest_server",
		       "type": "endpoint/http",
		       "name": "共享REST服务器",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9081"
		       }
		     }`)

		var sharedServerDef types.EndpointDsl
		err := json.Unmarshal(sharedServerDsl, &sharedServerDef)
		assert.Nil(t, err)
		sharedCtx, err := pool.NewFromEndpoint(sharedServerDef)
		assert.NotNil(t, sharedCtx)
		assert.Nil(t, err)

		// 验证只有一个共享服务器节点存在
		assert.Equal(t, 1, len(pool.GetAll()))

		// 获取共享服务器实例
		sharedInstance, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		sharedRest, ok := sharedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.Equal(t, ":9081", sharedRest.Config.Server)

		// 验证多次获取返回同一个实例
		instance1, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		instance2, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		assert.Equal(t, instance1, instance2) // 应该是同一个实例

		// 验证不存在的节点返回错误
		_, err = pool.GetInstance("non_existent_node")
		assert.NotNil(t, err)

		// 清理
		pool.Stop()
		assert.Equal(t, 0, len(pool.GetAll()))
	})
}

// TestRestSharedNodeLifecycleManagement 测试REST endpoint的生命周期管理（重启和注销）
func TestRestSharedNodeLifecycleManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 子测试1：重启功能测试
	t.Run("RestartFunctionality", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "restart_test_rest",
		       "type": "endpoint/http",
		       "name": "重启测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9082"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 获取REST实例
		restInstance, err := pool.GetInstance("restart_test_rest")
		assert.Nil(t, err)
		_, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)

		// 测试重启功能：删除旧节点并创建新节点
		pool.Del("restart_test_rest")
		time.Sleep(1 * time.Second)
		// 创建更新的配置
		var newRestDsl = []byte(`
			{
		       "id": "restart_test_rest",
		       "type": "endpoint/http",
		       "name": "重启测试REST端点-更新",
		       "debugMode": true,
		       "configuration": {
		         "server": ":9082",
		         "allowCors": true
		       }
		     }`)

		// 重新创建节点
		var newDef types.EndpointDsl
		err = json.Unmarshal(newRestDsl, &newDef)
		assert.Nil(t, err)
		newCtx, err := pool.NewFromEndpoint(newDef)
		assert.NotNil(t, newCtx)
		assert.Nil(t, err)

		// 验证配置已更新
		updatedInstance, err := pool.GetInstance("restart_test_rest")
		assert.Nil(t, err)
		updatedRest, ok := updatedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.True(t, updatedRest.Config.AllowCors)

		// 清理
		pool.Del("restart_test_rest")
	})

	// 子测试2：注销影响测试
	t.Run("UnregisterImpact", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "unregister_test_rest",
		       "type": "endpoint/http",
		       "name": "注销测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9083"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 验证节点存在
		_, ok := pool.Get("unregister_test_rest")
		assert.True(t, ok)
		assert.Equal(t, 1, len(pool.GetAll()))

		// 获取实例
		instance, err := pool.GetInstance("unregister_test_rest")
		assert.Nil(t, err)
		assert.NotNil(t, instance)

		// 注销节点
		pool.Del("unregister_test_rest")

		// 验证节点已被删除
		_, ok = pool.Get("unregister_test_rest")
		assert.False(t, ok)
		assert.Equal(t, 0, len(pool.GetAll()))

		// 尝试获取已删除的实例
		instance, err = pool.GetInstance("unregister_test_rest")
		assert.NotNil(t, err)
		assert.Nil(t, instance)
	})

	// 最终清理
	pool.Stop()
}

// TestRestSharedNodeAdvancedFeatures 测试REST endpoint的高级功能（路由和并发）
func TestRestSharedNodeAdvancedFeatures(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 子测试1：路由功能测试
	t.Run("RouteFunctionality", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "routes_test_rest",
		       "type": "endpoint/http",
		       "name": "路由测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9084"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 获取REST实例
		restInstance, err := pool.GetInstance("routes_test_rest")
		assert.Nil(t, err)
		restEndpoint, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)

		// 添加路由
		router := impl.NewRouter().From("/test").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			exchange.Out.SetBody([]byte("Hello from shared REST endpoint"))
			return true
		}).End()

		restEndpoint.GET(router)

		// 清理
		pool.Del("routes_test_rest")
	})

	// 子测试2：并发访问测试
	t.Run("ConcurrentAccess", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "concurrent_test_rest",
		       "type": "endpoint/http",
		       "name": "并发测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9085"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 并发获取实例
		const numGoroutines = 10
		results := make(chan interface{}, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				instance, err := pool.GetInstance("concurrent_test_rest")
				if err != nil {
					errors <- err
					return
				}
				results <- instance
			}()
		}

		// 收集结果
		var instances []interface{}
		for i := 0; i < numGoroutines; i++ {
			select {
			case instance := <-results:
				instances = append(instances, instance)
			case err := <-errors:
				t.Fatalf("Concurrent access failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent access")
			}
		}

		// 验证所有实例都是相同的（共享实例）
		assert.Equal(t, numGoroutines, len(instances))
		for i := 1; i < len(instances); i++ {
			assert.Equal(t, instances[0], instances[i])
		}

		// 清理
		pool.Del("concurrent_test_rest")
	})

	// 最终清理
	pool.Stop()
}

// TestRestSharedNodeWithRefProtocol 测试使用ref://方式引入共享REST endpoint及其生命周期管理
func TestRestSharedNodeWithRefProtocol(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 子测试1：基本ref://引用功能
	t.Run("BasicRefProtocol", func(t *testing.T) {
		// 创建共享节点
		var sharedRestDsl = []byte(`
			{
		       "id": "shared_rest_endpoint_ref",
		       "type": "endpoint/http",
		       "name": "共享REST端点-ref测试",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9087"
		       }
		     }`)

		var sharedDef types.EndpointDsl
		err := json.Unmarshal(sharedRestDsl, &sharedDef)
		assert.Nil(t, err)

		sharedCtx, err := pool.NewFromEndpoint(sharedDef)
		assert.NotNil(t, sharedCtx)
		assert.Nil(t, err)

		// 验证共享节点已创建
		_, ok := pool.Get("shared_rest_endpoint_ref")
		assert.True(t, ok)

		// 创建使用ref://引用的配置
		var refRestDsl = []byte(`
			{
		       "id": "ref_rest_endpoint",
		       "type": "endpoint/http",
		       "name": "引用REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": "ref://shared_rest_endpoint_ref"
		       }
		     }`)

		// 解析引用配置
		var refDef types.EndpointDsl
		err = json.Unmarshal(refRestDsl, &refDef)
		assert.Nil(t, err)
		assert.Equal(t, "ref://shared_rest_endpoint_ref", refDef.Configuration["server"])

		// 测试通过ref://获取共享实例
		serverConfig := refDef.Configuration["server"].(string)
		if strings.HasPrefix(serverConfig, "ref://") {
			instanceId := serverConfig[len("ref://"):]
			assert.Equal(t, "shared_rest_endpoint_ref", instanceId)

			// 从池中获取引用的实例
			sharedInstance, err := pool.GetInstance(instanceId)
			assert.Nil(t, err)
			assert.NotNil(t, sharedInstance)

			// 验证获取的是同一个共享实例
			sharedRest, ok := sharedInstance.(*rest.Rest)
			assert.True(t, ok)
			assert.Equal(t, ":9087", sharedRest.Config.Server)

		}

		// 验证ref://引用不会创建新的节点实例
		assert.Equal(t, 1, len(pool.GetAll())) // 只有一个共享节点
	})

	// 子测试2：测试共享节点重启不影响引用
	t.Run("SharedNodeRestartIsolation", func(t *testing.T) {
		// 获取原始共享实例的引用
		originalInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		originalRest, ok := originalInstance.(*rest.Rest)
		assert.True(t, ok)
		originalServer := originalRest.Config.Server

		// 模拟共享节点重启：删除并重新创建
		pool.Del("shared_rest_endpoint_ref")
		time.Sleep(1 * time.Second)
		// 验证共享节点已被删除
		_, ok = pool.Get("shared_rest_endpoint_ref")
		assert.False(t, ok)

		// 重新创建共享节点（模拟重启后的新配置）
		var restartedRestDsl = []byte(`
			{
		       "id": "shared_rest_endpoint_ref",
		       "type": "endpoint/http",
		       "name": "重启后的共享REST端点",
		       "debugMode": true,
		       "configuration": {
		         "server": ":9087",
		         "allowCors": true
		       }
		     }`)

		var restartedDef types.EndpointDsl
		err = json.Unmarshal(restartedRestDsl, &restartedDef)
		assert.Nil(t, err)

		_, err = pool.NewFromEndpoint(restartedDef)
		assert.Nil(t, err)

		// 验证重启后的实例配置已更新
		restartedInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		restartedRest, ok := restartedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.Equal(t, originalServer, restartedRest.Config.Server) // 服务器地址保持一致
		assert.True(t, restartedRest.Config.AllowCors)               // 新配置生效

		// 验证通过ref://仍然可以正常获取更新后的实例
		refInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		assert.Equal(t, restartedInstance, refInstance) // 引用获取的是同一个实例
	})

	// 子测试3：测试多个引用节点的独立性
	t.Run("MultipleReferencesIndependence", func(t *testing.T) {
		// 创建多个使用ref://的配置（模拟不同规则链中的引用）
		refConfigs := []string{
			`{"id": "ref1", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
			`{"id": "ref2", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
			`{"id": "ref3", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
		}

		// 验证所有引用都指向同一个共享实例
		sharedInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)

		for i, configStr := range refConfigs {
			var refDef types.EndpointDsl
			err := json.Unmarshal([]byte(configStr), &refDef)
			assert.Nil(t, err)

			serverConfig := refDef.Configuration["server"].(string)
			if strings.HasPrefix(serverConfig, "ref://") {
				instanceId := serverConfig[len("ref://"):]
				refInstance, err := pool.GetInstance(instanceId)
				assert.Nil(t, err)
				assert.Equal(t, sharedInstance, refInstance, "Reference %d should point to the same shared instance", i+1)
			}
		}

		// 验证节点池中仍然只有一个共享节点
		assert.Equal(t, 1, len(pool.GetAll()))
	})

	// 清理
	pool.Stop()
}

// TestRestSharedNodeDynamicRestart 测试REST endpoint的动态重启是否生效
func TestRestSharedNodeDynamicRestart(t *testing.T) {
	var restDsl = []byte(`
		{
	       "id": "dynamic_restart_test",
	       "type": "endpoint/http",
	       "name": "动态重启测试",
	       "debugMode": false,
	       "configuration": {
	         "server": ":9086",
	         "allowCors": false
	       }
	     }`)

	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 创建共享节点
	var def types.EndpointDsl
	err := json.Unmarshal(restDsl, &def)
	assert.Nil(t, err)

	ctx, err := pool.NewFromEndpoint(def)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	// 获取初始实例
	initialInstance, err := pool.GetInstance("dynamic_restart_test")
	assert.Nil(t, err)
	initialRest, ok := initialInstance.(*rest.Rest)
	assert.True(t, ok)
	assert.False(t, initialRest.Config.AllowCors) // 初始配置

	// 动态更新配置并重启
	var updatedRestDsl = []byte(`
		{
	       "id": "dynamic_restart_test",
	       "type": "endpoint/http",
	       "name": "动态重启测试-更新",
	       "debugMode": false,
	       "configuration": {
	         "server": ":9086",
	         "allowCors": true
	       }
	     }`)

	// 删除旧节点并重新创建
	pool.Del("dynamic_restart_test")
	time.Sleep(1 * time.Second)
	// 重新创建节点
	var updatedDef types.EndpointDsl
	err = json.Unmarshal(updatedRestDsl, &updatedDef)
	assert.Nil(t, err)
	_, err = pool.NewFromEndpoint(updatedDef)
	assert.Nil(t, err)

	// 获取更新后的实例
	updatedInstance, err := pool.GetInstance("dynamic_restart_test")
	assert.Nil(t, err)
	updatedRest, ok := updatedInstance.(*rest.Rest)
	assert.True(t, ok)

	// 验证配置已更新
	assert.True(t, updatedRest.Config.AllowCors) // 配置已更新

	// 验证配置更新成功

	// 添加一个简单的路由来测试功能
	router := impl.NewRouter().From("/cors-test").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("CORS enabled"))
		return true
	}).End()
	updatedRest.GET(router)

	// 验证路由已添加

	// 清理
	pool.Stop()
}
