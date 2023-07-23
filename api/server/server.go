package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"rulego"
	"rulego/api/types"
	"rulego/components/mqtt"
	string2 "rulego/utils/str"
	//_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion + "/"

	// path to rule. /msg/{tenant_id}/{msg_type}
	msgPath = apiBasePath + "msg/"
	// /rule/{tenant_id}/{rule_id}
	rulePath = apiBasePath + "rule/"

	// server version.
	version = "1.0.0"
)

var (
	httpAvailable bool
	port          int
	logfile       string
	ver           bool
	rulefile      string
	ruleEngine    *rulego.RuleEngine

	subscribeTopics  string
	mqttClientConfig = mqtt.Config{}
	mqttClient       *Mqtt
	mqttAvailable    bool
)

func init() {
	flag.BoolVar(&mqttAvailable, "mqtt", true, "mqtt client aviliable .")
	flag.StringVar(&mqttClientConfig.Server, "server", "127.0.0.1:1883", "Server of the mqtt broker.")
	flag.StringVar(&mqttClientConfig.Username, "username", "", "username of the mqtt client.")
	flag.StringVar(&mqttClientConfig.Password, "password", "", "Password of the mqtt client.")
	flag.DurationVar(&mqttClientConfig.MaxReconnectInterval, "maxReconnectInterval", 100000*100000*60, "MaxReconnectInterval of reconnect the mqtt broker.")
	flag.BoolVar(&mqttClientConfig.CleanSession, "cleansession", false, "cleanSession.")
	flag.StringVar(&mqttClientConfig.ClientID, "clientid", "", "clientID of the client.")
	flag.StringVar(&mqttClientConfig.CACert, "cacert", "", "CACert of the client.")
	flag.StringVar(&mqttClientConfig.TLSCert, "tlscert", "", "TLSCert of the client.")
	flag.StringVar(&mqttClientConfig.TLSKey, "tlskey", "", "TLSKey of the client.")
	flag.StringVar(&subscribeTopics, "topics", "#", "subscribe the topics .")

	flag.StringVar(&rulefile, "rulefile", "", "Location of the rulefile.")

	flag.IntVar(&port, "port", 9090, "The port to listen on.")
	flag.StringVar(&logfile, "logfile", "", "Location of the logfile.")
	flag.BoolVar(&ver, "version", false, "Print server version.")

}

func main() {

	flag.Parse()

	if ver {
		fmt.Printf("RuleGo Server v%s", version)
		os.Exit(0)
	}

	var logger *log.Logger

	if logfile == "" {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		logger = log.New(f, "", log.LstdFlags)
	}

	if rulefile == "" {
		fmt.Println("not the root rule file,set the flag of rulefile")
		os.Exit(0)
	} else {
		buf, err := os.ReadFile(rulefile)
		if err != nil {
			logger.Fatal("parser rule file error:", err)
		}
		config := rulego.NewConfig(types.WithDefaultPool())
		//调试模式回调信息
		//debugMode=true 的节点会打印
		config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		}
		ruleEngine, err = rulego.New(string2.RandomStr(10), buf, rulego.WithConfig(config))
		if err != nil {
			logger.Fatal("parser rule file error:", err)
		}
	}

	if mqttClientConfig.Server != "" {
		mqttClient = &Mqtt{logger: logger, config: mqttClientConfig, ruleEngine: ruleEngine, subscribeTopics: strings.Split(subscribeTopics, ",")}
		if err := mqttClient.Start(); err != nil {
			logger.Fatal(err)
		}
	}

	logger.Print("server initialised.")

	http.Handle(msgPath, msgIndexHandler(ruleEngine))
	http.Handle(rulePath, ruleIndexHandler(ruleEngine))

	logger.Printf("starting server on :%d", port)

	strPort := ":" + strconv.Itoa(port)
	log.Fatal("ListenAndServe: ", http.ListenAndServe(strPort, nil))

}
