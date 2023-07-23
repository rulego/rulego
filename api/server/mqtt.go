package main

import (
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"rulego"
	"rulego/api/types"
	"rulego/components/mqtt"
)

type Mqtt struct {
	logger     *log.Logger
	config     mqtt.Config
	mqttClient *mqtt.Client
	ruleEngine *rulego.RuleEngine

	subscribeTopics []string
}

func (x *Mqtt) Start() error {
	var err error
	x.mqttClient, err = mqtt.NewClient(x.config)
	if err == nil {
		x.mqttClient.RegisterHandler(x)
	}
	return err
}

func (x *Mqtt) Stop() {
	_ = x.Close()
}

func (x *Mqtt) Name() string {
	return "backend"
}
func (x *Mqtt) SubscribeTopics() []string {
	return x.subscribeTopics
}
func (x *Mqtt) SubscribeQos() byte {
	return 0
}
func (x *Mqtt) MsgHandler(c paho.Client, data paho.Message) {
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("topic", data.Topic())
	msg := types.NewMsg(0, data.Topic(), types.JSON, metaData, string(data.Payload()))

	x.ruleEngine.OnMsg(msg)
}

func (x *Mqtt) Close() error {
	if nil != x.mqttClient {
		return x.mqttClient.Close()
	}
	return nil
}
