package endpoint

import (
	"sync"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/utils/json"
	websocketEndpoint "github.com/rulego/rulego/endpoint/websocket"
)

// NewWebsocketEndpoint 创建 WebSocket 端点，用于实时推送调试日志。
func (s *Server) NewWebsocketEndpoint(restEp endpointApi.HttpEndpoint) (endpoint.Endpoint, error) {
	wsCfg := websocketEndpoint.Config{}
	wsCfg.Server = "ref://" + restEp.Id()
	wsCfg.AllowCors = s.config.AllowCors
	wsEp, err := endpoint.Registry.New(
		websocketEndpoint.Type,
		s.systemRulegoCfg,
		wsCfg,
	)
	if err != nil {
		return nil, err
	}

	// 按 clientId 跟踪已注册的客户端，用于断开时清理
	registry := newWsClientRegistry()

	wsEp.SetOnEvent(func(eventName string, params ...interface{}) {
		switch eventName {
		case endpointApi.EventConnect:
			exchange := params[0].(*endpointApi.Exchange)
			chainId := exchange.In.GetParam(constants.KeyChainId)
			clientId := exchange.In.GetParam(constants.KeyClientId)

			if chainId == "" || clientId == "" {
				return
			}
			// EventConnect 在 upgrade 后、任何路由 Process 前触发，路由上的 authProcess
			// 管不到这里，必须自行鉴权，否则未认证连接连上即收到调试数据广播
			username := s.config.DefaultUsername
			if s.config.RequireAuth {
				userCtx, err := getAuthenticator(s.container, s.config).Authenticate(extractAuthorization(exchange))
				if err != nil {
					// 不断开会留下「看似已连接但永远收不到数据」的连接，前端无从感知
					closeWsConnection(exchange)
					return
				}
				username = userCtx.Username
			}

			client := &runlog.DebugDataClient{
				Username: username,
				ChainId:  chainId,
				DataCh:   make(chan map[string]interface{}, 100),
			}
			// 同 clientId 重连：先注销旧客户端，否则永久滞留广播列表（泄漏+死通道）
			if old := registry.replace(clientId, client, exchange); old != nil {
				runlog.UnregisterDebugClient(old)
				close(old.DataCh)
			}
			runlog.RegisterDebugClient(client)

			go func() {
				for data := range client.DataCh {
					b, err := json.Marshal(data)
					if err != nil {
						continue
					}
					exchange.Out.SetBody(b)
					if exchange.Out.GetError() != nil {
						break
					}
				}
			}()

		case endpointApi.EventDisconnect:
			exchange := params[0].(*endpointApi.Exchange)
			clientId := exchange.In.GetParam(constants.KeyClientId)
			client, ok := registry.remove(clientId, exchange)
			if ok {
				// close 必须在 Unregister 之后：发送方持读锁发送，写锁移除完成后
				// 不再有并发发送者，close 才安全
				runlog.UnregisterDebugClient(client)
				close(client.DataCh)
			}
		}
	})

	// 注册 WebSocket 路由：/api/v1/logs/ws/:chainId/:clientId
	base := s.apiBasePath()
	_, _ = wsEp.AddRouter(endpoint.NewRouter().From(base+"/logs/ws/:chainId/:clientId").
		Process(s.authProcess()).
		Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
			return true
		}).End())

	return wsEp, nil
}

// closeWsConnection 关闭 websocket 连接（鉴权失败等拒绝场景）。用结构化接口
// 断言而非具体类型：core 旧版本无 Close 方法时退化为不关（与历史行为一致），
// pin 升级后自动生效
func closeWsConnection(exchange *endpointApi.Exchange) {
	if out, ok := exchange.Out.(interface{ Close() }); ok {
		out.Close()
	}
}

// wsClientRegistry 按 clientId 跟踪调试客户端。同一连接的 Connect/Disconnect
// 事件携带同一 exchange 指针，断开清理须核对指针：同 clientId 重连后，旧连接
// 迟到的断开事件不能误删新连接
type wsClientRegistry struct {
	mu      sync.Mutex
	entries map[string]*wsClientEntry
}

type wsClientEntry struct {
	client   *runlog.DebugDataClient
	exchange *endpointApi.Exchange
}

func newWsClientRegistry() *wsClientRegistry {
	return &wsClientRegistry{entries: make(map[string]*wsClientEntry)}
}

// replace 登记新客户端并返回被顶替的旧客户端（无则 nil），调用方负责注销旧客户端
func (r *wsClientRegistry) replace(clientId string, client *runlog.DebugDataClient, exchange *endpointApi.Exchange) *runlog.DebugDataClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	var old *runlog.DebugDataClient
	if e, ok := r.entries[clientId]; ok {
		old = e.client
	}
	r.entries[clientId] = &wsClientEntry{client: client, exchange: exchange}
	return old
}

// remove 注销客户端，仅当登记的 exchange 与断开事件的一致时生效（防迟到断开误杀重连）
func (r *wsClientRegistry) remove(clientId string, exchange *endpointApi.Exchange) (*runlog.DebugDataClient, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[clientId]
	if !ok || e.exchange != exchange {
		return nil, false
	}
	delete(r.entries, clientId)
	return e.client, true
}
