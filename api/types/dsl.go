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

package types

// RuleChain 规则链定义
type RuleChain struct {
	//规则链基础信息定义
	RuleChain RuleChainBaseInfo `json:"ruleChain"`
	//包含了规则链中节点和连接的信息
	Metadata RuleMetadata `json:"metadata"`
}

// RuleChainBaseInfo 规则链基础信息定义
type RuleChainBaseInfo struct {
	//规则链ID
	ID string `json:"id"`
	//Name 规则链的名称
	Name string `json:"name"`
	//表示这个节点是否处于调试模式。如果为真，当节点处理消息时，会触发调试回调函数。
	//优先使用子节点的DebugMode配置
	DebugMode bool `json:"debugMode"`
	//Root 表示这个规则链是根规则链还是子规则链。(只做标记使用，非应用在实际逻辑)
	Root bool `json:"root"`
	//Configuration 规则链配置信息
	Configuration Configuration `json:"configuration,omitempty"`
	//扩展字段
	AdditionalInfo map[string]string `json:"additionalInfo,omitempty"`
}

// RuleMetadata 规则链元数据定义，包含了规则链中节点和连接的信息
type RuleMetadata struct {
	//数据流转的第一个节点，默认:0
	FirstNodeIndex int `json:"firstNodeIndex"`
	//节点组件定义
	//每个对象代表规则链中的一个规则节点
	Nodes []*RuleNode `json:"nodes"`
	//连接定义
	//每个对象代表规则链中两个节点之间的连接
	Connections []NodeConnection `json:"connections"`

	//Deprecated
	//使用 Flow Node代替
	//子规则链链接
	//每个对象代表规则链中一个节点和一个子规则链之间的连接
	RuleChainConnections []RuleChainConnection `json:"ruleChainConnections,omitempty"`
}

// RuleNode 规则链节点信息定义
type RuleNode struct {
	//节点的唯一标识符，可以是任意字符串
	Id string `json:"id"`
	//扩展字段
	AdditionalInfo NodeAdditionalInfo `json:"additionalInfo,omitempty"`
	//节点的类型，决定了节点的逻辑和行为。它应该与规则引擎中注册的节点类型之一匹配。
	Type string `json:"type"`
	//节点的名称，可以是任意字符串
	Name string `json:"name"`
	//表示这个节点是否处于调试模式。如果为真，当节点处理消息时，会触发调试回调函数。
	DebugMode bool `json:"debugMode"`
	//包含了节点的配置参数，具体内容取决于节点类型。
	//例如，一个JS过滤器节点可能有一个`jsScript`字段，定义了过滤逻辑，
	//而一个REST API调用节点可能有一个`restEndpointUrlPattern`字段，定义了要调用的URL。
	Configuration Configuration `json:"configuration"`
}

// NodeAdditionalInfo 用于可视化位置信息(预留字段)
type NodeAdditionalInfo struct {
	Description string `json:"description"`
	LayoutX     int    `json:"layoutX"`
	LayoutY     int    `json:"layoutY"`
}

// NodeConnection 规则链节点连接定义
// 每个对象代表规则链中两个节点之间的连接
type NodeConnection struct {
	//连接的源节点的id，应该与nodes数组中的某个节点id匹配。
	FromId string `json:"fromId"`
	//连接的目标节点的id，应该与nodes数组中的某个节点id匹配
	ToId string `json:"toId"`
	//连接的类型，决定了什么时候以及如何把消息从一个节点发送到另一个节点。它应该与源节点类型支持的连接类型之一匹配。
	//例如，一个JS过滤器节点可能支持两种连接类型："True"和"False"，表示消息是否通过或者失败过滤条件。
	Type string `json:"type"`
}

// RuleChainConnection 子规则链连接定义
// 每个对象代表规则链中一个节点和一个子规则链之间的连接
type RuleChainConnection struct {
	//连接的源节点的id，应该与nodes数组中的某个节点id匹配。
	FromId string `json:"fromId"`
	//连接的目标子规则链的id，应该与规则引擎中注册的子规则链之一匹配。
	ToId string `json:"toId"`
	//连接的类型，决定了什么时候以及如何把消息从一个节点发送到另一个节点。它应该与源节点类型支持的连接类型之一匹配。
	Type string `json:"type"`
}

// RuleChainRunSnapshot 规则链运行日志快照
type RuleChainRunSnapshot struct {
	RuleChain
	// Id 执行ID
	Id string `json:"Id"`
	// StartTs 执行开始时间
	StartTs int64 `json:"startTs"`
	// EndTs 执行结束时间
	EndTs int64 `json:"endTs"`
	// Logs 每个节点的日志
	Logs []RuleNodeRunLog `json:"logs"`
	//扩展字段
	AdditionalInfo map[string]string `json:"additionalInfo,omitempty"`
}

// RuleNodeRunLog 节点日志
type RuleNodeRunLog struct {
	// Id 节点ID
	Id string `json:"nodeId"`
	// InMsg 输入消息
	InMsg RuleMsg `json:"inMsg"`
	// OutMsg 输出消息
	OutMsg RuleMsg `json:"outMsg"`
	// RelationType 和下一个节点连接类型
	RelationType string `json:"relationType"`
	// Err 错误信息
	Err string `json:"err"`
	// LogItems 执行过程中的日志
	LogItems []string `json:"logItems"`
	// StartTs 执行开始时间
	StartTs int64 `json:"startTs"`
	// EndTs 执行结束时间
	EndTs int64 `json:"endTs"`
}
