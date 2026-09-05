package schedule

import (
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
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
	assert.Equal(t, Type, scheduleEndpoint.Type())

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

	assert.True(t, math.Abs(float64(atomic.LoadInt64(&router1Count))-float64(15)) <= float64(1))
	assert.True(t, math.Abs(float64(atomic.LoadInt64(&router2Count))-float64(3)) <= float64(1))

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

	assert.True(t, math.Abs(float64(atomic.LoadInt64(&router1Count))-float64(15)) <= float64(1))
	assert.True(t, math.Abs(float64(atomic.LoadInt64(&router2Count))-float64(3)) <= float64(1))
	assert.True(t, math.Abs(float64(atomic.LoadInt64(&router3Count))-float64(5)) <= float64(1))
}

func TestScheduleEndPointWithParams(t *testing.T) {
	config := types.NewConfig()
	var ep = &Endpoint{}
	err := ep.Init(config, nil)
	assert.Nil(t, err)

	// Test with params
	router := impl.NewRouter().From("*/1 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// Verify message body and type
		msg := exchange.In.GetMsg()
		assert.Equal(t, "{\"id\":1}", msg.GetData())
		assert.Equal(t, types.JSON, msg.DataType)
		return true
	}).To("chain:default").End()

	_, err = ep.AddRouter(router, "{\"id\":1}", types.JSON)
	assert.Nil(t, err)

	// Simulate handler execution directly to avoid waiting for cron
	ep.handler(router)
}

func TestScheduleClusterOnce(t *testing.T) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	// 两个端点实例模拟两个副本，共享同一把锁
	config := engine.NewConfig(types.WithDefaultPool(), types.WithLocker(types.NewLocalLocker()))
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	var count int64
	newReplica := func() *Endpoint {
		ep := &Endpoint{}
		assert.Nil(t, ep.Init(config, nil))
		router := impl.NewRouter().SetId("cluster_once_r1").From("*/1 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.AddInt64(&count, 1)
			return true
		}).To("chain:default").End()
		_, err := ep.AddRouter(router)
		assert.Nil(t, err)
		assert.Nil(t, ep.Start())
		return ep
	}
	replica1 := newReplica()
	defer replica1.Destroy()
	replica2 := newReplica()
	defer replica2.Destroy()

	time.Sleep(3200 * time.Millisecond)
	// 去重生效时执行次数等于槽位数（3.2s 窗口内为 3 或 4，取决于起始相位）；
	// 未去重时两副本各跑一遍（约 6-8）
	n := atomic.LoadInt64(&count)
	assert.True(t, n >= 2 && n <= 4, fmt.Sprintf("expected 2-4 deduplicated ticks, got %d", n))
}

func TestAdvanceSlot(t *testing.T) {
	spec, err := cronParser.Parse("*/1 * * * * *")
	assert.Nil(t, err)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 计划链落后多个槽位：重锚到不晚于 now 的最近槽位
	slot, next := advanceSlot(spec, base, base.Add(3500*time.Millisecond))
	assert.Equal(t, base.Add(3*time.Second), slot)
	assert.Equal(t, base.Add(4*time.Second), next)

	// 计划链领先于 now（尚未到达）：原样返回
	slot, next = advanceSlot(spec, base.Add(5*time.Second), base)
	assert.Equal(t, base.Add(5*time.Second), slot)
	assert.Equal(t, base.Add(6*time.Second), next)
}

func TestScheduleClusterOnceDifferentRouters(t *testing.T) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool(), types.WithLocker(types.NewLocalLocker()))
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	// 不同路由 ID 的定时互不抑制：各自按槽位执行
	var count int64
	newReplica := func(routerId string) *Endpoint {
		ep := &Endpoint{}
		assert.Nil(t, ep.Init(config, nil))
		router := impl.NewRouter().SetId(routerId).From("*/1 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.AddInt64(&count, 1)
			return true
		}).To("chain:default").End()
		_, err := ep.AddRouter(router)
		assert.Nil(t, err)
		assert.Nil(t, ep.Start())
		return ep
	}
	replica1 := newReplica("router_a")
	defer replica1.Destroy()
	replica2 := newReplica("router_b")
	defer replica2.Destroy()

	time.Sleep(3200 * time.Millisecond)
	// 两条独立定时各执行一遍：约 6-8 次（每条约 3-4 拍）
	n := atomic.LoadInt64(&count)
	assert.True(t, n >= 5 && n <= 8, fmt.Sprintf("expected 5-8 ticks for two independent routers, got %d", n))
}

func TestScheduleAddRouterInvalidCronWithLocker(t *testing.T) {
	config := types.NewConfig(types.WithLocker(types.NewLocalLocker()))
	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, nil))

	_, err := ep.AddRouter(impl.NewRouter().SetId("bad").From("not-a-cron").End())
	if err == nil {
		t.Fatal("invalid cron expression should return a parse error")
	}
}

func TestScheduleClusterOnceOwnerIsolation(t *testing.T) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	// 两个租户引擎模拟两副本：共享同一把锁，同名路由，owner 不同
	shared := types.NewLocalLocker()
	configA := engine.NewConfig(types.WithDefaultPool(), types.WithLocker(shared), types.WithOwner("tenantA"))
	configB := engine.NewConfig(types.WithDefaultPool(), types.WithLocker(shared), types.WithOwner("tenantB"))
	_, _ = engine.New("default", buf, engine.WithConfig(configA))

	var count int64
	newReplica := func(config types.Config) *Endpoint {
		ep := &Endpoint{}
		assert.Nil(t, ep.Init(config, nil))
		router := impl.NewRouter().SetId("same_router").From("*/1 * * * * *").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.AddInt64(&count, 1)
			return true
		}).To("chain:default").End()
		_, err := ep.AddRouter(router)
		assert.Nil(t, err)
		assert.Nil(t, ep.Start())
		return ep
	}
	replicaA := newReplica(configA)
	defer replicaA.Destroy()
	replicaB := newReplica(configB)
	defer replicaB.Destroy()

	time.Sleep(3200 * time.Millisecond)
	// 不同 owner 的同名路由互不抑制：各租户各执行一遍（约 6-8 次）
	n := atomic.LoadInt64(&count)
	assert.True(t, n >= 5 && n <= 8, fmt.Sprintf("expected 5-8 ticks for two tenants, got %d", n))
}
