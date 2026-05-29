package node

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/test/assert"
)

// memoryNodePoolStore 内存实现的 NodePoolStore，用于测试
type memoryNodePoolStore struct {
	mu   sync.Mutex
	data []byte
}

func (s *memoryNodePoolStore) Save(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make([]byte, len(data))
	copy(s.data, data)
	return nil
}

func (s *memoryNodePoolStore) Get() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data, nil
}

// portCounter 用于为每个测试分配唯一端口，避免 REST endpoint 端口冲突
var portCounter int64 = 19000

func nextPort() string {
	portCounter++
	return fmt.Sprintf(":%d", portCounter)
}

// newTestPoolService 创建测试用的 UserNodePoolService
func newTestPoolService() *UserNodePoolService {
	config := engine.NewConfig()
	pool := node_pool.NewNodePool(config)
	config.NodePool = pool
	return &UserNodePoolService{
		store:    &memoryNodePoolStore{},
		nodePool: pool,
	}
}

// mqttNodeJSON 生成 mqtt 客户端节点 JSON（用指定 ID 和 name）
func mqttNodeJSON(id, name string) []byte {
	return []byte(fmt.Sprintf(`{
		"id": "%s",
		"type": "mqttClient",
		"name": "%s",
		"configuration": {
			"Server": "127.0.0.1:1883",
			"Topic": "/test/msg"
		}
	}`, id, name))
}

// endpointJSON 生成 REST endpoint JSON（用指定 ID 和唯一端口）
func endpointJSON(id, name string) []byte {
	return []byte(fmt.Sprintf(`{
		"id": "%s",
		"type": "endpoint/http",
		"name": "%s",
		"configuration": {
			"server": "%s"
		}
	}`, id, name, nextPort()))
}

// poolWithNodesJSON 生成包含指定数量 nodes 和 endpoints 的完整池 DSL
func poolWithNodesJSON(nodeCount, endpointCount int) []byte {
	nodes := ""
	for i := 0; i < nodeCount; i++ {
		if i > 0 {
			nodes += ","
		}
		nodes += fmt.Sprintf(`{
				"id": "pool_node_%d",
				"type": "mqttClient",
				"name": "池节点%d",
				"configuration": {"Server": "127.0.0.1:1883", "Topic": "/pool/msg"}
			}`, i, i)
	}
	endpoints := ""
	for i := 0; i < endpointCount; i++ {
		if i > 0 {
			endpoints += ","
		}
		p := nextPort()
		endpoints += fmt.Sprintf(`{
				"id": "pool_ep_%d",
				"type": "endpoint/http",
				"name": "池端点%d",
				"configuration": {"server": "%s"}
			}`, i, i, p)
	}
	return []byte(fmt.Sprintf(`{
		"ruleChain": {"id": "node_pool", "name": "Shared Node Pool"},
		"metadata": {
			"endpoints": [%s],
			"nodes": [%s]
		}
	}`, endpoints, nodes))
}

// ====== 测试：公共池 + 用户池合并 ======

func TestPoolMerge_PublicAndUser(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// 1. 加载公共池（1个node + 1个endpoint）
	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	_, ok := svc.nodePool.Get("pool_node_0")
	assert.True(t, ok, "公共池 node 应存在")
	_, ok = svc.nodePool.Get("pool_ep_0")
	assert.True(t, ok, "公共池 endpoint 应存在")

	// 2. 用户添加私有节点
	var node types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("user_mqtt_01", "用户MQTT"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// 3. 合并后两套节点都存在
	_, ok = svc.nodePool.Get("pool_node_0")
	assert.True(t, ok, "公共池 node 仍应存在")
	_, ok = svc.nodePool.Get("user_mqtt_01")
	assert.True(t, ok, "用户私有节点应存在")

	// 4. GetAllDef 包含全部
	defs, err := svc.nodePool.GetAllDef()
	assert.Nil(t, err)
	total := 0
	for _, nodes := range defs {
		total += len(nodes)
	}
	assert.Equal(t, 3, total, "公共池2个 + 用户1个 = 3个")
}

func TestPoolMerge_UserPoolReload(t *testing.T) {
	// 第一阶段：用户保存节点
	svc := newTestPoolService()
	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("persist_mqtt", "持久化MQTT"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	savedData, err := svc.store.Get()
	assert.Nil(t, err)
	assert.True(t, len(savedData) > 0)
	svc.nodePool.Stop()

	// 第二阶段：模拟重启，先加载公共池，再恢复用户数据
	svc2 := newTestPoolService()
	defer svc2.nodePool.Stop()

	globalDSL := poolWithNodesJSON(1, 0)
	_, err = svc2.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	svc2.store = &memoryNodePoolStore{data: savedData}
	err = svc2.Load()
	assert.Nil(t, err)

	_, ok := svc2.nodePool.Get("pool_node_0")
	assert.True(t, ok, "重启后公共池节点应存在")
	_, ok = svc2.nodePool.Get("persist_mqtt")
	assert.True(t, ok, "重启后用户私有节点应从 store 恢复")
}

// ====== 测试：添加后立即可用 ======

func TestSaveNode_ImmediateAvailability(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("imm_mqtt", "即时MQTT"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// 通过 Get 查找
	found, err := svc.Get("imm_mqtt", "node")
	assert.Nil(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "imm_mqtt", found.Id)
}

// ====== 测试：HTTP Endpoint 可见性 ======

func TestEndpointVisibility(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// 加载包含 endpoint 的 DSL
	globalDSL := poolWithNodesJSON(0, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	// endpoint 在 GetAll 中可见
	all := svc.nodePool.GetAll()
	assert.Equal(t, 1, len(all), "应有1个 endpoint")

	var def types.RuleNode
	err = json.Unmarshal(all[0].DSL(), &def)
	assert.Nil(t, err)
	assert.Equal(t, "pool_ep_0", def.Id)

	// endpoint 在 GetAllDef 中，key 以 "endpoint/" 为前缀
	defs, err := svc.nodePool.GetAllDef()
	assert.Nil(t, err)
	epNodes, hasEp := defs["endpoint/http"]
	assert.True(t, hasEp, "endpoint/http 类型应存在")
	if hasEp {
		assert.Equal(t, 1, len(epNodes))
		assert.Equal(t, "pool_ep_0", epNodes[0].Id)
	}
}

func TestSaveEndpoint_ImmediateAvailability(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	var endpointDef types.EndpointDsl
	epJSON := endpointJSON("imm_rest", "即时HTTP端点")
	err := json.Unmarshal(epJSON, &endpointDef)
	assert.Nil(t, err)

	err = svc.SaveEndpoint(endpointDef)
	assert.Nil(t, err)

	_, ok := svc.nodePool.Get("imm_rest")
	assert.True(t, ok, "SaveEndpoint 后应立即可获取")

	instance, err := svc.nodePool.GetInstance("imm_rest")
	assert.Nil(t, err)
	assert.NotNil(t, instance)
}

// ====== 测试：List 过滤与分页 ======

func TestList_FilterByCategory(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// 1个公共 node + 1个公共 endpoint
	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	// 再添加1个用户 node
	var node types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("user_node", "用户节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// 过滤 endpoint：应只有1个
	epList, epTotal, err := svc.List(1, 20, "", "endpoint")
	assert.Nil(t, err)
	assert.Equal(t, 1, epTotal, "应只有1个 endpoint")
	assert.Equal(t, 1, len(epList))

	// 过滤 node：应有2个（pool_node_0 + user_node）
	nodeList, nodeTotal, err := svc.List(1, 20, "", "node")
	assert.Nil(t, err)
	assert.Equal(t, 2, nodeTotal, "应有2个普通 node")
	assert.Equal(t, 2, len(nodeList))
	_ = nodeList
}

func TestList_KeywordFilter(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	var node types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("user_node", "用户节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// 按 "pool" 关键字过滤：匹配 pool_node_0 和 pool_ep_0
	_, total, err := svc.List(1, 20, "pool", "")
	assert.Nil(t, err)
	assert.Equal(t, 2, total, "匹配 'pool' 的应只有2个")

	// 无匹配
	_, total, err = svc.List(1, 20, "notexist", "")
	assert.Nil(t, err)
	assert.Equal(t, 0, total)
}

func TestList_Pagination(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	// 第1页，每页1条
	list, total, err := svc.List(1, 1, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 2, total, "总共2个节点")
	assert.Equal(t, 1, len(list))

	// 第2页
	list, total, err = svc.List(2, 1, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, 1, len(list))

	// 超出范围
	list, _, err = svc.List(10, 1, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(list))
}

// ====== 测试：CRUD 全流程 ======

func TestCRUD_RoundTrip(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// Create
	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("crud_mqtt", "CRUD测试MQTT"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// Read
	found, err := svc.Get("crud_mqtt", "node")
	assert.Nil(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "CRUD测试MQTT", found.Name)

	// Update（先删后加，因为 NewFromRuleNode 不支持重复 ID）
	svc.nodePool.Del("crud_mqtt")
	node.Name = "更新后的MQTT"
	node.Configuration = map[string]interface{}{
		"Server": "192.168.1.1:1883",
		"Topic":  "/updated",
	}
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	found, err = svc.Get("crud_mqtt", "node")
	assert.Nil(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "更新后的MQTT", found.Name)

	// Delete
	err = svc.Delete("crud_mqtt", "node")
	assert.Nil(t, err)

	found, err = svc.Get("crud_mqtt", "node")
	assert.Nil(t, err)
	assert.Nil(t, found, "删除后 Get 应返回 nil")
}

// ====== 测试：saveState 持久化 ======

func TestSaveState_PersistenceRoundTrip(t *testing.T) {
	// 保存 node + endpoint
	svc := newTestPoolService()
	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("persist_node", "持久化节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	var endpointDef types.EndpointDsl
	epJSON := endpointJSON("persist_ep", "持久化端点")
	err = json.Unmarshal(epJSON, &endpointDef)
	assert.Nil(t, err)
	err = svc.SaveEndpoint(endpointDef)
	assert.Nil(t, err)

	// 读取持久化数据
	savedData, err := svc.store.Get()
	assert.Nil(t, err)
	assert.True(t, len(savedData) > 0)

	// 验证序列化结构
	var saved types.RuleChain
	err = json.Unmarshal(savedData, &saved)
	assert.Nil(t, err)
	assert.Equal(t, "node_pool", saved.RuleChain.ID)
	assert.Equal(t, 1, len(saved.Metadata.Nodes), "应有1个 node")
	assert.Equal(t, 1, len(saved.Metadata.Endpoints), "应有1个 endpoint")
	svc.nodePool.Stop()

	// 在新 service 中恢复
	svc2 := newTestPoolService()
	defer svc2.nodePool.Stop()
	svc2.store = &memoryNodePoolStore{data: savedData}
	err = svc2.Load()
	assert.Nil(t, err)

	_, ok := svc2.nodePool.Get("persist_node")
	assert.True(t, ok, "恢复后 node 应存在")
	_, ok = svc2.nodePool.Get("persist_ep")
	assert.True(t, ok, "恢复后 endpoint 应存在")
}

// ====== 测试：GetAllDefs 结构正确 ======

func TestGetAllDefs_Structure(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	var node types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("defs_user_mqtt", "用户节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	defs, err := svc.nodePool.GetAllDef()
	assert.Nil(t, err)

	// mqttClient 类型应有2个节点
	mqttNodes, hasMqtt := defs["mqttClient"]
	assert.True(t, hasMqtt, "mqttClient 类型应存在")
	if hasMqtt {
		assert.Equal(t, 2, len(mqttNodes), "应有2个 mqttClient 节点")
	}

	// endpoint/http 类型应有1个
	epNodes, hasEp := defs["endpoint/http"]
	assert.True(t, hasEp, "endpoint/http 类型应存在")
	if hasEp {
		assert.Equal(t, 1, len(epNodes))
	}
}

// ====== 测试：删除/查询不存在节点 ======

func TestDelete_NonExistent(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()
	err := svc.Delete("not_exist", "node")
	assert.Nil(t, err, "删除不存在的节点不应报错")
}

func TestGet_NonExistent(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()
	found, err := svc.Get("not_exist", "node")
	assert.Nil(t, err)
	assert.Nil(t, found, "获取不存在的节点应返回 nil")
}
