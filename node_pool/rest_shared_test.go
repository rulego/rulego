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

// TestRestSharedNodeBasicOperations Tests the basic SharedNode functionality and multi-instance sharing of REST endpoints
func TestRestSharedNodeBasicOperations(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Subtest 1: Basic SharedNode functionality
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

		// Create a shared node
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// Verify that the shared node has been created
		sharedCtx, ok := pool.Get("shared_rest_endpoint")
		assert.True(t, ok)
		assert.NotNil(t, sharedCtx)

		// Get the REST instance
		restInstance, err := pool.GetInstance("shared_rest_endpoint")
		assert.Nil(t, err)
		assert.NotNil(t, restInstance)

		// Verify instance types and configurations
		restEndpoint, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.NotNil(t, restEndpoint)
		assert.Equal(t, ":9080", restEndpoint.Config.Server)

		// Cleanup
		pool.Del("shared_rest_endpoint")
		assert.Equal(t, 0, len(pool.GetAll()))
	})

	// Subtest 2: Multi-instance shared validation
	t.Run("MultipleInstancesSharing", func(t *testing.T) {
		// Create a shared REST server node
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

		// Verify that only one shared server node exists
		assert.Equal(t, 1, len(pool.GetAll()))

		// Obtain the shared server instance
		sharedInstance, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		sharedRest, ok := sharedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.Equal(t, ":9081", sharedRest.Config.Server)

		// Verification returns the same instance multiple times
		instance1, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		instance2, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		assert.Equal(t, instance1, instance2) // It should be the same example

		// Verify that a nonexistent node returns an error
		_, err = pool.GetInstance("non_existent_node")
		assert.NotNil(t, err)

		// Cleanup
		pool.Stop()
		assert.Equal(t, 0, len(pool.GetAll()))
	})
}

// TestRestSharedNodeLifecycleManagement Tests lifecycle management of REST endpoints (restart and logout)
func TestRestSharedNodeLifecycleManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Subtest 1: Restart the function test
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

		// Create a shared node
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// Get the REST instance
		restInstance, err := pool.GetInstance("restart_test_rest")
		assert.Nil(t, err)
		_, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)

		// Test the restart function: delete the old node and create a new one
		pool.Del("restart_test_rest")
		time.Sleep(1 * time.Second)
		// Create updated configurations
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

		// Recreate the node
		var newDef types.EndpointDsl
		err = json.Unmarshal(newRestDsl, &newDef)
		assert.Nil(t, err)
		newCtx, err := pool.NewFromEndpoint(newDef)
		assert.NotNil(t, newCtx)
		assert.Nil(t, err)

		// Verification configuration has been updated
		updatedInstance, err := pool.GetInstance("restart_test_rest")
		assert.Nil(t, err)
		updatedRest, ok := updatedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.True(t, updatedRest.Config.AllowCors)

		// Cleanup
		pool.Del("restart_test_rest")
	})

	// Subtest 2: Drop off impact test
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

		// Create a shared node
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// Verification nodes exist
		_, ok := pool.Get("unregister_test_rest")
		assert.True(t, ok)
		assert.Equal(t, 1, len(pool.GetAll()))

		// Get the instance
		instance, err := pool.GetInstance("unregister_test_rest")
		assert.Nil(t, err)
		assert.NotNil(t, instance)

		// Deregister nodes
		pool.Del("unregister_test_rest")

		// The verification node has been removed
		_, ok = pool.Get("unregister_test_rest")
		assert.False(t, ok)
		assert.Equal(t, 0, len(pool.GetAll()))

		// Attempt to retrieve deleted instances
		instance, err = pool.GetInstance("unregister_test_rest")
		assert.NotNil(t, err)
		assert.Nil(t, instance)
	})

	// Eventually, the cleanup was done
	pool.Stop()
}

// TestRestSharedNodeAdvancedFeatures Tests advanced features of REST endpoints (routing and concurrency)
func TestRestSharedNodeAdvancedFeatures(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Subtest 1: Routing function test
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

		// Create a shared node
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// Get the REST instance
		restInstance, err := pool.GetInstance("routes_test_rest")
		assert.Nil(t, err)
		restEndpoint, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)

		// Add routes
		router := impl.NewRouter().From("/test").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			exchange.Out.SetBody([]byte("Hello from shared REST endpoint"))
			return true
		}).End()

		restEndpoint.GET(router)

		// Cleanup
		pool.Del("routes_test_rest")
	})

	// Subtest 2: Concurrent Access Test
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

		// Create a shared node
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// Concurrent instance acquisition
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

		// Collect the results
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

		// Verify that all instances are the same (shared instance)
		assert.Equal(t, numGoroutines, len(instances))
		for i := 1; i < len(instances); i++ {
			assert.Equal(t, instances[0], instances[i])
		}

		// Cleanup
		pool.Del("concurrent_test_rest")
	})

	// Eventually, the cleanup was done
	pool.Stop()
}

// TestRestSharedNodeWithRefProtocol tests introduce shared REST endpoints and their lifecycle management using ref:// method
func TestRestSharedNodeWithRefProtocol(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Subtest 1: Basic ref:// Reference Function
	t.Run("BasicRefProtocol", func(t *testing.T) {
		// Create a shared node
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

		// Verify that the shared node has been created
		_, ok := pool.Get("shared_rest_endpoint_ref")
		assert.True(t, ok)

		// Create configurations that use ref:// references
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

		// Parse reference configuration
		var refDef types.EndpointDsl
		err = json.Unmarshal(refRestDsl, &refDef)
		assert.Nil(t, err)
		assert.Equal(t, "ref://shared_rest_endpoint_ref", refDef.Configuration["server"])

		// Testing ref:// to obtain shared instances
		serverConfig := refDef.Configuration["server"].(string)
		if strings.HasPrefix(serverConfig, "ref://") {
			instanceId := serverConfig[len("ref://"):]
			assert.Equal(t, "shared_rest_endpoint_ref", instanceId)

			// Retrieve the instance of references from the pool
			sharedInstance, err := pool.GetInstance(instanceId)
			assert.Nil(t, err)
			assert.NotNil(t, sharedInstance)

			// Verify that the same shared instance is obtained
			sharedRest, ok := sharedInstance.(*rest.Rest)
			assert.True(t, ok)
			assert.Equal(t, ":9087", sharedRest.Config.Server)

		}

		// Verification ref:// reference does not create a new node instance
		assert.Equal(t, 1, len(pool.GetAll())) // There is only one shared node
	})

	// Subtest 2: Test that restarting the shared node does not affect references
	t.Run("SharedNodeRestartIsolation", func(t *testing.T) {
		// Retrieves a reference to the original shared instance
		originalInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		originalRest, ok := originalInstance.(*rest.Rest)
		assert.True(t, ok)
		originalServer := originalRest.Config.Server

		// Simulated shared node restart: Delete and recreate
		pool.Del("shared_rest_endpoint_ref")
		time.Sleep(1 * time.Second)
		// Verify that the shared node has been removed
		_, ok = pool.Get("shared_rest_endpoint_ref")
		assert.False(t, ok)

		// Recreate the shared node (simulate the new configuration after a restart)
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

		// Verify that the rebooted instance configuration has been updated
		restartedInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		restartedRest, ok := restartedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.Equal(t, originalServer, restartedRest.Config.Server) // The server address remains consistent
		assert.True(t, restartedRest.Config.AllowCors)               // The new configuration takes effect

		// ref:// verification passes, the updated instance can still be obtained normally
		refInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		assert.Equal(t, restartedInstance, refInstance) // The reference retrieves the same instance
	})

	// Subtest 3: Testing the independence of multiple reference nodes
	t.Run("MultipleReferencesIndependence", func(t *testing.T) {
		// Create multiple configurations using ref:// (simulating references in different rule chains)
		refConfigs := []string{
			`{"id": "ref1", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
			`{"id": "ref2", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
			`{"id": "ref3", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
		}

		// Verify that all references point to the same shared instance
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

		// There is still only one shared node in the validator node pool
		assert.Equal(t, 1, len(pool.GetAll()))
	})

	// Cleanup
	pool.Stop()
}

// TestRestSharedNodeDynamicRestart tests whether the dynamic restart of the REST endpoint is effective
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
	config.NodePool = pool

	// Create a shared node
	var def types.EndpointDsl
	err := json.Unmarshal(restDsl, &def)
	assert.Nil(t, err)

	ctx, err := pool.NewFromEndpoint(def)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	// Obtain the initial instance
	initialInstance, err := pool.GetInstance("dynamic_restart_test")
	assert.Nil(t, err)
	initialRest, ok := initialInstance.(*rest.Rest)
	assert.True(t, ok)
	assert.False(t, initialRest.Config.AllowCors) // Initial configuration

	// Dynamically update configurations and restart
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

	// Delete the old node and recreate it
	pool.Del("dynamic_restart_test")
	time.Sleep(1 * time.Second)
	// Recreate the node
	var updatedDef types.EndpointDsl
	err = json.Unmarshal(updatedRestDsl, &updatedDef)
	assert.Nil(t, err)
	_, err = pool.NewFromEndpoint(updatedDef)
	assert.Nil(t, err)

	// Get the updated instance
	updatedInstance, err := pool.GetInstance("dynamic_restart_test")
	assert.Nil(t, err)
	updatedRest, ok := updatedInstance.(*rest.Rest)
	assert.True(t, ok)

	// Verification configuration has been updated
	assert.True(t, updatedRest.Config.AllowCors) // The configuration has been updated

	// Verify that the configuration update was successful

	// Add a simple route to test the functionality
	router := impl.NewRouter().From("/cors-test").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("CORS enabled"))
		return true
	}).End()
	updatedRest.GET(router)

	// Verification routes have been added

	// Cleanup
	pool.Stop()
}
