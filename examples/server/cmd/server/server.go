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
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/router"
	"examples/server/internal/service"
	"flag"
	"fmt"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/node_pool"
	"gopkg.in/ini.v1"
	"log"
	//_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

const (
	version = "1.0.0"
)

var (
	//是否是查询版本
	ver bool
	//配置文件
	configFile string
)

func init() {
	flag.StringVar(&configFile, "c", "", "配置文件")
	flag.BoolVar(&ver, "v", false, "打印版本")
}

func main() {
	//go http.ListenAndServe("0.0.0.0:6060", nil)
	flag.Parse()

	if ver {
		fmt.Printf("RuleGo-Ci Server v%s", version)
		os.Exit(0)
	}

	var c config.Config
	if configFile == "" {
		c = config.DefaultConfig
	} else if cfg, err := ini.Load(configFile); err != nil {
		log.Fatal("error:", err)
	} else {
		if err := cfg.MapTo(&c); err != nil {
			log.Fatal("error:", err)
		}
		if section, err := cfg.GetSection("global"); err == nil {
			c.Global = section.KeysHash()
		}
	}
	config.Set(c)
	logger.Set(initLogger(c))

	log.Printf("use config file=%s \n", configFile)

	if err := loadNodePool(c.NodePoolFile); err != nil {
		log.Fatal("loadNodePool error:", err)
	} else {
		log.Printf("loadNodePool file=%s \n", c.NodePoolFile)
	}

	//初始化服务
	if err := service.Setup(c); err != nil {
		log.Fatal("setup service error:", err)
	}
	var mqttEndpoint endpointApi.Endpoint
	//创建mqtt接入服务
	if c.Mqtt.Enabled {
		mqttEndpoint, _ = router.MqttServe(c, logger.Logger)
	}
	//创建rest服务
	restEndpoint := router.NewRestServe(c)
	restEndpoint.OnEvent = func(eventName string, params ...interface{}) {
		if eventName == endpointApi.EventInitServer {
			wsEndpoint := router.NewWebsocketServe(c, params[0].(*rest.Rest))
			if err := wsEndpoint.Start(); err != nil {
				log.Fatal("error:", err)
			}
		}
	}
	//启动服务
	if err := restEndpoint.Start(); err != nil {
		log.Fatal("error:", err)
	}

	sigs := make(chan os.Signal, 1)
	// 监听系统信号，包括中断信号和终止信号
	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
		if restEndpoint != nil {
			restEndpoint.Destroy()
		}
		if mqttEndpoint != nil {
			mqttEndpoint.Destroy()
		}
		log.Println("stopped server")
		os.Exit(0)
	}
}

// 初始化日志记录器
func initLogger(c config.Config) *log.Logger {
	if c.LogFile == "" {
		return log.New(os.Stdout, "", log.LstdFlags)
	} else {
		f, err := os.OpenFile(c.LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
		return log.New(f, "", log.LstdFlags)
	}
}

func loadNodePool(file string) error {
	if file != "" {
		if buf, err := os.ReadFile(file); err != nil {
			return err
		} else {
			_, err = node_pool.DefaultNodePool.Load(buf)
			return err
		}
	}
	return nil
}
