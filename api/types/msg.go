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
// It helps components understand how to process the message payload.
//
// DataType 定义消息中包含的数据类型。
// 它帮助组件理解如何处理消息负载。
type DataType string

// Constants for different data types that a message can represent.
// These types guide component behavior and processing logic.
// 表示消息可以代表的不同数据类型的常量。
// 这些类型指导组件行为和处理逻辑。
const (
	// JSON represents data in JSON format - most common for structured data
	// JSON 表示 JSON 格式的数据 - 结构化数据最常用的格式
	JSON = DataType("JSON")

	// TEXT represents plain text data - used for simple string content
	// TEXT 表示纯文本数据 - 用于简单的字符串内容
	TEXT = DataType("TEXT")

	// BINARY represents binary data - used for files, images, or other binary content
	// BINARY 表示二进制数据 - 用于文件、图像或其他二进制内容
	BINARY = DataType("BINARY")
)

// Constants for keys used in message handling and metadata operations.
// These standardized keys ensure consistency across the rule engine.
// 用于消息处理和元数据操作的键常量。
// 这些标准化的键确保规则引擎的一致性。
const (
	IdKey       = "id"       // Key for the message unique identifier  消息唯一标识符的键
	TsKey       = "ts"       // Key for the message timestamp  消息时间戳的键
	DataKey     = "data"     // Key for the message content  消息内容的键
	MsgKey      = "msg"      // Key for the message content object  消息内容对象的键
	MetadataKey = "metadata" // Key for the message metadata  消息元数据的键
	MsgTypeKey  = "msgType"  // Key for the message type  消息类型的键
	DataTypeKey = "dataType" // Key for the data type of the message  消息数据类型的键
)

// Properties is a simple map type for storing key-value pairs as metadata.
// It provides basic operations for metadata management without Copy-on-Write optimization.
// This type is suitable for scenarios where performance is not critical or when
// metadata sharing between multiple instances is not required.
//
// Properties 是用于存储键值对作为元数据的简单映射类型。
// 它提供基本的元数据管理操作，但不包含写时复制优化。
// 此类型适用于性能不关键或不需要在多个实例间共享元数据的场景。
type Properties map[string]string

// NewProperties creates a new empty Properties instance.
// Returns an initialized Properties map ready for use.
//
// NewProperties 创建一个新的空 Properties 实例。
// 返回一个已初始化的 Properties 映射，可立即使用。
func NewProperties() Properties {
	return make(Properties)
}

// BuildProperties creates a new Properties instance from existing data.
// If the input data is nil, returns an empty Properties instance.
// The function creates a deep copy of the input data to ensure isolation.
//
// BuildProperties 从现有数据创建新的 Properties 实例。
// 如果输入数据为 nil，返回空的 Properties 实例。
// 该函数创建输入数据的深度副本以确保隔离。
func BuildProperties(data Properties) Properties {
	if data == nil {
		return make(Properties)
	}
	// Pre-allocate with known capacity to reduce map resizing
	// 预分配已知容量以减少映射重新调整大小
	metadata := make(Properties, len(data))
	for k, v := range data {
		metadata[k] = v
	}
	return metadata
}

// Copy creates a deep copy of the Properties.
// This ensures that modifications to the copy do not affect the original.
//
// Copy 创建 Properties 的深度副本。
// 这确保对副本的修改不会影响原始数据。
func (md Properties) Copy() Properties {
	return BuildProperties(md)
}

// Has checks if a key exists in the metadata.
// Returns true if the key exists, false otherwise.
//
// Has 检查元数据中是否存在键。
// 如果键存在返回 true，否则返回 false。
func (md Properties) Has(key string) bool {
	_, ok := md[key]
	return ok
}

// GetValue retrieves a value by key from the metadata.
// Returns the value if the key exists, or an empty string if not found.
//
// GetValue 通过键从元数据中检索值。
// 如果键存在返回值，如果未找到返回空字符串。
func (md Properties) GetValue(key string) string {
	v, _ := md[key]
	return v
}

// PutValue sets a value in the metadata.
// If the key is empty, the operation is ignored to prevent invalid entries.
//
// PutValue 在元数据中设置值。
// 如果键为空，操作将被忽略以防止无效条目。
func (md Properties) PutValue(key, value string) {
	if key != "" {
		md[key] = value
	}
}

// Values returns the underlying map containing all key-value pairs.
// Note: This returns a direct reference to the internal map, so modifications
// will affect the original Properties instance.
//
// Values 返回包含所有键值对的底层映射。
// 注意：这返回内部映射的直接引用，因此修改会影响原始 Properties 实例。
func (md Properties) Values() map[string]string {
	return md
}

// Metadata is a type for message metadata within the rule engine.
// It uses Copy-on-Write mechanism to optimize performance in multi-node scenarios.
//
// Metadata 是规则引擎中消息元数据的类型。
// 它使用写时复制机制来优化多节点场景下的性能。
//
// Key Features:
// 主要特性：
//   - Copy-on-Write optimization for better performance  写时复制优化以获得更好的性能
//   - Thread-safe operations with mutex protection  使用互斥锁保护的线程安全操作
//   - Efficient sharing between multiple rule nodes  多个规则节点间的高效共享
//   - JSON marshaling/unmarshaling support  JSON 序列化/反序列化支持
type Metadata struct {
	// data holds the actual metadata key-value pairs
	// data 保存实际的元数据键值对
	data map[string]string

	// shared indicates if this metadata is shared with other instances
	// shared 表示此元数据是否与其他实例共享
	shared bool

	// mu protects the shared flag and data during copy operations
	// mu 在复制操作期间保护共享标志和数据
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

// GetReadOnlyValues returns the underlying metadata map without copying.
// WARNING: The returned map MUST NOT be modified. It's intended for read-only access only.
// Modifying the returned map will corrupt the shared data and may cause data races.
// Use this method only when you need read-only access and want to avoid allocation overhead.
//
// For safe modification, use Values() which returns a copy.
//
// Example usage:
//
//	values := metadata.GetReadOnlyValues()
//	for k, v := range values { // Read-only iteration is safe
//		fmt.Printf("%s: %s\n", k, v)
//	}
func (md *Metadata) GetReadOnlyValues() map[string]string {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.data // Zero-copy, but caller must not modify
}

// ForEach iterates over all key-value pairs in the metadata using a callback function.
// This method provides zero-copy iteration without exposing the internal map.
// The iteration will stop early if the callback function returns false.
//
// This is the safest zero-copy method as it doesn't expose the internal map.
//
// Example usage:
//
//	metadata.ForEach(func(key, value string) bool {
//		fmt.Printf("%s: %s\n", key, value)
//		return true // Continue iteration
//	})
func (md *Metadata) ForEach(fn func(key, value string) bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	for k, v := range md.data {
		if !fn(k, v) {
			break
		}
	}
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
//
// RuleMsg 表示规则引擎系统中的消息。
// 它封装了消息处理所需的所有信息，包括消息内容、元数据、类型信息和时间详情。
// RuleMsg 是流经规则引擎节点的核心数据结构。
//
// Key Features:
// 主要特性：
//   - Copy-on-Write optimization for data and metadata  数据和元数据的写时复制优化
//   - Support for multiple data types (JSON, TEXT, BINARY)  支持多种数据类型（JSON、TEXT、BINARY）
//   - Efficient copying and sharing between nodes  节点间的高效复制和共享
//
// Usage Example:
// 使用示例：
//
//	// Create a new message with JSON data
//	// 创建包含 JSON 数据的新消息
//	metadata := types.NewMetadata()
//	metadata.PutValue("deviceId", "sensor001")
//	msg := types.NewMsg(0, "TELEMETRY", types.JSON, metadata, `{"temperature": 25.5}`)
//
//	// Copy message efficiently (uses Copy-on-Write)
//	// 高效复制消息（使用写时复制）
//	msgCopy := msg.Copy()
//
//	// Modify copy without affecting original
//	// 修改副本而不影响原始消息
//	msgCopy.SetData(`{"temperature": 26.0}`)
type RuleMsg struct {
	// Ts is the message timestamp in milliseconds since Unix epoch.
	// This field is automatically set when creating a new message if not provided.
	// Ts 是自 Unix 纪元以来的消息时间戳（毫秒）。
	// 如果未提供，创建新消息时会自动设置此字段。
	Ts int64 `json:"ts"`

	// Id is the unique identifier for the message as it flows through the rule engine.
	// Each message gets a UUID when created, ensuring uniqueness across the system.
	// Id 是消息在规则引擎中流转时的唯一标识符。
	// 每个消息在创建时都会获得一个 UUID，确保在系统中的唯一性。
	Id string `json:"id"`

	// DataType specifies the format of the data contained in the message.
	// Supported types include JSON, TEXT, and BINARY.
	// DataType 指定消息中包含数据的格式。
	// 支持的类型包括 JSON、TEXT 和 BINARY。
	DataType DataType `json:"dataType"`

	// Type is a crucial field for the rule engine to distribute and categorize messages.
	// This field determines how the rule engine routes and processes the message.
	// Common examples include: POST_TELEMETRY, ACTIVITY_EVENT, INACTIVITY_EVENT,
	// CONNECT_EVENT, DISCONNECT_EVENT, ENTITY_CREATED, ENTITY_UPDATED, ENTITY_DELETED,
	// DEVICE_ALARM, POST_DEVICE_DATA.
	// Type 是规则引擎分发和分类消息的关键字段。
	// 此字段决定规则引擎如何路由和处理消息。
	// 常见示例包括：POST_TELEMETRY、ACTIVITY_EVENT、INACTIVITY_EVENT、
	// CONNECT_EVENT、DISCONNECT_EVENT、ENTITY_CREATED、ENTITY_UPDATED、ENTITY_DELETED、
	// DEVICE_ALARM、POST_DEVICE_DATA。
	Type string `json:"type"`

	// Data contains the actual message payload using Copy-on-Write optimization.
	// The format of this data should match the DataType field.
	// Data 包含使用写时复制优化的实际消息负载。
	// 此数据的格式应与 DataType 字段匹配。
	Data *SharedData `json:"data"`

	// Metadata contains additional key-value pairs associated with the message.
	// This field uses Copy-on-Write optimization for better performance in multi-node scenarios.
	// Metadata 包含与消息相关的附加键值对。
	// 此字段使用写时复制优化，在多节点场景下提供更好的性能。
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
