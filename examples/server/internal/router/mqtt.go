package router

//// MqttServe mqtt 订阅服务
//func MqttServe(c config.Config, logger *log.Logger) (endpoint.Endpoint, error) {
//	if !c.Mqtt.Enabled {
//		return nil, nil
//	}
//	//mqtt 订阅服务 接收端点
//	mqttEndpoint, err := endpoint.Registry.New(endpointMqtt.Type, rulego.NewConfig(), c.Mqtt)
//	if err != nil {
//		logger.Fatal(err)
//	}
//	for _, topic := range strings.Split(c.Mqtt.Topics, ",") {
//		if topic == "" {
//			topic = "#"
//		}
//		router := endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(controller.GetRuleGoFunc)).From(topic).To(c.Mqtt.ToChainId).End()
//		_, _ = mqttEndpoint.AddRouter(router)
//	}
//	if err := mqttEndpoint.Start(); err != nil {
//		logger.Fatal(err)
//	}
//	return mqttEndpoint, err
//}
