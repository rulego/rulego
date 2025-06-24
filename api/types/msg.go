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
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
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

	// Data contains the actual message payload using Copy-on-Write optimization.
	// The format of this data should match the DataType field.
	Data *SharedData `json:"data"`

	// Metadata contains additional key-value pairs associated with the message.
	// This field uses Copy-on-Write optimization for better performance in multi-node scenarios.
	Metadata *Metadata `json:"metadata"`
}

// NewMsg creates a new message instance and generates a message ID using UUID.
func NewMsg(ts int64, msgType string, dataType DataType, metaData *Metadata, data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), ts, msgType, dataType, metaData, str.UnsafeBytesFromString(data))
}

// NewMsgFromBytes creates a new message instance from []byte data and generates a message ID using UUID.
func NewMsgFromBytes(ts int64, msgType string, dataType DataType, metaData *Metadata, data []byte) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), ts, msgType, dataType, metaData, data)
}

func NewMsgWithJsonData(data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), 0, "", JSON, NewMetadata(), str.UnsafeBytesFromString(data))
}

// NewMsgWithJsonDataFromBytes creates a new message instance with JSON data from []byte.
func NewMsgWithJsonDataFromBytes(data []byte) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), 0, "", JSON, NewMetadata(), data)
}

// newMsg is a helper function to create a new RuleMsg from []byte data.
func newMsg(id string, ts int64, msgType string, dataType DataType, metaData *Metadata, data []byte) RuleMsg {
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

	// Create the message
	msg := RuleMsg{
		Ts:       ts,
		Id:       id,
		Type:     msgType,
		DataType: dataType,
		Data:     NewSharedDataFromBytes(data),
		Metadata: metadata,
	}

	// Note: We cannot set the callback here because when the function returns,
	// it returns a copy of the message, and the callback would point to the wrong instance.
	// The callback will be set in SetData method when needed.

	return msg
}

// SetData sets the message data from string with zero-copy optimization.
// This method uses zero-copy conversion for optimal performance while maintaining
// safety through Copy-on-Write mechanism.
//
// Performance: Zero-copy, optimal performance
// Safety: Protected by Copy-on-Write mechanism
func (m *RuleMsg) SetData(data string) {
	if m.Data == nil {
		m.Data = NewSharedData(data)
	} else {
		m.Data.SetUnsafe(data)
	}
}

// SetBytes sets the message data from []byte.
// The input []byte will be copied to ensure data isolation.
func (m *RuleMsg) SetBytes(data []byte) {
	if m.Data == nil {
		m.Data = NewSharedDataFromBytes(data)
	} else {
		m.Data.SetBytes(data)
	}
}

// GetData returns the message data as string using zero-copy conversion.
// This method is optimized for performance while maintaining safety through Copy-on-Write mechanism.
//
// Performance: Zero-copy, optimal performance
// Safety: Protected by Copy-on-Write mechanism in SharedData
func (m *RuleMsg) GetData() string {
	if m.Data == nil {
		return ""
	}
	return m.Data.GetUnsafe()
}

// GetBytes returns the message data as []byte.
//
// IMPORTANT: The returned []byte slice shares memory with the internal data and
// MUST NOT be modified. Any modification will corrupt the shared data and may
// cause data races. If you need to modify the data, use GetMutableBytes() instead.
//
// For string data, use GetData() which is safer and more efficient.
func (m *RuleMsg) GetBytes() []byte {
	if m.Data == nil {
		return nil
	}
	return m.Data.GetBytes()
}

// GetSharedData returns the underlying SharedData instance.
// This provides direct access to the SharedData for advanced operations.
//
// Use cases:
// - Direct manipulation of SharedData methods (GetMutableBytes, GetRefCount, etc.)
// - Advanced copy-on-write operations
// - Performance-critical scenarios requiring fine-grained control
//
// WARNING: Direct modification of the returned SharedData affects this message instance.
// Consider using Copy() if you need to modify data without affecting the original.
//
// Example usage:
//
//	sharedData := msg.GetSharedData()
//	mutableBytes := sharedData.GetMutableBytes()
//	// modify mutableBytes...
//	sharedData.SetBytes(mutableBytes)
func (m *RuleMsg) GetSharedData() *SharedData {
	if m.Data == nil {
		// Return a new empty SharedData to avoid nil pointer issues
		m.Data = NewSharedData("")
	}
	return m.Data
}

// SetSharedData sets the underlying SharedData instance.
// This allows for advanced SharedData management and zero-copy message construction.
//
// Use cases:
// - Sharing data between multiple messages without copying
// - Advanced memory management scenarios
// - Performance-critical message construction
//
// The method automatically sets up the data change callback to clear JSON cache.
//
// Example usage:
//
//	sharedData := someOtherMsg.GetSharedData().Copy()
//	msg.SetSharedData(sharedData)
func (m *RuleMsg) SetSharedData(data *SharedData) {
	if data == nil {
		m.Data = NewSharedData("")
	} else {
		m.Data = data
	}
}

// GetJsonData returns the message data parsed as JSON with caching.
// If the data has already been parsed, returns cached result.
// If the data is not valid JSON, it returns an error.
// Returns map[string]interface{} for JSON objects or []interface{} for JSON arrays.
// This method now delegates to SharedData for better concurrency control and unified caching.
func (m *RuleMsg) GetJsonData() (interface{}, error) {
	if m.Data == nil {
		return make(map[string]interface{}), nil
	}
	return m.Data.GetJsonData()
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

	// Create a copy with shared data for COW optimization
	var copiedData *SharedData
	if m.Data != nil {
		copiedData = m.Data.Copy()
	} else {
		copiedData = NewSharedData("")
	}

	copiedMsg := RuleMsg{
		Ts:       m.Ts,
		Id:       m.Id,
		Type:     m.Type,
		DataType: m.DataType,
		Data:     copiedData,
		Metadata: copiedMetadata,
	}

	// Note: Cannot set callback here for the same reason as in newMsg.
	// The callback will be set when SetData is called on the copied message.

	return copiedMsg
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

// SharedData represents a thread-safe copy-on-write data structure for message payload.
// This improved version addresses potential race conditions in the original implementation.
type SharedData struct {
	// data holds the actual payload data
	data []byte
	// refCount tracks how many instances share this data (using atomic operations)
	// This pointer is shared among all instances that share the same data
	refCount *int64
	// mu protects the data during copy and modification operations
	mu sync.RWMutex
	// onDataChanged callback to notify when data changes
	onDataChanged func()
	// parsedData caches the parsed JSON data to avoid repeated unmarshaling
	// Can be map[string]interface{} for JSON objects or []interface{} for JSON arrays
	parsedData interface{}
	// dataVersion tracks the version of the data to prevent caching stale parsed results
	// This version is incremented every time the data is modified
	dataVersion int64
}

// NewSharedData creates a new SharedData instance from string.
func NewSharedData(data string) *SharedData {
	refCount := int64(1)
	return &SharedData{
		data:     str.UnsafeBytesFromString(data),
		refCount: &refCount, // Initial reference count is 1
	}
}

// NewSharedDataFromBytes creates a new SharedData instance from []byte.
func NewSharedDataFromBytes(data []byte) *SharedData {
	refCount := int64(1)
	return &SharedData{
		data:     data,
		refCount: &refCount,
	}
}

// Copy creates a copy of the SharedData using improved Copy-on-Write optimization.
func (sd *SharedData) Copy() *SharedData {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	// Both reading the data/refCount pointer and incrementing the reference count
	// must be done atomically under the same lock to prevent race conditions
	data := sd.data
	refCountPtr := sd.refCount
	parsedData := sd.parsedData
	dataVersion := sd.dataVersion

	// Increment reference count atomically while still holding the read lock
	// This prevents ensureUnique() from replacing the refCount pointer between
	// reading and incrementing
	atomic.AddInt64(refCountPtr, 1)

	// Return a new instance that shares the same data, refCount pointer, and parsed cache
	// Safe to share parsedData because:
	// 1. JSON parsing results are typically read-only
	// 2. When data is modified (SetData/SetBytes), COW mechanism ensures isolation
	// 3. ensureUnique() will clear parsedData when creating unique copies
	return &SharedData{
		data:        data,
		refCount:    refCountPtr, // Share the same reference count pointer
		parsedData:  parsedData,  // Share parsed cache for performance (protected by COW)
		dataVersion: dataVersion, // Copy the data version to maintain consistency
	}
}

// ensureUnique ensures the data is unique before modification using atomic operations
func (sd *SharedData) ensureUnique() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Use atomic Compare-And-Swap to safely check and decrement reference count
	// This prevents race conditions where multiple goroutines could pass the > 1 check
	// and then all decrement the counter
	for {
		currentRefCount := atomic.LoadInt64(sd.refCount)
		if currentRefCount <= 1 {
			// Already unique or only one reference, no need to copy
			return
		}

		// Try to atomically decrement the reference count
		// If CAS fails, another goroutine modified the refCount, so retry
		if atomic.CompareAndSwapInt64(sd.refCount, currentRefCount, currentRefCount-1) {
			// Successfully decremented, now create a unique copy
			break
		}
		// CAS failed, retry with the updated value
	}

	// Create a new copy of the data
	newData := make([]byte, len(sd.data))
	copy(newData, sd.data)

	// Set new data and create new reference count
	sd.data = newData
	newRefCount := int64(1)
	sd.refCount = &newRefCount
	// Note: Do NOT increment dataVersion here, as we're just creating a unique copy
	// with the same content. Version should only increment when data content changes.
	// Clear parsed data cache since we're creating a unique copy
	sd.parsedData = nil
}

// Get returns the data value as string using safe conversion.
func (sd *SharedData) Get() string {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if len(sd.data) == 0 {
		return ""
	}

	return str.SafeStringFromBytes(sd.data)
}

// GetUnsafe returns the data as string using zero-copy conversion.
func (sd *SharedData) GetUnsafe() string {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if len(sd.data) == 0 {
		return ""
	}

	return str.UnsafeStringFromBytes(sd.data)
}

// GetBytes returns the data as []byte.
//
// IMPORTANT: The returned []byte slice shares memory with the internal data and
// MUST NOT be modified. Any modification will corrupt the shared data and may
// cause data races. If you need to modify the data, use GetMutableBytes() instead.
func (sd *SharedData) GetBytes() []byte {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return sd.data
}

// GetMutableBytes returns a mutable copy of the data as []byte.
// This method ensures copy-on-write semantics and returns a []byte that can be safely modified
// without affecting other instances that share the same underlying data.
//
// Usage:
//
//	data := sharedData.GetMutableBytes()
//	data[0] = 'X' // Safe to modify
//	sharedData.SetBytes(data) // Optional: set the modified data back
func (sd *SharedData) GetMutableBytes() []byte {
	// First ensure we have a unique copy
	sd.ensureUnique()

	sd.mu.RLock()
	defer sd.mu.RUnlock()

	// Return a copy to prevent direct modification of internal data
	result := make([]byte, len(sd.data))
	copy(result, sd.data)
	return result
}

// String implements the fmt.Stringer interface.
func (sd *SharedData) String() string {
	return sd.Get()
}

// Set sets the data value, ensuring copy-on-write semantics.
func (sd *SharedData) Set(data string) {
	// Ensure unique copy first
	sd.ensureUnique()

	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Use zero-copy conversion for better performance
	sd.data = str.UnsafeBytesFromString(data)

	// Increment data version to invalidate cached parsed data
	sd.dataVersion++
	// Clear parsed data cache since data has changed
	sd.parsedData = nil

	// Notify data change if callback is set
	if sd.onDataChanged != nil {
		sd.onDataChanged()
	}
}

// SetUnsafe sets the data value using zero-copy conversion (use with caution).
func (sd *SharedData) SetUnsafe(data string) {
	// Ensure unique copy first
	sd.ensureUnique()

	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Use zero-copy conversion for performance
	sd.data = str.UnsafeBytesFromString(data)

	// Increment data version to invalidate cached parsed data
	sd.dataVersion++
	// Clear parsed data cache since data has changed
	sd.parsedData = nil

	// Notify data change if callback is set
	if sd.onDataChanged != nil {
		sd.onDataChanged()
	}
}

// SetBytes sets the data from []byte.
func (sd *SharedData) SetBytes(data []byte) {
	// Ensure unique copy first
	sd.ensureUnique()

	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Directly use the provided data
	sd.data = data

	// Increment data version to invalidate cached parsed data
	sd.dataVersion++
	// Clear parsed data cache since data has changed
	sd.parsedData = nil

	// Notify data change if callback is set
	if sd.onDataChanged != nil {
		sd.onDataChanged()
	}
}

// Len returns the length of the data.
func (sd *SharedData) Len() int {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return len(sd.data)
}

// IsEmpty checks if the data is empty.
func (sd *SharedData) IsEmpty() bool {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return len(sd.data) == 0
}

// GetRefCount returns the current reference count (for debugging/testing).
func (sd *SharedData) GetRefCount() int64 {
	return atomic.LoadInt64(sd.refCount)
}

// MarshalJSON implements the json.Marshaler interface.
func (sd *SharedData) MarshalJSON() ([]byte, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return json.Marshal(str.UnsafeStringFromBytes(sd.data))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (sd *SharedData) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Initialize refCount if it's nil (during JSON unmarshaling)
	if sd.refCount == nil {
		newRefCount := int64(1)
		sd.refCount = &newRefCount
	} else {
		// If refCount exists, ensure unique copy before replacing data
		if atomic.LoadInt64(sd.refCount) > 1 {
			atomic.AddInt64(sd.refCount, -1)
			newRefCount := int64(1)
			sd.refCount = &newRefCount
		}
	}

	sd.data = str.UnsafeBytesFromString(s)
	// Increment data version to invalidate cached parsed data
	sd.dataVersion++
	// Clear parsed data cache since data has changed
	sd.parsedData = nil
	return nil
}

// GetJsonData returns the data parsed as JSON with caching.
// If the data has already been parsed, returns cached result.
// If the data is not valid JSON, it returns an error.
// Returns map[string]interface{} for JSON objects or []interface{} for JSON arrays.
// This method is thread-safe and uses version-based validation to prevent caching stale data.
func (sd *SharedData) GetJsonData() (interface{}, error) {
	// First check if we have cached data (with read lock)
	sd.mu.RLock()
	if sd.parsedData != nil {
		cachedData := sd.parsedData
		sd.mu.RUnlock()
		return cachedData, nil
	}

	// Get data and version snapshot for parsing while still holding read lock
	dataStr := str.UnsafeStringFromBytes(sd.data)
	dataVersion := sd.dataVersion
	sd.mu.RUnlock()

	if dataStr == "" {
		return make(map[string]interface{}), nil
	}

	// Parse the JSON data outside of lock to reduce lock contention
	// Use the dataStr snapshot to avoid race conditions with concurrent modifications
	var result interface{}
	err := json.Unmarshal([]byte(dataStr), &result)
	if err != nil {
		return nil, err
	}

	// Cache the parsed data (with write lock) only if data version matches
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Version-based validation: only cache if the data hasn't changed since we read it
	if sd.dataVersion == dataVersion && sd.parsedData == nil {
		// Data version matches and no one else cached it, safe to cache our result
		sd.parsedData = result
		return result, nil
	} else if sd.parsedData != nil {
		// Someone else cached it while we were parsing, use the cached version
		return sd.parsedData, nil
	} else {
		// Data was modified while we were parsing (version mismatch)
		// Return our parsed result but don't cache it as it's based on stale data
		return result, nil
	}
}
