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

// Metadata is a type for message metadata within the rule engine.
type Metadata map[string]string

// NewMetadata creates a new instance of rule engine message metadata.
func NewMetadata() Metadata {
	return make(Metadata)
}

// BuildMetadata creates a new instance of rule engine message metadata from a map.
func BuildMetadata(data Metadata) Metadata {
	if data == nil {
		return make(Metadata)
	}
	// Pre-allocate with known capacity to reduce map resizing
	metadata := make(Metadata, len(data))
	for k, v := range data {
		metadata[k] = v
	}
	return metadata
}

// Copy creates a copy of the metadata.
func (md Metadata) Copy() Metadata {
	return BuildMetadata(md)
}

// Has checks if a key exists in the metadata.
func (md Metadata) Has(key string) bool {
	_, ok := md[key]
	return ok
}

// GetValue retrieves a value by key from the metadata.
func (md Metadata) GetValue(key string) string {
	v, _ := md[key]
	return v
}

// PutValue sets a value in the metadata.
func (md Metadata) PutValue(key, value string) {
	if key != "" {
		md[key] = value
	}
}

// Values returns all key-value pairs in the metadata.
func (md Metadata) Values() map[string]string {
	return md
}

// RuleMsg is a type for messages within the rule engine.
type RuleMsg struct {
	// Ts is the message timestamp.
	Ts int64 `json:"ts"`
	// Id is the unique identifier for the message as it flows through the rule engine.
	Id string `json:"id"`
	// DataType is the type of data contained in the message.
	DataType DataType `json:"dataType"`
	// Type is a crucial field for the rule engine to distribute data. Messages are categorized and assigned a Type when processed by the rule engine (`ruleEngine.OnMsg(msg)`).
	// Examples include: POST_TELEMETRY, ACTIVITY_EVENT, INACTIVITY_EVENT, CONNECT_EVENT, DISCONNECT_EVENT, ENTITY_CREATED, ENTITY_UPDATED, ENTITY_DELETED, DEVICE_ALARM, POST_DEVICE_DATA.
	Type string `json:"type"`
	// Data is the content of the message.
	Data string `json:"data"`
	// Metadata contains metadata associated with the message.
	Metadata Metadata `json:"metadata"`
}

// NewMsg creates a new message instance and generates a message ID using UUID.
func NewMsg(ts int64, msgType string, dataType DataType, metaData Metadata, data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), ts, msgType, dataType, metaData, data)
}

func NewMsgWithJsonData(data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), 0, "", JSON, NewMetadata(), data)
}

// newMsg is a helper function to create a new RuleMsg.
func newMsg(id string, ts int64, msgType string, dataType DataType, metaData Metadata, data string) RuleMsg {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	if id == "" {
		uuId, _ := uuid.NewV4()
		id = uuId.String()
	}
	return RuleMsg{
		Ts:       ts,
		Id:       id,
		Type:     msgType,
		DataType: dataType,
		Data:     data,
		Metadata: metaData,
	}
}

// Copy creates a copy of the message.
func (m *RuleMsg) Copy() RuleMsg {
	return newMsg(m.Id, m.Ts, m.Type, m.DataType, m.Metadata.Copy(), m.Data)
}

// WrapperMsg is a type for wrapping the results of node execution, used to encapsulate the results of multiple nodes.
type WrapperMsg struct {
	// Msg is the message.
	Msg RuleMsg `json:"msg"`
	// Err is the error, if any occurred during node execution.
	Err string `json:"err"`
	// NodeId is the ID of the ending node.
	NodeId string `json:"nodeId"`
}
