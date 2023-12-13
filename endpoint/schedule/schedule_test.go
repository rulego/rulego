package schedule

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"math"
	"net/textproto"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var testdataFolder = "../../testdata"

// 测试请求/响应消息
func TestMessage(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		var request = &RequestMessage{headers: make(textproto.MIMEHeader)}
		test.EndpointMessage(t, request)
	})
	t.Run("Response", func(t *testing.T) {
		var response = &ResponseMessage{headers: make(textproto.MIMEHeader)}
		test.EndpointMessage(t, response)
	})
}

func TestScheduleEndPoint(t *testing.T) {

	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	schedule := New(config)

	schedule = &Schedule{RuleConfig: config}
	err = schedule.Start()
	assert.Equal(t, "cron has not been initialized yet", err.Error())

	// nil from
	_, _ = schedule.AddRouter(endpoint.NewRouter())

	_, _ = schedule.AddRouter(endpoint.NewRouter().From("*/1 * * * * *").End())

	schedule.Printf("run %s", "schedule")

	assert.Equal(t, schedule.id, schedule.Id())

	schedule.Destroy()
	schedule.Close()

	//创建schedule endpoint服务
	scheduleEndpoint, err := endpoint.New(Type, config, nil)

	_, err = scheduleEndpoint.AddRouter(nil)
	assert.Equal(t, "router can not nil", err.Error())
	err = scheduleEndpoint.RemoveRouter("aa")
	assert.Equal(t, "aa it is an illegal routing id", err.Error())

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

	assert.True(t, math.Abs(float64(router1Count)-float64(30)) <= float64(1))
	assert.True(t, math.Abs(float64(router2Count)-float64(6)) <= float64(1))

	scheduleEndpoint.Destroy()
	//fmt.Println("执行结束退出...")
}
