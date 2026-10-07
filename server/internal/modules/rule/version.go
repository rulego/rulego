package rule

import (
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
	versionSourceSave     = "save"
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
	m.verPendingSource = make(map[string]string)
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
	ver := model.RuleVersion{
		Id:      newVersionId(),
		ChainId: e.ChainId,
		Ts:      time.Now().UnixMilli(),
		Source:  m.takePendingSource(e.Username, e.ChainId),
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

// setPendingSource/takePendingSource 给随后的一次 OnSaved 打来源标记。
// OnSaved 在 SaveAndLoad 内同步派发，标记在同 goroutine 内消费；
// 保存失败未被消费的由 RollbackVersion 清除，避免误标下一次保存。
func (m *Module) setPendingSource(username, chainId, source string) {
	m.verSrcMu.Lock()
	defer m.verSrcMu.Unlock()
	m.verPendingSource[username+"\x00"+chainId] = source
}

func (m *Module) takePendingSource(username, chainId string) string {
	m.verSrcMu.Lock()
	defer m.verSrcMu.Unlock()
	key := username + "\x00" + chainId
	s, ok := m.verPendingSource[key]
	if ok {
		delete(m.verPendingSource, key)
	}
	if s == "" {
		return versionSourceSave
	}
	return s
}

func (m *Module) clearPendingSource(username, chainId string) {
	m.verSrcMu.Lock()
	defer m.verSrcMu.Unlock()
	delete(m.verPendingSource, username+"\x00"+chainId)
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
	m.setPendingSource(username, chainId, versionSourceRollback)
	if err := m.SaveAndLoad(username, chainId, ver.Dsl); err != nil {
		m.clearPendingSource(username, chainId)
		return err
	}
	return nil
}

var _ services.RuleVersionService = (*Module)(nil)
