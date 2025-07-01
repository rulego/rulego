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

// Package main demonstrates how to create an MQTT client
// that sends binary and JSON data to an MQTT endpoint server.
//
// This example shows:
// - Connecting to an MQTT broker
// - Publishing JSON data messages to specific topics
// - Publishing binary data messages to specific topics
// - Using different QoS levels
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rulego/rulego/utils/mqtt"
)

const (
	// MQTT broker configuration
	// MQTT代理配置
	mqttServer = "127.0.0.1:1883"
	clientID   = "rulego_mqtt_client_example"
)

// SensorData represents a sample JSON message structure
// SensorData 表示示例JSON消息结构
type SensorData struct {
	SensorID     string  `json:"sensorId"`
	Temperature  float64 `json:"temperature"`
	Humidity     float64 `json:"humidity"`
	Timestamp    int64   `json:"timestamp"`
	Location     string  `json:"location"`
	BatteryLevel float64 `json:"batteryLevel"`
}

// SystemMessage represents system-level messages
// SystemMessage 表示系统级消息
type SystemMessage struct {
	MessageID string `json:"messageId"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// DeviceCommand represents a binary command structure
// DeviceCommand 表示二进制命令结构
type DeviceCommand struct {
	DeviceID uint16 `json:"deviceId"`
	Command  uint8  `json:"command"`
	Value    uint32 `json:"value"`
}

func main() {
	fmt.Println("MQTT Client Example")
	fmt.Println("Connecting to MQTT broker at", mqttServer)

	// Create MQTT client configuration
	// 创建MQTT客户端配置
	config := mqtt.Config{
		Server:   mqttServer,
		Username: "",
		Password: "",
		QOS:      1,
		ClientID: clientID,
	}

	// Create MQTT client
	// 创建MQTT客户端
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mqtt.NewClient(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create MQTT client: %v", err)
	}
	defer client.Close()

	fmt.Println("Connected to MQTT broker successfully")

	// Send JSON sensor data
	// 发送JSON传感器数据
	sendJSONData(client)

	// Send binary device commands
	// 发送二进制设备命令
	sendBinaryData(client)

	// Send system messages
	// 发送系统消息
	sendSystemMessages(client)

	fmt.Println("Client finished sending data")
}

// sendJSONData sends sample JSON sensor data to MQTT topics
// sendJSONData 向MQTT主题发送示例JSON传感器数据
func sendJSONData(client *mqtt.Client) {
	fmt.Println("\n=== Sending JSON Sensor Data ===")

	// Sample sensor data
	// 示例传感器数据
	sensorData := []SensorData{
		{
			SensorID:     "TEMP_001",
			Temperature:  23.5,
			Humidity:     65.2,
			Timestamp:    time.Now().Unix(),
			Location:     "Office Room A",
			BatteryLevel: 85.6,
		},
		{
			SensorID:     "TEMP_002",
			Temperature:  35.8, // High temperature
			Humidity:     58.7,
			Timestamp:    time.Now().Unix(),
			Location:     "Warehouse B",
			BatteryLevel: 92.3,
		},
		{
			SensorID:     "HUM_001",
			Temperature:  22.1,
			Humidity:     75.5, // High humidity
			Timestamp:    time.Now().Unix(),
			Location:     "Laboratory C",
			BatteryLevel: 78.9,
		},
	}

	for i, data := range sensorData {
		// Convert to JSON
		// 转换为JSON
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Printf("Failed to marshal JSON data: %v", err)
			continue
		}

		// Publish to sensor-specific topic
		// 发布到传感器特定主题
		topic := fmt.Sprintf("sensors/%s/data", data.SensorID)
		fmt.Printf("Publishing JSON data %d to topic '%s': %s\n", i+1, topic, string(jsonData))

		err = client.Publish(topic, 1, jsonData)
		if err != nil {
			log.Printf("Failed to publish JSON data: %v", err)
			continue
		}

		// Wait a bit between messages
		// 消息之间等待一点时间
		time.Sleep(time.Second)
	}
}

// sendBinaryData sends sample binary device commands to MQTT topics
// sendBinaryData 向MQTT主题发送示例二进制设备命令
func sendBinaryData(client *mqtt.Client) {
	fmt.Println("\n=== Sending Binary Device Commands ===")

	// Sample binary commands
	// 示例二进制命令
	commands := []DeviceCommand{
		{DeviceID: 1001, Command: 0x01, Value: 100}, // SET_PARAMETER
		{DeviceID: 1002, Command: 0x02, Value: 255}, // GET_STATUS
		{DeviceID: 1003, Command: 0x03, Value: 0},   // RESET
		{DeviceID: 1004, Command: 0x04, Value: 50},  // SET_THRESHOLD
	}

	for i, cmd := range commands {
		// Create binary data (protocol: deviceId(2) + command(1) + value(4))
		// 创建二进制数据（协议：deviceId(2) + command(1) + value(4)）
		binaryData := make([]byte, 7)

		// Device ID (2 bytes, big endian)
		// 设备ID（2字节，大端序）
		binaryData[0] = byte(cmd.DeviceID >> 8)
		binaryData[1] = byte(cmd.DeviceID & 0xFF)

		// Command (1 byte)
		// 命令（1字节）
		binaryData[2] = cmd.Command

		// Value (4 bytes, big endian)
		// 值（4字节，大端序）
		binaryData[3] = byte(cmd.Value >> 24)
		binaryData[4] = byte(cmd.Value >> 16)
		binaryData[5] = byte(cmd.Value >> 8)
		binaryData[6] = byte(cmd.Value & 0xFF)

		// Publish to device-specific topic
		// 发布到设备特定主题
		topic := fmt.Sprintf("devices/%d/command", cmd.DeviceID)
		fmt.Printf("Publishing binary data %d to topic '%s': Device=%d, Command=0x%02X, Value=%d\n",
			i+1, topic, cmd.DeviceID, cmd.Command, cmd.Value)

		err := client.Publish(topic, 1, binaryData)
		if err != nil {
			log.Printf("Failed to publish binary data: %v", err)
			continue
		}

		// Wait a bit between messages
		// 消息之间等待一点时间
		time.Sleep(time.Second)
	}
}

// sendSystemMessages sends sample system messages
// sendSystemMessages 发送示例系统消息
func sendSystemMessages(client *mqtt.Client) {
	fmt.Println("\n=== Sending System Messages ===")

	// Sample system messages
	// 示例系统消息
	systemMessages := []SystemMessage{
		{
			MessageID: "SYS_001",
			Level:     "INFO",
			Source:    "DataProcessor",
			Content:   "System startup completed successfully",
			Timestamp: time.Now().Unix(),
		},
		{
			MessageID: "SYS_002",
			Level:     "WARNING",
			Source:    "TemperatureMonitor",
			Content:   "High temperature detected in zone B",
			Timestamp: time.Now().Unix(),
		},
		{
			MessageID: "SYS_003",
			Level:     "ERROR",
			Source:    "NetworkManager",
			Content:   "Connection timeout to external service",
			Timestamp: time.Now().Unix(),
		},
	}

	for i, msg := range systemMessages {
		// Convert to JSON
		// 转换为JSON
		jsonData, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Failed to marshal system message: %v", err)
			continue
		}

		// Publish to system topic based on level
		// 根据级别发布到系统主题
		topic := fmt.Sprintf("system/%s", msg.Level)
		fmt.Printf("Publishing system message %d to topic '%s': %s\n", i+1, topic, string(jsonData))

		err = client.Publish(topic, 1, jsonData)
		if err != nil {
			log.Printf("Failed to publish system message: %v", err)
			continue
		}

		// Wait a bit between messages
		// 消息之间等待一点时间
		time.Sleep(time.Second)
	}
}
