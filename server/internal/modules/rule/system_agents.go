package rule

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/registry"
)

// deploySystemAgents deploys the built-in agent rule chain under dataDir/system/agents when launched.
func (m *Module) deploySystemAgents() error {
	agentsDir := filepath.Join(m.cfg.DataDir, constants.DirSystemAgents)

	// When the AI component is loaded, the missing default agent is automatically created
	if registry.AiToolsProvider != nil {
		m.ensureDefaultAgents(agentsDir)
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		jsonFiles, err := filepath.Glob(filepath.Join(agentsDir, entry.Name(), "*.json"))
		if err != nil {
			continue
		}
		for _, f := range jsonFiles {
			def, err := os.ReadFile(f)
			if err != nil {
				m.logger.Warnf("[rule] read agent file %s: %v", f, err)
				continue
			}
			chainId := strings.TrimSuffix(filepath.Base(f), constants.RuleChainFileSuffix)
			def = m.markSystemAgent(def)
			if err := m.SaveAndLoad(m.cfg.DefaultUsername, chainId, def); err != nil {
				m.logger.Warnf("[rule] deploy agent %s: %v", chainId, err)
			} else {
				m.logger.Infof("[rule] deployed system agent: %s", chainId)
			}
		}
	}
	return nil
}

// ensureDefaultAgents checks the embedded default agent template and automatically creates a missing agent directory.
func (m *Module) ensureDefaultAgents(agentsDir string) {
	entries, err := defaultAgentsFS.ReadDir("template")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := entry.Name()
		targetJSON := filepath.Join(agentsDir, agentID, agentID+constants.RuleChainFileSuffix)
		if _, err := os.Stat(targetJSON); err == nil {
			continue
		}
		srcDir := "template/" + agentID
		if err := copyEmbeddedDir(defaultAgentsFS, srcDir, filepath.Join(agentsDir, agentID)); err != nil {
			m.logger.Warnf("[rule] auto-create agent %s failed: %v", agentID, err)
		} else {
			m.logger.Infof("[rule] auto-created default agent: %s", agentID)
		}
	}
}

// markSystemAgent injects the systemAgent=true tag into the rule chain JSON
func (m *Module) markSystemAgent(def []byte) []byte {
	var chain map[string]interface{}
	if err := json.Unmarshal(def, &chain); err != nil {
		return def
	}
	rc, ok := chain["ruleChain"].(map[string]interface{})
	if !ok {
		return def
	}
	info, _ := rc["additionalInfo"].(map[string]interface{})
	if info == nil {
		info = make(map[string]interface{})
	}
	info[constants.KeySystemAgent] = true
	rc["additionalInfo"] = info
	chain["ruleChain"] = rc
	b, err := json.Marshal(chain)
	if err != nil {
		return def
	}
	return b
}

// copyEmbeddedDir Reconstructively copy the directory from embed.FS to the target path.
func copyEmbeddedDir(fsys fs.FS, src, dst string) error {
	return fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// embed.FS uses a forward slash, calculates relative paths, and converts them to local paths
		rel := strings.TrimPrefix(p, src)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
