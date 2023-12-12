package schedule

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/test/assert"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var testdataFolder = "../../testdata"

func TestScheduleEndPoint(t *testing.T) {

	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	//创建schedule endpoint服务
	scheduleEndpoint, err := endpoint.New(Type, config, nil)

	//每隔1秒执行
	var router1Count = int64(0)
	router1 := endpoint.NewRouter().From("*/1 * * * * *").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE1"
		atomic.AddInt64(&router1Count, 1)
		//fmt.Println(time.Now().Local().Local().String(), "router1 执行...")
		//业务逻辑，例如读取文件、定时去拉取一些数据交给规则链处理

		return true
	}). //指定交给哪个规则链ID处理
		To("chain:default").End()

	routeId1, err := scheduleEndpoint.AddRouter(router1)

	//启动任务
	err = scheduleEndpoint.Start()

	//每隔5秒执行
	var router2Count = int64(0)
	router2 := endpoint.NewRouter().From("*/5 * * * * *").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		atomic.AddInt64(&router2Count, 1)
		//fmt.Println(time.Now().Local().Local().String(), "router2 执行...")
		//业务逻辑，例如读取文件

		return true
	}).To("chain:default").End()

	//测试定时器已经启动，是否允许继续添加任务
	routeId2, err := scheduleEndpoint.AddRouter(router2)

	time.Sleep(30 * time.Second)

	//删除某个任务
	_ = scheduleEndpoint.RemoveRouter(routeId1)
	_ = scheduleEndpoint.RemoveRouter(routeId2)

	assert.Equal(t, router1Count, int64(30))
	assert.Equal(t, router2Count, int64(6))
	//fmt.Println("执行结束退出...")
}
