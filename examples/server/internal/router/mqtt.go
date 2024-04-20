package router

import (
	"examples/server/config"
	"examples/server/internal/controller"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/endpoint"
	endpointMqtt "github.com/rulego/rulego/endpoint/mqtt"
	"log"
	"strings"
)

// MqttServe mqtt 订阅服务
func MqttServe(c config.Config, logger *log.Logger) {
	if !c.Mqtt.Enabled {
		return
	}
	//mqtt 订阅服务 接收端点
	mqttEndpoint, err := endpoint.New(endpointMqtt.Type, rulego.NewConfig(), c.Mqtt)
	if err != nil {
		logger.Fatal(err)
	}
	for _, topic := range strings.Split(c.Mqtt.Topics, ",") {
		if topic == "" {
			topic = "#"
		}
		router := endpoint.NewRouter(endpoint.WithRuleGoFunc(controller.GetRuleGoFunc)).From(topic).To(c.Mqtt.ToChainId).End()
		_, _ = mqttEndpoint.AddRouter(router)
	}
	if err := mqttEndpoint.Start(); err != nil {
		logger.Fatal(err)
	}
}
