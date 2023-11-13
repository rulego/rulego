package main

import (
	"context"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	string2 "github.com/rulego/rulego/utils/str"
	"time"
)

var (
	shareKey   = "shareKey"
	shareValue = "shareValue"
)

//测试插件，需要在linux运行
//先编译plugin.go
//go build -buildmode=plugin -o plugin.so plugin.go
func main() {
	_ = rulego.Registry.Unregister("test")
	//注册插件组件
	err := rulego.Registry.RegisterPlugin("test", "./plugin.so")
	if err != nil {
		panic(err)
	}
	config := rulego.NewConfig()

	ruleEngine, err := rulego.New(string2.RandomStr(10), []byte(chainJsonFile), rulego.WithConfig(config))
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")

	ruleEngine.OnMsgWithOptions(msg, types.WithContext(context.WithValue(context.Background(), shareKey, shareValue)), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println(msg)
	}))

	time.Sleep(time.Second * 1)
}

var chainJsonFile = `
	{
	  "ruleChain": {
		"name": "测试规则链",
		"root": true
	  },
	  "metadata": {
		"nodes": [
		  {
			"id":"s1",
			"type": "test/upper",
			"name": "转大写",
			"debugMode": true
		  },
		  {
			"id":"s2",
			"type": "test/time",
			"name": "增加时间",
			"debugMode": true
		  }
		],
		"connections": [
		  {
			"fromId": "s1",
			"toId": "s2",
			"type": "Success"
		  }
		]
	  }
	}
`
