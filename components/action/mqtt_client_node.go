package action

import (
	"rulego/api/types"
	"rulego/components/mqtt"
	"rulego/utils/maps"
	"rulego/utils/str"
	"time"
)

//规则链节点配置示例：
// {
//        "id": "s3",
//        "type": "mqttClient",
//        "name": "mqtt推送数据",
//        "debugMode": false,
//        "configuration": {
//          "Server": "127.0.0.1:1883",
//          "Topic": "/device/msg"
//        }
//      }
func init() {
	Registry.Add(&MqttClientNode{})
}

type MqttClientNodeConfiguration struct {
	//publish topic
	Topic                string
	Server               string
	Username             string
	Password             string
	MaxReconnectInterval time.Duration
	QOS                  uint8
	CleanSession         bool
	ClientID             string
	CACert               string
	TLSCert              string
	TLSKey               string
}

func (x *MqttClientNodeConfiguration) ToMqttConfig() mqtt.Config {
	return mqtt.Config{
		Server:               x.Server,
		Username:             x.Username,
		Password:             x.Password,
		QOS:                  x.QOS,
		MaxReconnectInterval: x.MaxReconnectInterval,
		CleanSession:         x.CleanSession,
		ClientID:             x.ClientID,
		CACert:               x.CACert,
		TLSCert:              x.TLSCert,
		TLSKey:               x.TLSKey,
	}
}

type MqttClientNode struct {
	config     MqttClientNodeConfiguration
	mqttClient *mqtt.Client
}

//Type 组件类型
func (x *MqttClientNode) Type() string {
	return "mqttClient"
}

func (x *MqttClientNode) New() types.Node {
	return &MqttClientNode{}
}

//Init 初始化
func (x *MqttClientNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.config)
	if err == nil {
		x.mqttClient, err = mqtt.NewClient(x.config.ToMqttConfig())
	}
	return err
}

//OnMsg 处理消息
func (x *MqttClientNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	topic := str.SprintfDict(x.config.Topic, msg.Metadata.Values())
	err := x.mqttClient.Publish(topic, x.config.QOS, []byte(msg.Data))
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		ctx.TellSuccess(msg)
	}
	return err
}

//Destroy 销毁
func (x *MqttClientNode) Destroy() {
	if x.mqttClient != nil {
		_ = x.mqttClient.Close()
	}
}
