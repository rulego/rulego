/*
 * Copyright 2025 The RuleGo Authors.
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

package integration

import (
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	mqttEndpoint "github.com/rulego/rulego/endpoint/mqtt"
)

// TestMQTTEndpointShutdownSignal 测试 MQTT 端点的停机信号功能
func TestMQTTEndpointShutdownSignal(t *testing.T) {
	// 创建 MQTT 端点，但不连接到真实的 MQTT 服务器
	config := types.Configuration{
		"server":   "tcp://127.0.0.1:1883",
		"clientId": "test-signal-" + time.Now().Format("20060102150405"),
		"qos":      0,
	}

	mqttEp := &mqttEndpoint.Mqtt{}
	err := mqttEp.Init(rulego.NewConfig(), config)
	assert.Nil(t, err)

	// 测试停机信号在停机前不应该触发
	select {
	case <-mqttEp.GetShutdownSignal():
		t.Fatal("Should not receive shutdown signal before shutdown")
	default:
		// 正常情况
		t.Log("No shutdown signal before shutdown - as expected")
	}

	// 开始停机
	t.Log("Starting graceful shutdown...")
	go func() {
		time.Sleep(50 * time.Millisecond)
		t.Log("Calling GracefulStop()...")
		mqttEp.GracefulStop()
		t.Log("GracefulStop() completed")
	}()

	// 等待停机信号
	select {
	case <-mqttEp.GetShutdownSignal():
		t.Log("Received shutdown signal as expected")
	case <-time.After(5 * time.Second):
		t.Fatal("Should receive shutdown signal within 5 seconds")
	}

}

// TestMQTTEndpointBasicShutdown 测试 MQTT 端点的基本停机功能
func TestMQTTEndpointBasicShutdown(t *testing.T) {
	// 创建 MQTT 端点
	config := types.Configuration{
		"server":   "tcp://127.0.0.1:1883",
		"clientId": "test-basic-" + time.Now().Format("20060102150405"),
		"qos":      0,
	}

	mqttEp := &mqttEndpoint.Mqtt{}
	err := mqttEp.Init(rulego.NewConfig(), config)
	assert.Nil(t, err)

	// 测试停机状态
	assert.False(t, mqttEp.IsShuttingDown(), "Should not be shutting down initially")

	// 开始停机
	mqttEp.GracefulStop()

	// 检查停机状态
	assert.True(t, mqttEp.IsShuttingDown(), "Should be shutting down after GracefulStop")

}

// TestGracefulShutdownBasic 测试基本的 GracefulShutdown 功能
func TestGracefulShutdownBasic(t *testing.T) {
	// 创建一个基本的 GracefulShutdown 实例
	var gs base.GracefulShutdown
	gs.InitGracefulShutdown(nil, 5*time.Second)

	// 测试停机信号在停机前不应该触发
	select {
	case <-gs.GetShutdownSignal():
		t.Fatal("Should not receive shutdown signal before shutdown")
	default:
	}

	// 测试停机状态
	assert.False(t, gs.IsShuttingDown(), "Should not be shutting down initially")

	// 开始停机
	t.Log("Starting graceful shutdown...")
	go func() {
		time.Sleep(50 * time.Millisecond)
		gs.GracefulStop(func() {
			t.Log("Stop function called")
		})
	}()

	// 等待停机信号
	select {
	case <-gs.GetShutdownSignal():
		t.Log("Received shutdown signal as expected")
	case <-time.After(2 * time.Second):
		t.Fatal("Should receive shutdown signal within 2 seconds")
	}

	// 检查停机状态
	assert.True(t, gs.IsShuttingDown(), "Should be shutting down after GracefulStop")

}
