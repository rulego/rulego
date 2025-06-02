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

package types

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

// DataType defines the type of data contained in a message.
type DataType string

// Constants for different data types that a message can represent.
const (
	JSON   = DataType("JSON")   // Represents data in JSON format.
	TEXT   = DataType("TEXT")   // Represents plain text data.
	BINARY = DataType("BINARY") // Represents binary data.
)

// Constants for keys used in message handling.
const (
	IdKey       = "id"       // Key for the message id.
	TsKey       = "ts"       // Key for the message ts.
	DataKey     = "data"     // Key for the message content.
	MsgKey      = "msg"      // Key for the message content object.
	MetadataKey = "metadata" // Key for the message metadata.
	MsgTypeKey  = "msgType"  // Key for the message type.
	TypeKey     = "type"     // Key for the message type.
	DataTypeKey = "dataType" // Key for the data type of the message.
)

// Properties is a simple map type for storing key-value pairs as metadata.
// It provides basic operations for metadata management without Copy-on-Write optimization.
// This type is suitable for scenarios where performance is not critical or when
// metadata sharing between multiple instances is not required.
type Properties map[string]string

// NewProperties creates a new empty Properties instance.
// Returns an initialized Properties map ready for use.
func NewProperties() Properties {
	return make(Properties)
}

// BuildProperties creates a new Properties instance from existing data.
// If the input data is nil, returns an empty Properties instance.
// The function creates a deep copy of the input data to ensure isolation.
func BuildProperties(data Properties) Properties {
	if data == nil {
		return make(Properties)
	}
	// Pre-allocate with known capacity to reduce map resizing
	metadata := make(Properties, len(data))
	for k, v := range data {
		metadata[k] = v
	}
	return metadata
}

// Copy creates a deep copy of the Properties.
// This ensures that modifications to the copy do not affect the original.
func (md Properties) Copy() Properties {
	return BuildProperties(md)
}

// Has checks if a key exists in the metadata.
// Returns true if the key exists, false otherwise.
func (md Properties) Has(key string) bool {
	_, ok := md[key]
	return ok
}

// GetValue retrieves a value by key from the metadata.
// Returns the value if the key exists, or an empty string if not found.
func (md Properties) GetValue(key string) string {
	v, _ := md[key]
	return v
}

// PutValue sets a value in the metadata.
// If the key is empty, the operation is ignored to prevent invalid entries.
func (md Properties) PutValue(key, value string) {
	if key != "" {
		md[key] = value
	}
}

// Values returns the underlying map containing all key-value pairs.
// Note: This returns a direct reference to the internal map, so modifications
// will affect the original Properties instance.
func (md Properties) Values() map[string]string {
	return md
}

// Metadata is a type for message metadata within the rule engine.
// It uses Copy-on-Write mechanism to optimize performance in multi-node scenarios.
type Metadata struct {
	// data holds the actual metadata key-value pairs
	data map[string]string
	// shared indicates if this metadata is shared with other instances
	shared bool
	// mu protects the shared flag and data during copy operations
	mu sync.RWMutex
}

// NewMetadata creates a new instance of rule engine message metadata.
func NewMetadata() *Metadata {
	return &Metadata{
		data:   make(map[string]string),
		shared: false,
	}
}

// BuildMetadata creates a new instance of rule engine message metadata from a map.
func BuildMetadata(data map[string]string) *Metadata {
	if data == nil {
		return NewMetadata()
	}
	// Pre-allocate with known capacity to reduce map resizing
	metadata := make(map[string]string, len(data))
	for k, v := range data {
		metadata[k] = v
	}
	return &Metadata{
		data:   metadata,
		shared: false,
	}
}

// BuildMetadataFromMetadata creates a new instance from existing Metadata (for backward compatibility).
func BuildMetadataFromMetadata(md *Metadata) *Metadata {
	if md == nil || md.data == nil {
		return NewMetadata()
	}
	return BuildMetadata(md.data)
}

// Copy creates a copy of the metadata using Copy-on-Write optimization.
func (md *Metadata) Copy() *Metadata {
	md.mu.Lock()
	defer md.mu.Unlock()

	// Mark current instance as shared
	md.shared = true

	// Return a new instance that shares the same data initially
	// Note: The new instance gets its own mutex (zero value is ready to use)
	return &Metadata{
		data:   md.data,
		shared: true,
		// mu is automatically initialized as zero value (ready to use)
	}
}

// ensureUnique ensures the metadata has its own copy of data before modification.
func (md *Metadata) ensureUnique() {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.shared {
		// Create a new copy of the data
		newData := make(map[string]string, len(md.data))
		for k, v := range md.data {
			newData[k] = v
		}
		md.data = newData
		md.shared = false
	}
}

// MarshalJSON implements the json.Marshaler interface for Metadata
func (md *Metadata) MarshalJSON() ([]byte, error) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return json.Marshal(md.data)
}

// UnmarshalJSON implements the json.Unmarshaler interface for Metadata
func (md *Metadata) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	md.mu.Lock()
	defer md.mu.Unlock()

	// Ensure unique copy before replacing all data
	if md.shared {
		// We need to create a new data map since we're shared
		md.shared = false
	}

	md.data = m
	return nil
}

// Has checks if a key exists in the metadata.
func (md *Metadata) Has(key string) bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	_, ok := md.data[key]
	return ok
}

// GetValue retrieves a value by key from the metadata.
func (md *Metadata) GetValue(key string) string {
	md.mu.RLock()
	defer md.mu.RUnlock()
	v, _ := md.data[key]
	return v
}

// PutValue sets a value in the metadata.
func (md *Metadata) PutValue(key, value string) {
	if key == "" {
		return
	}

	md.mu.Lock()
	defer md.mu.Unlock()

	// Ensure unique copy within the same lock
	if md.shared {
		// Create a new copy of the data
		newData := make(map[string]string, len(md.data))
		for k, v := range md.data {
			newData[k] = v
		}
		md.data = newData
		md.shared = false
	}

	md.data[key] = value
}

// Values returns all key-value pairs in the metadata.
func (md *Metadata) Values() map[string]string {
	md.mu.RLock()
	defer md.mu.RUnlock()
	// Return a copy to prevent external modification
	result := make(map[string]string, len(md.data))
	for k, v := range md.data {
		result[k] = v
	}
	return result
}

// ReplaceAll replaces all metadata with new data.
func (md *Metadata) ReplaceAll(newData map[string]string) {
	md.mu.Lock()
	defer md.mu.Unlock()

	// Ensure unique copy before replacing all data
	if md.shared {
		// We need to create a new data map since we're shared
		md.shared = false
	}

	// Create new data map
	md.data = make(map[string]string, len(newData))
	for k, v := range newData {
		md.data[k] = v
	}
}

// Clear clears all metadata.
func (md *Metadata) Clear() {
	md.mu.Lock()
	defer md.mu.Unlock()

	// Ensure unique copy before clearing all data
	if md.shared {
		// We need to create a new data map since we're shared
		md.shared = false
	}

	// Create new empty data map
	md.data = make(map[string]string)
}

// Len returns the number of key-value pairs in the metadata.
func (md *Metadata) Len() int {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return len(md.data)
}

// RuleMsg represents a message within the rule engine system.
// It encapsulates all the information needed for message processing, including
// the message content, metadata, type information, and timing details.
// RuleMsg is the core data structure that flows through the rule engine nodes.
type RuleMsg struct {
	// Ts is the message timestamp in milliseconds since Unix epoch.
	// This field is automatically set when creating a new message if not provided.
	Ts int64 `json:"ts"`

	// Id is the unique identifier for the message as it flows through the rule engine.
	// Each message gets a UUID when created, ensuring uniqueness across the system.
	Id string `json:"id"`

	// DataType specifies the format of the data contained in the message.
	// Supported types include JSON, TEXT, and BINARY.
	DataType DataType `json:"dataType"`

	// Type is a crucial field for the rule engine to distribute and categorize messages.
	// This field determines how the rule engine routes and processes the message.
	// Common examples include: POST_TELEMETRY, ACTIVITY_EVENT, INACTIVITY_EVENT,
	// CONNECT_EVENT, DISCONNECT_EVENT, ENTITY_CREATED, ENTITY_UPDATED, ENTITY_DELETED,
	// DEVICE_ALARM, POST_DEVICE_DATA.
	Type string `json:"type"`

	// Data contains the actual message payload as a string.
	// The format of this data should match the DataType field.
	Data string `json:"data"`

	// Metadata contains additional key-value pairs associated with the message.
	// This field uses Copy-on-Write optimization for better performance in multi-node scenarios.
	Metadata *Metadata `json:"metadata"`
}

// NewMsg creates a new message instance and generates a message ID using UUID.
func NewMsg(ts int64, msgType string, dataType DataType, metaData *Metadata, data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), ts, msgType, dataType, metaData, data)
}

func NewMsgWithJsonData(data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), 0, "", JSON, NewMetadata(), data)
}

// newMsg is a helper function to create a new RuleMsg.
func newMsg(id string, ts int64, msgType string, dataType DataType, metaData *Metadata, data string) RuleMsg {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	if id == "" {
		uuId, _ := uuid.NewV4()
		id = uuId.String()
	}
	var metadata *Metadata
	if metaData != nil {
		metadata = metaData
	} else {
		metadata = NewMetadata()
	}
	return RuleMsg{
		Ts:       ts,
		Id:       id,
		Type:     msgType,
		DataType: dataType,
		Data:     data,
		Metadata: metadata,
	}
}

// Copy creates a deep copy of the message.
// This method ensures that the copied message is completely independent
// from the original, including its metadata.
func (m *RuleMsg) Copy() RuleMsg {
	var copiedMetadata *Metadata
	if m.Metadata != nil {
		copiedMetadata = m.Metadata.Copy()
	} else {
		copiedMetadata = NewMetadata()
	}
	return newMsg(m.Id, m.Ts, m.Type, m.DataType, copiedMetadata, m.Data)
}

// GetTs returns the timestamp of the message.
func (m *RuleMsg) GetTs() int64 {
	return m.Ts
}

// SetTs sets the timestamp of the message.
func (m *RuleMsg) SetTs(ts int64) {
	m.Ts = ts
}

// GetId returns the unique identifier of the message.
func (m *RuleMsg) GetId() string {
	return m.Id
}

// SetId sets the unique identifier of the message.
func (m *RuleMsg) SetId(id string) {
	m.Id = id
}

// GetDataType returns the data type of the message.
func (m *RuleMsg) GetDataType() DataType {
	return m.DataType
}

// SetDataType sets the data type of the message.
func (m *RuleMsg) SetDataType(dataType DataType) {
	m.DataType = dataType
}

// GetType returns the message type used for routing and categorization.
func (m *RuleMsg) GetType() string {
	return m.Type
}

// SetType sets the message type used for routing and categorization.
func (m *RuleMsg) SetType(msgType string) {
	m.Type = msgType
}

// GetData returns the message payload data.
func (m *RuleMsg) GetData() string {
	return m.Data
}

// SetData sets the message payload data.
func (m *RuleMsg) SetData(data string) {
	m.Data = data
}

// GetMetadata returns the metadata associated with the message.
// Returns nil if no metadata is set.
func (m *RuleMsg) GetMetadata() *Metadata {
	return m.Metadata
}

// SetMetadata sets the metadata for the message.
// If metadata is nil, a new empty Metadata instance will be created.
func (m *RuleMsg) SetMetadata(metadata *Metadata) {
	if metadata == nil {
		m.Metadata = NewMetadata()
	} else {
		m.Metadata = metadata
	}
}

// WrapperMsg is a container type for wrapping the results of node execution.
// It encapsulates the processed message along with execution context information,
// making it useful for tracking message flow and handling errors in the rule engine.
type WrapperMsg struct {
	// Msg contains the processed message after node execution.
	Msg RuleMsg `json:"msg"`

	// Err contains any error message that occurred during node execution.
	// Empty string indicates successful execution.
	Err string `json:"err"`

	// NodeId identifies the node that produced this result.
	// This is useful for debugging and tracking message flow through the rule chain.
	NodeId string `json:"nodeId"`
}
