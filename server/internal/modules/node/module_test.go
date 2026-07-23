package node

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/test/assert"
)

// memoryNodePoolStore memory-based NodePoolStore for testing
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

// portCounter is used to assign a unique port to each test to avoid REST endpoint port conflicts
var portCounter int64 = 19000

func nextPort() string {
	portCounter++
	return fmt.Sprintf(":%d", portCounter)
}

// newTestPoolService creates a UserNodePoolService for testing
func newTestPoolService() *UserNodePoolService {
	config := engine.NewConfig()
	pool := node_pool.NewNodePool(config)
	config.NodePool = pool
	return &UserNodePoolService{
		store:    &memoryNodePoolStore{},
		nodePool: pool,
	}
}

// mqttNodeJSON Generates mqtt client node JSON (with specified ID and name)
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

// endpointJSON generates REST endpoint JSON (with specified ID and unique port)
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

// poolWithNodesJSON generates a complete pool DSL containing a specified number of nodes and endpoints
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

// ====== Test: Merge public pool + user pool ======

func TestPoolMerge_PublicAndUser(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// 1. Load the public pool (1 node + 1 endpoint)
	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	_, ok := svc.nodePool.Get("pool_node_0")
	assert.True(t, ok, "公共池 node 应存在")
	_, ok = svc.nodePool.Get("pool_ep_0")
	assert.True(t, ok, "公共池 endpoint 应存在")

	// 2. Users add private nodes
	var node types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("user_mqtt_01", "用户MQTT"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// 3. Both sets of nodes exist after the merge
	_, ok = svc.nodePool.Get("pool_node_0")
	assert.True(t, ok, "公共池 node 仍应存在")
	_, ok = svc.nodePool.Get("user_mqtt_01")
	assert.True(t, ok, "用户私有节点应存在")

	// 4. GetAllDef includes everything
	defs, err := svc.nodePool.GetAllDef()
	assert.Nil(t, err)
	total := 0
	for _, nodes := range defs {
		total += len(nodes)
	}
	assert.Equal(t, 3, total, "公共池2个 + 用户1个 = 3个")
}

func TestPoolMerge_UserPoolReload(t *testing.T) {
	// Phase One: Users save nodes
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

	// Stage Two: Simulate a restart, first load the public pool, then restore user data
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

// ====== Test: Immediately available ====== after adding

func TestSaveNode_ImmediateAvailability(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("imm_mqtt", "即时MQTT"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// Find it through Get search
	found, err := svc.Get("imm_mqtt", "node")
	assert.Nil(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "imm_mqtt", found.Id)
}

// ====== Test: HTTP endpoint visibility ======

func TestEndpointVisibility(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// Load the DSL containing the endpoint
	globalDSL := poolWithNodesJSON(0, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	// Endpoint is visible in GetAll
	all := svc.nodePool.GetAll()
	assert.Equal(t, 1, len(all), "应有1个 endpoint")

	var def types.RuleNode
	err = json.Unmarshal(all[0].DSL(), &def)
	assert.Nil(t, err)
	assert.Equal(t, "pool_ep_0", def.Id)

	// In GetAllDef, the key is prefixed with "endpoint/"
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

// ====== Test: List filtering and pagination ======

func TestList_FilterByCategory(t *testing.T) {
	svc := newTestPoolService()
	defer svc.nodePool.Stop()

	// 1 public node + 1 public endpoint
	globalDSL := poolWithNodesJSON(1, 1)
	_, err := svc.nodePool.Load(globalDSL)
	assert.Nil(t, err)

	// Add one more user node
	var node types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("user_node", "用户节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	// Filter endpoint: There should be only one
	epList, epTotal, err := svc.List(1, 20, "", "endpoint")
	assert.Nil(t, err)
	assert.Equal(t, 1, epTotal, "应只有1个 endpoint")
	assert.Equal(t, 1, len(epList))

	// Filter nodes: should have 2 (pool_node_0 + user_node)
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

	// Filter by the "pool" keyword: match pool_node_0 and pool_ep_0
	_, total, err := svc.List(1, 20, "pool", "")
	assert.Nil(t, err)
	assert.Equal(t, 2, total, "匹配 'pool' 的应只有2个")

	// No matching
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

	// Page 1, one entry per page
	list, total, err := svc.List(1, 1, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 2, total, "总共2个节点")
	assert.Equal(t, 1, len(list))

	// Page 2
	list, total, err = svc.List(2, 1, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, 1, len(list))

	// Beyond the scope
	list, _, err = svc.List(10, 1, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(list))
}

// ====== Testing: CRUD full-process ======

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

	// Update (delete first, then add, because NewFromRuleNode does not support duplicate IDs)
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

// ====== Test: saveState persistence ======

func TestSaveState_PersistenceRoundTrip(t *testing.T) {
	// Save node + endpoint
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

	// Read persistence data
	savedData, err := svc.store.Get()
	assert.Nil(t, err)
	assert.True(t, len(savedData) > 0)

	// Verify serialized structures
	var saved types.RuleChain
	err = json.Unmarshal(savedData, &saved)
	assert.Nil(t, err)
	assert.Equal(t, "node_pool", saved.RuleChain.ID)
	assert.Equal(t, 1, len(saved.Metadata.Nodes), "应有1个 node")
	assert.Equal(t, 1, len(saved.Metadata.Endpoints), "应有1个 endpoint")
	svc.nodePool.Stop()

	// Restoring in the new service
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

// ====== Test: GetAllDefs structure correctly ======

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

	// The mqttClient type should have 2 nodes
	mqttNodes, hasMqtt := defs["mqttClient"]
	assert.True(t, hasMqtt, "mqttClient 类型应存在")
	if hasMqtt {
		assert.Equal(t, 2, len(mqttNodes), "应有2个 mqttClient 节点")
	}

	// There should be one endpoint/http type
	epNodes, hasEp := defs["endpoint/http"]
	assert.True(t, hasEp, "endpoint/http 类型应存在")
	if hasEp {
		assert.Equal(t, 1, len(epNodes))
	}
}

// ====== Test: Delete/query node ====== does not exist

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

// newTestPoolServiceWithSystem Create a test service with the system node ID (simulation share_http_server enabled).
func newTestPoolServiceWithSystem(systemNodeId string) *UserNodePoolService {
	config := engine.NewConfig()
	pool := node_pool.NewNodePool(config)
	config.NodePool = pool
	return &UserNodePoolService{
		store:        &memoryNodePoolStore{},
		nodePool:     pool,
		systemNodeId: systemNodeId,
	}
}

// ====== Testing: System node protection (when share_http_server enabled, the main HTTP server endpoint cannot be changed/deleted/persistent) ======

func TestSystemNodeProtection_RejectModifyDelete(t *testing.T) {
	svc := newTestPoolServiceWithSystem(":9090")
	defer svc.nodePool.Stop()

	// The SaveNode system ID should be denied
	err := svc.SaveNode(types.RuleNode{Id: ":9090", Type: "mqttClient"})
	assert.True(t, err != nil && strings.Contains(err.Error(), "system node"), "SaveNode 系统 id 应被拒绝")

	// The SaveEndpoint system ID should be denied
	err = svc.SaveEndpoint(types.EndpointDsl{RuleNode: types.RuleNode{Id: ":9090", Type: "endpoint/http"}})
	assert.True(t, err != nil && strings.Contains(err.Error(), "system node"), "SaveEndpoint 系统 id 应被拒绝")

	// Delete system ID should be denied
	err = svc.Delete(":9090", "endpoint")
	assert.True(t, err != nil && strings.Contains(err.Error(), "system node"), "Delete 系统 id 应被拒绝")
}

func TestSaveState_SkipsSystemNode(t *testing.T) {
	svc := newTestPoolServiceWithSystem(":9090")
	defer svc.nodePool.Stop()

	// Simulating the system nodes injected by Manager (directly using NewFromRuleNode to bypass SaveNode interception)
	var sysNode types.RuleNode
	err := json.Unmarshal(mqttNodeJSON(":9090", "系统节点"), &sysNode)
	assert.Nil(t, err)
	_, err = svc.nodePool.NewFromRuleNode(sysNode)
	assert.Nil(t, err)

	// Ordinary nodes
	var normalNode types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("normal_mqtt", "普通节点"), &normalNode)
	assert.Nil(t, err)
	_, err = svc.nodePool.NewFromRuleNode(normalNode)
	assert.Nil(t, err)

	// saveState should skip system nodes
	err = svc.saveState()
	assert.Nil(t, err)

	saved, err := svc.store.Get()
	assert.Nil(t, err)
	var poolDef types.RuleChain
	err = json.Unmarshal(saved, &poolDef)
	assert.Nil(t, err)

	hasSystem, hasNormal := false, false
	for _, n := range poolDef.Metadata.Nodes {
		if n.Id == ":9090" {
			hasSystem = true
		}
		if n.Id == "normal_mqtt" {
			hasNormal = true
		}
	}
	assert.True(t, !hasSystem, "系统节点 :9090 不应被持久化")
	assert.True(t, hasNormal, "普通节点应被持久化")
}

func TestSystemNodeProtection_NormalNodeUnaffected(t *testing.T) {
	svc := newTestPoolServiceWithSystem(":9090")
	defer svc.nodePool.Stop()

	// Ordinary nodes (id!=:9090) should be saved normally
	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("normal_mqtt", "普通节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	_, ok := svc.nodePool.Get("normal_mqtt")
	assert.True(t, ok, "普通节点应正常保存")
}
