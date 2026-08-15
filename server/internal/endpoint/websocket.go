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
	var clientMu sync.Mutex
	clientMap := make(map[string]*runlog.DebugDataClient)

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
			clientMu.Lock()
			if old, ok := clientMap[clientId]; ok {
				runlog.UnregisterDebugClient(old)
				close(old.DataCh)
			}
			clientMap[clientId] = client
			clientMu.Unlock()
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
			clientMu.Lock()
			client, ok := clientMap[clientId]
			if ok {
				delete(clientMap, clientId)
			}
			clientMu.Unlock()
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
