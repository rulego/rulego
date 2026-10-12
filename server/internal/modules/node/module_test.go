package node

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	serverEngine "github.com/rulego/rulego/server/internal/engine"
	"github.com/rulego/rulego/server/internal/store/filestore"
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

// newTestPoolServiceWithSystem 创建带系统节点 id 的测试 service（模拟 share_http_server 开启）。
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

// ====== 测试：系统节点保护（share_http_server 开启时，主 HTTP server 端点不可改/删/持久化）======

func TestSystemNodeProtection_RejectModifyDelete(t *testing.T) {
	svc := newTestPoolServiceWithSystem(":9090")
	defer svc.nodePool.Stop()

	// SaveNode 系统 id 应被拒绝
	err := svc.SaveNode(types.RuleNode{Id: ":9090", Type: "mqttClient"})
	assert.True(t, err != nil && strings.Contains(err.Error(), "system node"), "SaveNode 系统 id 应被拒绝")

	// SaveEndpoint 系统 id 应被拒绝
	err = svc.SaveEndpoint(types.EndpointDsl{RuleNode: types.RuleNode{Id: ":9090", Type: "endpoint/http"}})
	assert.True(t, err != nil && strings.Contains(err.Error(), "system node"), "SaveEndpoint 系统 id 应被拒绝")

	// Delete 系统 id 应被拒绝
	err = svc.Delete(":9090", "endpoint")
	assert.True(t, err != nil && strings.Contains(err.Error(), "system node"), "Delete 系统 id 应被拒绝")
}

func TestSaveState_SkipsSystemNode(t *testing.T) {
	svc := newTestPoolServiceWithSystem(":9090")
	defer svc.nodePool.Stop()

	// 模拟 Manager 注入的系统节点（直接 NewFromRuleNode，绕过 SaveNode 拦截）
	var sysNode types.RuleNode
	err := json.Unmarshal(mqttNodeJSON(":9090", "系统节点"), &sysNode)
	assert.Nil(t, err)
	_, err = svc.nodePool.NewFromRuleNode(sysNode)
	assert.Nil(t, err)

	// 普通节点
	var normalNode types.RuleNode
	err = json.Unmarshal(mqttNodeJSON("normal_mqtt", "普通节点"), &normalNode)
	assert.Nil(t, err)
	_, err = svc.nodePool.NewFromRuleNode(normalNode)
	assert.Nil(t, err)

	// saveState 应跳过系统节点
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

	// 普通节点（id != :9090）应正常保存
	var node types.RuleNode
	err := json.Unmarshal(mqttNodeJSON("normal_mqtt", "普通节点"), &node)
	assert.Nil(t, err)
	err = svc.SaveNode(node)
	assert.Nil(t, err)

	_, ok := svc.nodePool.Get("normal_mqtt")
	assert.True(t, ok, "普通节点应正常保存")
}

// 组件 id 与 DSL 内 ruleChain.id 不一致必须拒绝：重启加载按 DSL 内 id 注册，
// 不一致会让组件在重启后换类型，引用路径 id 的规则链节点找不到组件
func TestInstall_RejectsIdMismatch(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "workflows", "admin", "components"), 0755)
	cs, err := filestore.NewComponentStore(config.Config{DataDir: dir}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	svc := &UserComponentService{
		username:   "admin",
		ruleConfig: engine.NewConfig(),
		store:      cs,
	}
	dsl := func(id string) []byte {
		return []byte(fmt.Sprintf(`{"ruleChain":{"id":%q,"name":"t"},"metadata":{"nodes":[],"connections":[]}}`, id))
	}
	if err := svc.Install("mismatch-test", dsl("other-id")); err == nil {
		t.Fatal("expected id mismatch error")
	}
	if err := svc.Install("mismatch-test", dsl("mismatch-test")); err != nil {
		t.Fatalf("matching id should install: %v", err)
	}
}

// 冷启动回归：endpoint 引用 ref:// 共享节点的链在池为空时会整链初始化失败且无重试，
// 引擎创建 hook 必须保证 GetOrCreate 返回时全局池与用户池都已就绪。
func TestEngineCreatedHook_LoadsPoolAtEngineCreation(t *testing.T) {
	dir := t.TempDir()

	// 全局池文件：一个共享节点
	globalPoolFile := filepath.Join(dir, "node_pool_global.json")
	globalDsl := poolWithNodesJSON(1, 0)
	if err := os.WriteFile(globalPoolFile, globalDsl, 0644); err != nil {
		t.Fatal(err)
	}

	// 用户池 store：预置一个私有共享节点（store 存的是 saveState 产出的完整池 DSL）
	userStore, err := filestore.NewNodePoolStore(config.Config{DataDir: dir}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	userPoolDsl := `{"ruleChain":{"id":"node_pool","name":"Shared Node Pool"},"metadata":{"endpoints":[],"nodes":[` +
		string(mqttNodeJSON("hook_user_mqtt", "用户共享节点")) + `]}}`
	if err := userStore.Save([]byte(userPoolDsl)); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{DataDir: dir, DefaultUsername: "admin", NodePoolFile: globalPoolFile}
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := serverEngine.NewManager(cfg, logger, provider)

	m := New()
	m.cfg = cfg
	m.logger = logger
	m.storeProvider = provider
	m.container = app.NewContainer()
	m.engineMgr = mgr
	m.globalPoolData = globalDsl
	m.registerEngineHook()

	ue, err := mgr.GetOrCreate("admin")
	if err != nil {
		t.Fatal(err)
	}
	pool, ok := ue.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		t.Fatal("NodePool 类型不符")
	}
	if _, ok := pool.Get("pool_node_0"); !ok {
		t.Error("全局池节点未在引擎创建时加载")
	}
	if _, ok := pool.Get("hook_user_mqtt"); !ok {
		t.Error("用户私有池节点未在引擎创建时加载")
	}

	// 运行期懒创建的引擎同样走 hook：第二个用户创建即有全局池内容
	ue2, err := mgr.GetOrCreate("lateuser")
	if err != nil {
		t.Fatal(err)
	}
	pool2, ok := ue2.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		t.Fatal("NodePool 类型不符")
	}
	if _, ok := pool2.Get("pool_node_0"); !ok {
		t.Error("运行期创建的引擎未加载全局池")
	}
}

// coldstartEcho 测试探针节点：记录被路由命中
type coldstartEcho struct{}

var coldstartHits atomic.Int32

func (n *coldstartEcho) Type() string { return "coldstartEcho" }
func (n *coldstartEcho) New() types.Node {
	return &coldstartEcho{}
}
func (n *coldstartEcho) Init(_ types.Config, _ types.Configuration) error { return nil }
func (n *coldstartEcho) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	coldstartHits.Add(1)
	ctx.TellNext(msg)
}
func (n *coldstartEcho) Destroy() {}

func init() {
	_ = rulego.Registry.Register(&coldstartEcho{})
}

// 端到端复现 A 类冷启动故障场景：链的 endpoint 引用 ref:// 共享端点，
// 走真实 InitUserEngines 启动路径，断言链加载成功且共享端点能服务请求。
// 修复前（池在链之后加载）：endpoint Start 报 node resource not found，
// NewRuleChainEndpoint 回滚 → 整链初始化失败且无重试。
func TestColdStart_EndpointChainWithRefSharedEndpoint(t *testing.T) {
	dir := t.TempDir()
	port := nextPort()
	const chainId = "coldstart_e2e"

	// 预置：用户池 store 里一个共享 REST 端点
	userStore, err := filestore.NewNodePoolStore(config.Config{DataDir: dir}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	userPoolDsl := `{"ruleChain":{"id":"node_pool","name":"Shared Node Pool"},"metadata":{"endpoints":[{"id":"hook_shared_rest","type":"endpoint/http","name":"共享REST","configuration":{"server":"` + port + `"}}],"nodes":[]}}`
	if err := userStore.Save([]byte(userPoolDsl)); err != nil {
		t.Fatal(err)
	}

	// 预置：一条规则链，endpoint 的 server 指向 ref:// 共享端点
	provider := filestore.NewFileStoreProvider(config.Config{DataDir: dir}, types.DefaultLogger())
	ruleStore, err := provider.GetRuleStore("admin")
	if err != nil {
		t.Fatal(err)
	}
	chainDsl := `{
		"ruleChain": {"id":"` + chainId + `","name":"ColdStartE2E","root":true},
		"metadata": {
			"nodes": [{"id":"s1","type":"coldstartEcho","name":"ok"}],
			"connections": [],
			"endpoints": [{
				"id":"ep_coldstart","type":"endpoint/http","name":"ref-rest",
				"configuration":{"server":"ref://hook_shared_rest"},
				"routers":[{"id":"r1","params":["GET"],"from":{"path":"/coldstart-e2e"},"to":{"path":"` + chainId + `","wait":true}}]
			}]
		}
	}`
	if err := ruleStore.Save("admin", chainId, []byte(chainDsl)); err != nil {
		t.Fatal(err)
	}

	// 冷启动：manager + 引擎创建 hook + InitUserEngines（与 app 启动同路径）
	cfg := &config.Config{DataDir: dir, DefaultUsername: "admin"}
	logger := types.DefaultLogger()
	mgr := serverEngine.NewManager(cfg, logger, provider)
	m := New()
	m.cfg = cfg
	m.logger = logger
	m.storeProvider = provider
	m.container = app.NewContainer()
	m.engineMgr = mgr
	m.globalPoolData = nil
	m.registerEngineHook()
	if err := mgr.InitUserEngines(); err != nil {
		t.Fatal(err)
	}
	defer mgr.Stop()

	ueX, err := mgr.GetOrCreate("admin")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*serverEngine.UserEngine)

	// 断言 1：链已加载进引擎池
	loaded := false
	ue.Pool().Range(func(k, v any) bool {
		if k == chainId {
			loaded = true
		}
		return true
	})
	if !loaded {
		t.Fatal("endpoint 引用 ref:// 共享端点的链未加载（冷启动顺序回归）")
	}

	// 断言 2：共享端点在服务，且请求真的路由进了链（探针节点被命中）
	var resp *http.Response
	for i := 0; i < 20; i++ {
		r, err := http.Get("http://127.0.0.1" + port + "/coldstart-e2e")
		if err == nil {
			resp = r
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal("共享 REST 端点未在监听，无法服务 ref:// 引用它的链")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && coldstartHits.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if coldstartHits.Load() == 0 {
		t.Fatal("请求未路由进链（探针节点未命中）")
	}
}
