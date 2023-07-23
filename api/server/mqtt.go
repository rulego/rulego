/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"log"
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
