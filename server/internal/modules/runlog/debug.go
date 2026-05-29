package runlog

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/utils/fs"
)

// HandleDebugLog 处理调试日志数据
func (m *Module) HandleDebugLog(username, chainId string, data map[string]interface{}) {
	if ruleChainId, ok := data["ruleChainId"]; ok {
		chainId = fmt.Sprintf("%v", ruleChainId)
	}
	if value, ok := data["data"]; ok {
		if dataMap, ok := value.(map[string]interface{}); ok {
			m.SaveDebugData(username, chainId, dataMap)
		}
	}
}

// SaveDebugData 保存调试数据到文件
func (m *Module) SaveDebugData(username, chainId string, data map[string]interface{}) {
	_ = saveDebugDataToPath(m.cfg, username, data, chainId)
}

func saveDebugDataToPath(cfg *config.Config, username string, data map[string]interface{}, chainId string) error {
	var pathStr = []string{cfg.DataDir, constants.DirWorkflows}
	pathStr = append(pathStr, username, constants.DirWorkflowsRun, chainId)
	_ = fs.CreateDirs(path.Join(pathStr...))
	now := time.Now()
	data["timestamp"] = now.Format("2006/01/02 15:04:05.000")
	if byteV, err := json.Marshal(data); err == nil {
		fileName := now.Format("20060102150405.000")
		return fs.SaveFile(filepath.Join(path.Join(pathStr...), fileName), byteV)
	}
	return nil
}

// 调试数据 WebSocket 客户端管理

var (
	debugClientsMu sync.RWMutex
	debugClients   = make(map[string][]*DebugDataClient)
)

// DebugDataClient 调试数据客户端
type DebugDataClient struct {
	ChainId string
	DataCh  chan map[string]interface{}
}

// SendDebugDataToClients 向所有监听指定规则链的客户端发送调试数据
func SendDebugDataToClients(chainId string, data map[string]interface{}) {
	debugClientsMu.RLock()
	clients := debugClients[chainId]
	debugClientsMu.RUnlock()
	for _, client := range clients {
		select {
		case client.DataCh <- data:
		default:
		}
	}
}

// RegisterDebugClient 注册调试数据客户端
func RegisterDebugClient(client *DebugDataClient) {
	debugClientsMu.Lock()
	debugClients[client.ChainId] = append(debugClients[client.ChainId], client)
	debugClientsMu.Unlock()
}

// UnregisterDebugClient 注销调试数据客户端
func UnregisterDebugClient(client *DebugDataClient) {
	debugClientsMu.Lock()
	defer debugClientsMu.Unlock()
	if clients, ok := debugClients[client.ChainId]; ok {
		for i, c := range clients {
			if c == client {
				debugClients[client.ChainId] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}
}
