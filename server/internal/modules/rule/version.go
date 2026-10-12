package rule

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

// 快照来源
const (
	// versionSourceMark 「保存」记版本时产生
	versionSourceMark = "mark"
	// versionSourceSave 「保存草稿」的兜底快照：同内容去重，按草稿档上限滚动裁剪
	versionSourceSave = "save"
	// versionSourceRollback 回滚到更早版本时留存
	versionSourceRollback = "rollback"
)

// initVersioning 挂接版本快照监听并注册服务。
// 配置关闭或存储未提供时版本功能整体不启用，不影响保存主流程。
func (m *Module) initVersioning(storeProvider store.StoreProvider) {
	if m.cfg != nil && m.cfg.RuleVersionDisable {
		return
	}
	sp, ok := storeProvider.(store.RuleVersionStoreProvider)
	if !ok {
		return
	}
	vs, err := sp.GetRuleVersionStore()
	if err != nil {
		m.logger.Warnf("rule version store unavailable, version history disabled: %s", err.Error())
		return
	}
	m.versions = vs
	m.verPending = make(map[string]string)
	m.AddLifecycleListener(&versionSnapshotter{m: m})
}

// versionSnapshotter 链生命周期监听：保存自动快照、删除联动清理。
type versionSnapshotter struct {
	services.BaseChainLifecycleListener
	m *Module
}

func (v *versionSnapshotter) OnSaved(e services.ChainLifecycleEvent) {
	v.m.writeVersionSnapshot(e)
}

func (v *versionSnapshotter) OnDeleted(e services.ChainLifecycleEvent) {
	if v.m.versions == nil {
		return
	}
	if err := v.m.versions.DeleteByChainId(e.Username, e.ChainId); err != nil {
		v.m.logger.Errorf("clean rule versions of chain %s: %s", e.ChainId, err.Error())
	}
}

func (m *Module) writeVersionSnapshot(e services.ChainLifecycleEvent) {
	if m.versions == nil || len(e.DSL) == 0 {
		return
	}
	source := m.takePending(e.Username, e.ChainId)
	if source == versionSourceSave {
		// 草稿快照：配置 0 不写；与最新快照同内容跳过——重启（原样重存）不产生记录
		if m.cfg != nil && m.cfg.RuleVersionDraftRetention <= 0 {
			return
		}
		if m.sameAsLatestSnapshot(e.Username, e.ChainId, e.DSL) {
			return
		}
	}
	ver := model.RuleVersion{
		Id:      newVersionId(),
		ChainId: e.ChainId,
		Ts:      time.Now().UnixMilli(),
		Source:  source,
		DslSize: len(e.DSL),
		Dsl:     append(json.RawMessage(nil), e.DSL...),
	}
	// 摘要字段解析失败不影响快照本身
	var head struct {
		RuleChain struct {
			Name string `json:"name"`
		} `json:"ruleChain"`
		Metadata struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(e.DSL, &head); err == nil {
		ver.ChainName = head.RuleChain.Name
		ver.NodeCount = len(head.Metadata.Nodes)
	}
	if err := m.versions.Save(e.Username, ver); err != nil {
		m.logger.Errorf("save rule version of chain %s: %s", e.ChainId, err.Error())
	}
}

// sameAsLatestSnapshot 判断 DSL 与该链最新一条快照的业务内容是否相同。
// 快照 DSL 均经 SaveAndLoad 规范化重序列化；additionalInfo 里的
// createTime/updateTime 是保存时自动打的时间戳（提交方不带 additionalInfo
// 时 createTime 也会重打），须剔除后比较，否则跨秒保存必判不等，去重失效。
// 读取失败按不同处理（fail-open，宁可多留一条快照）
func (m *Module) sameAsLatestSnapshot(username, chainId string, dsl []byte) bool {
	items, _, err := m.versions.List(username, chainId, 1, 1)
	if err != nil || len(items) == 0 {
		return false
	}
	latest, err := m.versions.Get(username, chainId, items[0].Id)
	if err != nil || len(latest.Dsl) == 0 {
		return false
	}
	return bytes.Equal(stableDsl(latest.Dsl), stableDsl(dsl))
}

// stableDsl 剔除自动时间戳后重序列化，作业务内容指纹；解析失败返回原字节
func stableDsl(dsl []byte) []byte {
	var v map[string]interface{}
	if err := json.Unmarshal(dsl, &v); err != nil {
		return dsl
	}
	if rc, ok := v["ruleChain"].(map[string]interface{}); ok {
		if ai, ok := rc["additionalInfo"].(map[string]interface{}); ok {
			delete(ai, "updateTime")
			delete(ai, "createTime")
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return dsl
	}
	return b
}

// setPending/takePending 给随后的一次 OnSaved 打来源标记。
// OnSaved 在 SaveAndLoad 内同步派发，标记在同 goroutine 内消费；
// 保存失败未被消费的由调用方清除，避免误标下一次保存。
func (m *Module) setPending(username, chainId, source string) {
	m.verSrcMu.Lock()
	defer m.verSrcMu.Unlock()
	m.verPending[username+"\x00"+chainId] = source
}

func (m *Module) takePending(username, chainId string) string {
	m.verSrcMu.Lock()
	defer m.verSrcMu.Unlock()
	key := username + "\x00" + chainId
	s, ok := m.verPending[key]
	if ok {
		delete(m.verPending, key)
	}
	if s == "" {
		return versionSourceSave
	}
	return s
}

func (m *Module) clearPending(username, chainId string) {
	m.verSrcMu.Lock()
	defer m.verSrcMu.Unlock()
	delete(m.verPending, username+"\x00"+chainId)
}

func newVersionId() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// ListVersions 实现 services.RuleVersionService
func (m *Module) ListVersions(username, chainId string, size, page int) ([]model.RuleVersion, int, error) {
	if m.versions == nil {
		return nil, 0, fmt.Errorf("rule version store disabled")
	}
	return m.versions.List(username, chainId, size, page)
}

// GetVersion 实现 services.RuleVersionService
func (m *Module) GetVersion(username, chainId, versionId string) (model.RuleVersion, error) {
	if m.versions == nil {
		return model.RuleVersion{}, fmt.Errorf("rule version store disabled")
	}
	return m.versions.Get(username, chainId, versionId)
}

// SaveVersion 实现 services.RuleVersionService：保存并把本次保存记为手动版本。
// 与 RollbackVersion 同款 pending 标记模式；版本功能关闭（versions==nil）时等价于普通保存
func (m *Module) SaveVersion(username, chainId string, def []byte) error {
	if m.versions != nil {
		m.setPending(username, chainId, versionSourceMark)
	}
	err := m.SaveAndLoad(username, chainId, def)
	if err != nil && m.versions != nil {
		m.clearPending(username, chainId)
	}
	return err
}

// RollbackVersion 实现 services.RuleVersionService。
// 取版本 DSL 后走公共 SaveAndLoad（同一把链锁，保存即部署语义一致）；
// 回滚本身经 OnSaved 再产生一个 source=rollback 的新版本，回滚可再回滚
func (m *Module) RollbackVersion(username, chainId, versionId string) error {
	if m.versions == nil {
		return fmt.Errorf("rule version store disabled")
	}
	ver, err := m.versions.Get(username, chainId, versionId)
	if err != nil {
		return err
	}
	if len(ver.Dsl) == 0 {
		return fmt.Errorf("rule version %s has no dsl", versionId)
	}
	m.setPending(username, chainId, versionSourceRollback)
	if err := m.SaveAndLoad(username, chainId, ver.Dsl); err != nil {
		m.clearPending(username, chainId)
		return err
	}
	return nil
}

var _ services.RuleVersionService = (*Module)(nil)
