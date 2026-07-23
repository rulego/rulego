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

// Package main demonstrates how to create a NET client
// that sends binary data to a NET endpoint server.
//
// This example shows:
// - Connecting to a NET endpoint server
// - Sending binary data messages
// - Receiving and parsing binary server responses in hex format
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	// Server address to connect to
	// The server address to connect to
	serverAddr = "localhost:8088"
)

// DeviceCommand represents a binary command structure
// DeviceCommand represents the binary command structure
type DeviceCommand struct {
	DeviceID uint16 `json:"deviceId"`
	Command  uint8  `json:"command"`
	Value    uint32 `json:"value"`
}

func main() {
	fmt.Println("NET Client Example")
	fmt.Println("Connecting to server at", serverAddr)

	// Connect to the server
	// Connect to the server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to server successfully")

	// Create a reader for server responses
	// Create a reader for server responses
	reader := bufio.NewReader(conn)

	// Send binary data examples
	// Send binary data examples
	sendBinaryData(conn, reader)

	fmt.Println("Client finished sending data")
}

// sendBinaryData sends sample binary messages to the server
// sendBinaryData sends a sample binary message to the server
func sendBinaryData(conn net.Conn, reader *bufio.Reader) {
	fmt.Println("\n=== Sending Binary Data ===")

	// Sample binary commands
	// Example binary command
	commands := []DeviceCommand{
		{DeviceID: 1001, Command: 0x01, Value: 100},
		{DeviceID: 1002, Command: 0x02, Value: 255},
		{DeviceID: 1003, Command: 0x03, Value: 0},
	}

	for i, cmd := range commands {
		// Create binary data (simple protocol: deviceId(2) + command(1) + value(4))
		// Create binary data (simple protocol: deviceId(2) + command(1) + value(4))
		binaryData := make([]byte, 7)

		// Device ID (2 bytes, big endian)
		// Device ID (2 bytes, large-ending)
		binaryData[0] = byte(cmd.DeviceID >> 8)
		binaryData[1] = byte(cmd.DeviceID & 0xFF)

		// Command (1 byte)
		// Command (1 byte)
		binaryData[2] = cmd.Command

		// Value (4 bytes, big endian)
		// Value (4 bytes, large-endix order)
		binaryData[3] = byte(cmd.Value >> 24)
		binaryData[4] = byte(cmd.Value >> 16)
		binaryData[5] = byte(cmd.Value >> 8)
		binaryData[6] = byte(cmd.Value & 0xFF)

		// Send binary data
		// Send binary data
		fmt.Printf("Sending binary data %d: Device=%d, Command=0x%02X, Value=%d (hex: %02X %02X %02X %02X %02X %02X %02X)\n",
			i+1, cmd.DeviceID, cmd.Command, cmd.Value,
			binaryData[0], binaryData[1], binaryData[2], binaryData[3], binaryData[4], binaryData[5], binaryData[6])

		startTime := time.Now()
		_, err := conn.Write(append(binaryData, '\n'))
		if err != nil {
			log.Printf("Failed to send binary data: %v", err)
			continue
		}

		// Read server response
		// Read the server's response
		err = readAndDisplayResponse(reader, "Binary", startTime)
		if err != nil {
			log.Printf("Failed to read response: %v", err)
			continue
		}

		// Wait a bit between messages
		// Wait a little between messages
		time.Sleep(time.Second)
	}
}

// readAndDisplayResponse reads server response and displays it with enhanced formatting
// readAndDisplayResponse reads the server response and displays it in enhanced format
func readAndDisplayResponse(reader *bufio.Reader, dataType string, startTime time.Time) error {
	// For binary data, read raw bytes until newline
	// For binary data, the original byte is read up to the line new
	var responseBytes []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("failed to read response: %v", err)
		}
		responseBytes = append(responseBytes, b)
		if b == '\n' {
			break
		}
	}

	// Calculate response time
	// Calculate response time
	responseTime := time.Since(startTime)
	fmt.Printf("Response Time: %v\n", responseTime)

	// Print binary response in hex format
	// Print binary responses in hex format
	fmt.Printf("=== Binary Response ===\n")
	fmt.Printf("Length: %d bytes\n", len(responseBytes))
	fmt.Printf("Hex: ")
	for i, b := range responseBytes {
		if i > 0 {
			fmt.Printf(" ")
		}
		fmt.Printf("%02X", b)
	}
	fmt.Printf("\n")

	// Print as ASCII (for debugging)
	// Printed as ASCII (for debugging)
	fmt.Printf("ASCII: ")
	for _, b := range responseBytes {
		if b >= 32 && b <= 126 {
			fmt.Printf("%c", b)
		} else if b == '\n' {
			fmt.Printf("\\n")
		} else {
			fmt.Printf(".")
		}
	}
	fmt.Printf("\n")

	// Decode binary response (without newline)
	// Decoding binary responses (excluding line breaks)
	responseData := responseBytes[:len(responseBytes)-1] // Remove newline
	if len(responseData) >= 4 {
		status := responseData[0]
		deviceIdHigh := responseData[1]
		deviceIdLow := responseData[2]
		command := responseData[3]
		deviceId := (uint16(deviceIdHigh) << 8) | uint16(deviceIdLow)

		fmt.Printf("Decoded:\n")
		fmt.Printf("  Status: 0x%02X (%s)\n", status, func() string {
			if status == 0x01 {
				return "Success"
			} else {
				return "Error"
			}
		}())
		fmt.Printf("  Device ID: %d (0x%04X)\n", deviceId, deviceId)
		fmt.Printf("  Command Echo: 0x%02X\n", command)
	} else if len(responseData) >= 2 {
		status := responseData[0]
		errorCode := responseData[1]
		fmt.Printf("Decoded Error:\n")
		fmt.Printf("  Status: 0x%02X (Error)\n", status)
		fmt.Printf("  Error Code: 0x%02X\n", errorCode)
	}

	fmt.Println("========================")
	return nil
}
