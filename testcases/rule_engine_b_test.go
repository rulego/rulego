package testcases

import (
	"rulego"
	"rulego/api/types"
	"rulego/utils/str"
	"testing"
)

func BenchmarkChainNotChangeMetadata(b *testing.B) {
	b.ResetTimer()
	config := rulego.NewConfig()
	ruleEngine, err := rulego.New(str.RandomStr(10), []byte(ruleChainFile), rulego.WithConfig(config))
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < b.N; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsg(msg)
	}
}

func BenchmarkChainChangeMetadataAndMsg(b *testing.B) {

	config := rulego.NewConfig()

	ruleEngine, err := rulego.New(str.RandomStr(10), []byte(ruleChainFile), rulego.WithConfig(config))
	if err != nil {
		b.Error(err)
	}
	//modify s1 node content
	ruleEngine.ReloadChild(types.EmptyRuleNodeId, types.RuleNodeId{Id: "s2"}, []byte(modifyMetadataAndMsgNode))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsg(msg)
	}
}

func BenchmarkCallRestApiNodeGo(b *testing.B) {
	//不使用协程池
	config := rulego.NewConfig()
	ruleEngine, _ := rulego.New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), rulego.WithConfig(config))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callRestApiNode(ruleEngine)
	}
}

func BenchmarkCallRestApiNodeWorkerPool(b *testing.B) {
	//使用协程池
	config := rulego.NewConfig(types.WithDefaultPool())
	ruleEngine, _ := rulego.New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), rulego.WithConfig(config))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callRestApiNode(ruleEngine)
	}
}

//func BenchmarkCallRestApiNodeAnts(b *testing.B) {
//	defaultAntsPool, _ := ants.NewPool(200000)
//	//使用协程池
//	config := rulego.NewConfig(types.WithPool(defaultAntsPool))
//	ruleEngine, _ := rulego.New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), rulego.WithConfig(config))
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		callRestApiNode(ruleEngine)
//	}
//}
func callRestApiNode(ruleEngine *rulego.RuleEngine) {
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"aa\":\"aaaaaaaaaaaaaa\"}")
	ruleEngine.OnMsg(msg)
}
