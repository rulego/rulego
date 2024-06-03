package schedule

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var testdataFolder = "../../testdata/rule"

// 测试请求/响应消息
func TestMessage(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		var request = &RequestMessage{}
		test.EndpointMessage(t, request)
	})
	t.Run("Response", func(t *testing.T) {
		var response = &ResponseMessage{}
		test.EndpointMessage(t, response)
	})
}

func TestRouterId(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	var ep = &Endpoint{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)
	router := impl.NewRouter().SetId("r1").From("*/1 * * * * *").End()
	routerId, _ := ep.AddRouter(router)
	assert.Equal(t, "1", routerId)

	router = impl.NewRouter().SetId("r1").From("*/1 * * * * *").End()
	routerId, _ = ep.AddRouter(router)
	assert.Equal(t, "2", routerId)

	err = ep.RemoveRouter("1")
	assert.Nil(t, err)

	err = ep.RemoveRouter("2")
	assert.Nil(t, err)
}

func TestScheduleEndPoint(t *testing.T) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	schedule := New(config)

	schedule = &Schedule{RuleConfig: config}
	err = schedule.Start()
	assert.Equal(t, "cron has not been initialized yet", err.Error())

	// nil from
	_, _ = schedule.AddRouter(impl.NewRouter())

	_, _ = schedule.AddRouter(impl.NewRouter().From("*/1 * * * * *").End())

	schedule.Printf("run %s", "schedule")

	assert.Equal(t, schedule.id, schedule.Id())

	schedule.Destroy()
	schedule.Close()

	var scheduleEndpoint = &Endpoint{}
	err = scheduleEndpoint.Init(config, nil)
	assert.Nil(t, err)
	assert.Equal(t, "schedule", scheduleEndpoint.Type())

	////创建schedule endpoint服务
	//scheduleEndpoint, err := registry.New(Type, config, nil)

	_, err = scheduleEndpoint.AddRouter(nil)
	assert.Equal(t, "router can not nil", err.Error())
	err = scheduleEndpoint.RemoveRouter("aa")
	assert.Equal(t, "aa it is an illegal routing id", err.Error())

	//每隔1秒执行
	var router1Count = int64(0)
	router1 := impl.NewRouter().From("*/1 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
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
	router2 := impl.NewRouter().From("*/5 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		atomic.AddInt64(&router2Count, 1)
		//fmt.Println(time.Now().Local().Local().String(), "router2 执行...")
		//业务逻辑，例如读取文件

		return true
	}).To("chain:default").End()

	//测试定时器已经启动，是否允许继续添加任务
	routeId2, err := scheduleEndpoint.AddRouter(router2)

	time.Sleep(15 * time.Second)

	assert.True(t, math.Abs(float64(router1Count)-float64(15)) <= float64(1))
	assert.True(t, math.Abs(float64(router2Count)-float64(3)) <= float64(1))

	//删除某个任务
	_ = scheduleEndpoint.RemoveRouter(routeId1)
	_ = scheduleEndpoint.RemoveRouter(routeId2)

	scheduleEndpoint.Destroy()

	var router3Count = int64(0)
	//restart
	router3 := impl.NewRouter().From("*/3 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		atomic.AddInt64(&router3Count, 1)
		//fmt.Println(time.Now().Local().Local().String(), "router3 执行...")
		//业务逻辑，例如读取文件

		return true
	}).To("chain:default").End()

	_, err = scheduleEndpoint.AddRouter(router3)
	err = scheduleEndpoint.Start()

	assert.Nil(t, err)
	time.Sleep(15 * time.Second)
	scheduleEndpoint.Destroy()

	assert.True(t, math.Abs(float64(router1Count)-float64(15)) <= float64(1))
	assert.True(t, math.Abs(float64(router2Count)-float64(3)) <= float64(1))
	assert.True(t, math.Abs(float64(router3Count)-float64(5)) <= float64(1))
}
