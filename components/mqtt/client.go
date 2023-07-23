package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	string2 "rulego/utils/str"

	"io/ioutil"
	"sync"
	"time"
)

//IHandler 处理订阅数据
type IHandler interface {
	//Name 处理器名称
	Name() string
	//SubscribeTopics 订阅主题列表
	SubscribeTopics() []string
	//SubscribeQos 订阅Qos
	SubscribeQos() byte
	//MsgHandler 接收订阅数据
	MsgHandler(c paho.Client, data paho.Message)
	//Close 关闭
	Close() error
}

//Config 客户端配置
type Config struct {
	//mqtt broker 地址
	Server string
	//用户名
	Username string
	//密码
	Password string
	//重连重试间隔
	MaxReconnectInterval time.Duration
	QOS                  uint8
	CleanSession         bool
	//client Id
	ClientID string
	CACert   string
	TLSCert  string
	TLSKey   string
}

//Client mqtt客户端
type Client struct {
	sync.RWMutex
	wg     sync.WaitGroup
	client paho.Client
	//订阅主题和处理器映射
	msgHandlerMap map[string]IHandler
}

// NewClient 创建一个MQTT客户端实例
func NewClient(conf Config) (*Client, error) {
	var err error

	b := Client{
		msgHandlerMap: make(map[string]IHandler),
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(conf.Server)
	opts.SetUsername(conf.Username)
	opts.SetPassword(conf.Password)
	opts.SetCleanSession(conf.CleanSession)
	if conf.ClientID == "" {
		//随机clientId
		opts.SetClientID("rulego/" + string2.RandomStr(8))
	} else {
		opts.SetClientID(conf.ClientID)
	}
	opts.SetOnConnectHandler(b.onConnected)
	opts.SetConnectionLostHandler(b.onConnectionLost)
	if conf.MaxReconnectInterval <= 0 {
		conf.MaxReconnectInterval = time.Second * 60
	}
	opts.SetMaxReconnectInterval(conf.MaxReconnectInterval)

	tlsconfig, err := newTLSConfig(conf.CACert, conf.TLSCert, conf.TLSKey)
	if err != nil {
		log.Printf("error loading mqtt certificate files,ca_cert=%s,tls_cert=%s,tls_key=%s", conf.CACert, conf.TLSCert, conf.TLSKey)
	}
	//tls
	if tlsconfig != nil {
		opts.SetTLSConfig(tlsconfig)
	}
	log.Printf("connecting to mqtt broker,server=%s", conf.Server)
	b.client = paho.NewClient(opts)
	for {
		if token := b.client.Connect(); token.Wait() && token.Error() != nil {
			log.Printf("connecting to mqtt broker failed, will retry in 2s: %s", token.Error())
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	return &b, nil
}

//RegisterHandler 注册订阅数据处理器
func (b *Client) RegisterHandler(handler IHandler) {
	b.Lock()
	defer b.Unlock()
	for _, topic := range handler.SubscribeTopics() {
		b.msgHandlerMap[topic] = handler
	}
	b.subscribe()
}

//GetHandlerByUpTopic 通过主题获取数据处理器
func (b *Client) GetHandlerByUpTopic(topic string) IHandler {
	b.RLock()
	defer b.RUnlock()
	return b.msgHandlerMap[topic]
}

func (b *Client) Close() error {
	for _, v := range b.msgHandlerMap {
		_ = v.Close()
	}
	return nil
}

//Publish 发布数据
func (b *Client) Publish(topic string, qos byte, data []byte) error {
	if token := b.client.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		return nil
	}

}

func (b *Client) onConnected(c paho.Client) {
	log.Printf("connected to mqtt server")
	b.subscribe()

}

func (b *Client) subscribe() {
	for topic, handler := range b.msgHandlerMap {
		for {
			log.Printf("subscribing to topic,topic=%s,qos=%d", topic, handler.SubscribeQos())
			if token := b.client.Subscribe(topic, handler.SubscribeQos(), handler.MsgHandler).(*paho.SubscribeToken); token.Wait() && (token.Error() != nil || is128Err(token, topic)) { //128 ACK错误
				log.Printf("subscribe error,topic=%s,qos=%d", topic, handler.SubscribeQos())
				time.Sleep(2 * time.Second)
				continue
			}
			break
		}
	}
}

//判断是否是acl 128错误
func is128Err(token *paho.SubscribeToken, topic string) bool {
	result, ok := token.Result()[topic]
	return ok && result == 128
}

func (b *Client) onConnectionLost(c paho.Client, reason error) {
	log.Printf("mqtt connection error: %s", reason)
}

func newTLSConfig(CAFile, certFile, certKeyFile string) (*tls.Config, error) {
	if CAFile == "" && certFile == "" && certKeyFile == "" {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	// Import trusted certificates from CAFile.pem.
	if CAFile != "" {
		caCert, err := ioutil.ReadFile(CAFile)
		if err != nil {
			log.Printf("could not load ca certificate")
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCert)

		tlsConfig.RootCAs = certPool // RootCAs = certs used to verify server cert.
	}

	// Import certificate and the key
	if certFile != "" && certKeyFile != "" {
		kp, err := tls.LoadX509KeyPair(certFile, certKeyFile)
		if err != nil {
			log.Printf("could not load mqtt tls key-pair")
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{kp}
	}
	return tlsConfig, nil
}
