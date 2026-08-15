package runlog

import (
	"sync"
)

// 调试数据 WebSocket 客户端管理

var (
	debugClientsMu sync.RWMutex
	debugClients   = make(map[string][]*DebugDataClient)
)

// debugKey 不同用户可能存在同名链，按 username+chainId 隔离广播与存储。
func debugKey(username, chainId string) string {
	return username + "\x00" + chainId
}

// DebugDataClient 调试数据客户端
type DebugDataClient struct {
	Username string
	ChainId  string
	DataCh   chan map[string]interface{}
}

// SendDebugDataToClients 向所有监听指定规则链的客户端发送调试数据。
// 发送必须持读锁：UnregisterDebugClient 在写锁内移除后才 close(DataCh)，
// 若锁外发送，会与 close 竞态产生 send on closed channel panic。
func SendDebugDataToClients(username, chainId string, data map[string]interface{}) {
	debugClientsMu.RLock()
	defer debugClientsMu.RUnlock()
	for _, client := range debugClients[debugKey(username, chainId)] {
		select {
		case client.DataCh <- data:
		default:
		}
	}
}

// RegisterDebugClient 注册调试数据客户端
func RegisterDebugClient(client *DebugDataClient) {
	debugClientsMu.Lock()
	key := debugKey(client.Username, client.ChainId)
	debugClients[key] = append(debugClients[key], client)
	debugClientsMu.Unlock()
}

// UnregisterDebugClient 注销调试数据客户端
func UnregisterDebugClient(client *DebugDataClient) {
	debugClientsMu.Lock()
	defer debugClientsMu.Unlock()
	key := debugKey(client.Username, client.ChainId)
	if clients, ok := debugClients[key]; ok {
		for i, c := range clients {
			if c == client {
				debugClients[key] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}
}
