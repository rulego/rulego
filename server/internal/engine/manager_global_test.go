package engine

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	rulegoEngine "github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/server/internal/constants"
)

// testGlobalProbe 计数型探针组件：每次 Init 记录当时配置解析出的 probe 值。
// 用于断言「重载真的重建了实例、且拿到的是新 global 值」与「未引用的链/节点没被重载」。
type testGlobalProbe struct {
	probe string
}

var probeStats struct {
	sync.Mutex
	inits     int
	lastProbe string
}

func (n *testGlobalProbe) Type() string { return "testGlobalProbe" }

func (n *testGlobalProbe) New() types.Node { return &testGlobalProbe{} }

func (n *testGlobalProbe) Init(config types.Config, configuration types.Configuration) error {
	if v, ok := configuration["v"].(string); ok {
		n.probe = v
	}
	probeStats.Lock()
	probeStats.inits++
	probeStats.lastProbe = n.probe
	probeStats.Unlock()
	return nil
}

func (n *testGlobalProbe) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellNext(msg)
}

func (n *testGlobalProbe) Destroy() {}

func (n *testGlobalProbe) GetInstance() (interface{}, error) { return n, nil }

func init() {
	_ = rulegoEngine.Registry.Register(&testGlobalProbe{})
}

func resetProbeStats() {
	probeStats.Lock()
	probeStats.inits = 0
	probeStats.lastProbe = ""
	probeStats.Unlock()
}

func probeSnapshot() (int, string) {
	probeStats.Lock()
	defer probeStats.Unlock()
	return probeStats.inits, probeStats.lastProbe
}

func chainDsl(id, name, probeKey string) []byte {
	nodeConf := map[string]interface{}{"v": "${global." + probeKey + "}"}
	return buildChainDsl(id, name, nodeConf)
}

// staticChainDsl 完全不引用 global 的对照链：任何 global 变更都不应重载它
func staticChainDsl(id, name string) []byte {
	return buildChainDsl(id, name, map[string]interface{}{"v": "static"})
}

func buildChainDsl(id, name string, nodeConf map[string]interface{}) []byte {
	dsl := map[string]interface{}{
		"ruleChain": map[string]interface{}{"id": id, "name": name, "root": true},
		"metadata": map[string]interface{}{
			"nodes": []map[string]interface{}{
				{"id": "n1", "type": "testGlobalProbe", "configuration": nodeConf},
			},
			"endpoints": []interface{}{},
		},
	}
	b, _ := json.Marshal(dsl)
	return b
}

func TestGlobalRefHit(t *testing.T) {
	dsl := `{"nodes":[{"configuration":{"a":"${global.key}","b":"${global.key1}","c":"${global[\"key\"]}","d":"${global['key']}"}}]}`
	if !globalRefHit(dsl, []string{"key"}) {
		t.Error("应命中 ${global.key}")
	}
	if !globalRefHit(dsl, []string{"key1"}) {
		t.Error("应命中 ${global.key1}")
	}
	// DSL 存在 global. 引用（虽非变更键）：JS 运行时可能访问任意键，保守命中
	if !globalRefHit(dsl, []string{"other"}) {
		t.Error("存在 global. 引用的 DSL 应保守命中（JS 运行时盲区）")
	}
	// key 不应误中 key1：构造只含 key1 模板的 DSL，用 key 探测——但含 global. 引用即兜底命中，
	// 该语义下此 case 也为 true，由上一断言覆盖；真正要防的是无引用的 DSL 误命中
	if globalRefHit(`{"v":"no ref here"}`, []string{"key"}) {
		t.Error("无任何 global 引用不应命中")
	}
	// JS 运行时引用：DSL 无 ${} 痕迹也应命中
	if !globalRefHit(`{"jsScript":"return global.probe === 'v1';"}`, []string{"probe"}) {
		t.Error("JS 运行时 global.probe 引用应命中")
	}
	if !globalRefHit(`{"jsScript":"return global.probe === 'v1';"}`, []string{"other"}) {
		t.Error("JS 引用无法定位键，任意变更键都应保守命中")
	}
	if globalRefHit(`{"v":"${vars.x} ${msg.y}"}`, []string{"key"}) {
		t.Error("vars/msg 模板与 global 无关，不应命中")
	}
}

func TestUpdateGlobalPropertiesReplacesAndKeepsServerKeys(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ueX, err := mgr.GetOrCreate("globaluser")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*UserEngine)
	ue.UpdateGlobalProperties(map[string]string{"probe": "v1", "keep": "k"})

	cfg := ue.RuleConfig()
	if got := cfg.Properties.GetValue("probe"); got != "v1" {
		t.Errorf("probe = %q, want v1", got)
	}
	// server 级键必须保留：热更换表后 exec 白名单等不能丢
	if _, ok := cfg.Properties[constants.LoadLuaLibs]; !ok {
		t.Error("server 键 LoadLuaLibs 丢失")
	}
	if !cfg.Properties.Has(action.KeyWorkDir) {
		t.Error("server 键 DataDir(workDir) 丢失")
	}

	// 全量替换语义：旧表中未被新表包含的键应消失
	ue.UpdateGlobalProperties(map[string]string{"probe": "v2"})
	cfg = ue.RuleConfig()
	if got := cfg.Properties.GetValue("probe"); got != "v2" {
		t.Errorf("probe = %q, want v2", got)
	}
	if cfg.Properties.Has("keep") {
		t.Error("全量替换语义下，新表没有的旧键应被移除")
	}

	// 共享节点池的 config 须同步换新
	np, ok := cfg.NodePool.(*node_pool.NodePool)
	if !ok {
		t.Fatal("ruleConfig.NodePool 不是 *node_pool.NodePool")
	}
	if np.Config.Properties.GetValue("probe") != "v2" {
		t.Error("共享节点池 Config 未同步新 global")
	}
}

func TestUpdateGlobalPropertiesConcurrent(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ueX, err := mgr.GetOrCreate("raceuser")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*UserEngine)
	ue.UpdateGlobalProperties(map[string]string{"probe": "v0"})

	done := make(chan struct{})
	go func() { // 读者：模拟链保存并发取 config
		defer close(done)
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			cfg := ue.RuleConfig()
			_ = cfg.Properties.GetValue("probe")
		}
	}()
	for i := 1; i <= 50; i++ {
		ue.UpdateGlobalProperties(map[string]string{"probe": "v" + itoa(i)})
	}
	<-done
	if got := ue.RuleConfig().Properties.GetValue("probe"); got != "v50" {
		t.Errorf("最终值 = %q, want v50", got)
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestReloadChainsReferencingGlobal(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ueX, err := mgr.GetOrCreate("reloaduser")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*UserEngine)
	ue.UpdateGlobalProperties(map[string]string{"probe": "old"})

	resetProbeStats()
	if _, err := ue.Pool().New("chainA", chainDsl("chainA", "A链", "probe"), rulego.WithConfig(ue.RuleConfig())); err != nil {
		t.Fatal(err)
	}
	if _, err := ue.Pool().New("chainB", staticChainDsl("chainB", "B链"), rulego.WithConfig(ue.RuleConfig())); err != nil {
		t.Fatal(err)
	}
	inits, last := probeSnapshot()
	if inits != 2 || last != "static" {
		t.Fatalf("初始加载后 inits=%d last=%q, want 2/static", inits, last)
	}

	// 热更 + 精准重载：只动引用了 probe 的链 A
	ue.UpdateGlobalProperties(map[string]string{"probe": "new"})
	reloaded, failed := ue.ReloadChainsReferencingGlobal([]string{"probe"})
	if len(failed) != 0 {
		t.Fatalf("重载失败: %v", failed)
	}
	if len(reloaded) != 1 || reloaded[0] != "A链" {
		t.Fatalf("reloaded = %v, want [A链]", reloaded)
	}
	inits, last = probeSnapshot()
	if inits != 3 {
		t.Fatalf("只有链 A 重载，inits=%d, want 3", inits)
	}
	if last != "new" {
		t.Fatalf("重载后探针值 = %q, want new", last)
	}

	// 任意 global 变更：有引用的链 A 兜底重载，无引用的链 B（对照）不动
	before, _ := probeSnapshot()
	reloaded, _ = ue.ReloadChainsReferencingGlobal([]string{"absent"})
	if len(reloaded) != 1 || reloaded[0] != "A链" {
		t.Fatalf("兜底语义：有引用的链应重载, got %v", reloaded)
	}
	if after, last := probeSnapshot(); after != before+1 || last != "new" {
		t.Fatalf("B 链（无引用）不应重载: inits=%d last=%q", after, last)
	}
}

func TestReloadSharedNodesReferencingGlobal(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ueX, err := mgr.GetOrCreate("nodeuser")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*UserEngine)
	ue.UpdateGlobalProperties(map[string]string{"probe": "old"})

	np, ok := ue.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		t.Fatal("ruleConfig.NodePool 不是 *node_pool.NodePool")
	}

	resetProbeStats()
	nodeDef := types.RuleNode{
		Id:          "sharedProbe",
		Type:        "testGlobalProbe",
		Configuration: types.Configuration{
			"v":       "${global.probe}",
		"server":  "${global.skill_path}",
		},
	}
	if _, err := np.NewFromRuleNode(nodeDef); err != nil {
		t.Fatal(err)
	}
	if _, err := np.NewFromRuleNode(types.RuleNode{
		Id: "plainNode", Type: "testGlobalProbe",
		Configuration: types.Configuration{"v": "static"},
	}); err != nil {
		t.Fatal(err)
	}
	if inits, last := probeSnapshot(); inits != 2 || last != "static" {
		t.Fatalf("池初始化后 inits=%d last=%q, want 2/static", inits, last)
	}

	ue.UpdateGlobalProperties(map[string]string{"probe": "new"})
	reloaded, failed := ue.ReloadSharedNodesReferencingGlobal([]string{"probe"})
	if len(failed) != 0 {
		t.Fatalf("共享节点重载失败: %v", failed)
	}
	if len(reloaded) != 1 || reloaded[0] != "sharedProbe" {
		t.Fatalf("reloaded = %v, want [sharedProbe]", reloaded)
	}
	if inits, last := probeSnapshot(); inits != 4 || last != "new" {
		// 4 = 池初始 2 + 试 Init 验证 1 + 正式重建 1
		t.Fatalf("重载后 inits=%d last=%q, want 4/new", inits, last)
	}

	// 重建后池内仍可寻址（Del+New 未破坏注册）
	if _, ok := np.Get("sharedProbe"); !ok {
		t.Error("重载后共享节点寻址失败")
	}
}

func TestPropagateGlobalAggregates(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ueX, err := mgr.GetOrCreate("agguser")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*UserEngine)
	ue.UpdateGlobalProperties(map[string]string{"probe": "old"})
	resetProbeStats()
	if _, err := ue.Pool().New("aggChain", chainDsl("aggChain", "聚合链", "probe"), rulego.WithConfig(ue.RuleConfig())); err != nil {
		t.Fatal(err)
	}

	result := mgr.PropagateGlobal(map[string]string{"probe": "new"}, []string{"probe"})
	if len(result.FailedChains) != 0 || len(result.FailedNodes) != 0 {
		t.Fatalf("失败明细非空: %+v", result)
	}
	if len(result.ReloadedChains) != 1 || result.ReloadedChains[0] != "聚合链" {
		t.Fatalf("ReloadedChains = %v", result.ReloadedChains)
	}
	if inits, last := probeSnapshot(); inits != 2 || last != "new" {
		t.Fatalf("inits=%d last=%q, want 2/new", inits, last)
	}

	// 空变更键直接返回空结果（防御路径）
	empty := mgr.PropagateGlobal(map[string]string{}, nil)
	if len(empty.ReloadedChains) != 0 {
		t.Fatal("空键不应重载")
	}
}

func TestReloadSharedNodesSkipsSystemNode(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ueX, err := mgr.GetOrCreate("sysnodeuser")
	if err != nil {
		t.Fatal(err)
	}
	ue := ueX.(*UserEngine)
	ue.UpdateGlobalProperties(map[string]string{"probe": "old"})

	np, ok := ue.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		t.Fatal("ruleConfig.NodePool 不是 *node_pool.NodePool")
	}
	// 伪造成系统注入节点：引用 global 但标记为 systemNodeId，热更不得重建
	nodeDef := types.RuleNode{Id: "sysProbe", Type: "testGlobalProbe",
		Configuration: types.Configuration{"v": "${global.probe}"}}
	if _, err := np.NewFromRuleNode(nodeDef); err != nil {
		t.Fatal(err)
	}
	ue.systemNodeId = "sysProbe"

	resetProbeStats()
	reloaded, failed := ue.ReloadSharedNodesReferencingGlobal([]string{"probe"})
	if len(failed) != 0 {
		t.Fatalf("不应有失败: %v", failed)
	}
	if len(reloaded) != 0 {
		t.Fatalf("系统节点应被跳过, got %v", reloaded)
	}
	if _, ok := np.Get("sysProbe"); !ok {
		t.Fatal("系统节点不应被动过")
	}
}
