package main

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	string2 "github.com/rulego/rulego/utils/str"
	"sync"
	"testing"
)

var testPluginRuleFile = `
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

func TestPlugin(t *testing.T) {
	_ = rulego.Registry.Unregister("test")
	err := rulego.Registry.RegisterPlugin("test", "./plugin.so")
	if err != nil {
		t.Fatal(err)
	}
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes)
	config := rulego.NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Data, msg.Metadata, relationType, err)
	}

	ruleEngine, err := rulego.New(string2.RandomStr(10), []byte(testPluginRuleFile), rulego.WithConfig(config))
	defer ruleEngine.Stop()
	for i := 0; i < maxTimes; i++ {
		if err == nil {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")
			//time.Sleep(time.Millisecond * 50)
			ruleEngine.OnMsgWithOptions(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
				assert.Equal(t, "AA", msg.Data)
				v := msg.Metadata.GetValue("timestamp")
				assert.True(t, v != "")
				group.Done()
				config.Logger.Printf("OnEnd data=%s,metaData=%s,err=%s", msg.Data, msg.Metadata, err)
			}))
		}
	}
	group.Wait()
}

func TestReloadPlugin(t *testing.T) {
	_ = rulego.Registry.Unregister("test")
	err := rulego.Registry.RegisterPlugin("test", "./plugin.so")
	if err != nil {
		t.Fatal(err)
	}
	err = rulego.Registry.RegisterPlugin("test", "./plugin.so")
	assert.NotNil(t, err)

	err = rulego.Registry.Unregister("test")

	err = rulego.Registry.RegisterPlugin("test", "./plugin.so")
	assert.Nil(t, err)
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes)
	config := rulego.NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Data, msg.Metadata, relationType, err)
	}

	ruleEngine, err := rulego.New(string2.RandomStr(10), []byte(testPluginRuleFile), rulego.WithConfig(config))
	defer ruleEngine.Stop()
	for i := 0; i < maxTimes; i++ {
		if err == nil {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")
			//time.Sleep(time.Millisecond * 50)
			ruleEngine.OnMsgWithEndFunc(msg, func(ctx types.RuleContext, msg types.RuleMsg, err error) {
				assert.Equal(t, "AA", msg.Data)
				v := msg.Metadata.GetValue("timestamp")
				assert.True(t, v != "")
				group.Done()
				config.Logger.Printf("OnEnd data=%s,metaData=%s,err=%s", msg.Data, msg.Metadata, err)
			})

		}
	}
	group.Wait()
}
