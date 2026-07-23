package endpoint

import (
	"sync"
	"time"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	websocketEndpoint "github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/utils/json"
)

// NewWebsocketEndpoint creates WebSocket endpoints to push debug logs in real time.
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

	// Track registered clients by clientId for cleanup when disconnected
	var clientMu sync.Mutex
	clientMap := make(map[string]*runlog.DebugDataClient)

	wsEp.SetOnEvent(func(eventName string, params ...interface{}) {
		switch eventName {
		case endpointApi.EventConnect:
			exchange := params[0].(*endpointApi.Exchange)
			username := exchange.In.Headers().Get(constants.KeyUsername)
			if username == "" {
				username = s.config.DefaultUsername
			}
			chainId := exchange.In.GetParam(constants.KeyChainId)
			clientId := exchange.In.GetParam(constants.KeyClientId)

			if chainId == "" || clientId == "" {
				return
			}

			client := &runlog.DebugDataClient{
				ChainId: chainId,
				DataCh:  make(chan map[string]interface{}, 100),
			}
			runlog.RegisterDebugClient(client)

			clientMu.Lock()
			clientMap[clientId] = client
			clientMu.Unlock()

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
				runlog.UnregisterDebugClient(client)
				close(client.DataCh)
			}
		}
	})

	// Register WebSocket routes: /api/v1/logs/ws/:chainId/:clientId
	base := s.apiBasePath()
	_, _ = wsEp.AddRouter(endpoint.NewRouter().From(base + "/logs/ws/:chainId/:clientId").
		Process(s.authProcess()).
		Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
			return true
		}).End())

	return wsEp, nil
}

// sendWsDebugLog constructs debug log data and pushes it to the WebSocket client.
func sendWsDebugLog(chainId, flowType, nodeId string, relationType string, errStr string, msg interface{}) {
	logData := map[string]interface{}{
		"chainId":      chainId,
		"flowType":     flowType,
		"nodeId":       nodeId,
		"relationType": relationType,
		"err":          errStr,
		"msg":          msg,
		"ts":           time.Now().UnixMilli(),
	}
	runlog.SendDebugDataToClients(chainId, logData)
}
