package endpoint

import (
	"sync/atomic"
	"testing"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/test/assert"
)

var deferredStubAddRouterCount int64

type deferredStubEndpoint struct {
	impl.BaseEndpoint
}

func (e *deferredStubEndpoint) Type() string { return "deferredStub" }
func (e *deferredStubEndpoint) Id() string   { return "ep_deferred_reload" }
func (e *deferredStubEndpoint) New() types.Node {
	return &deferredStubEndpoint{}
}
func (e *deferredStubEndpoint) Init(_ types.Config, _ types.Configuration) error { return nil }
func (e *deferredStubEndpoint) AddRouter(_ endpointApi.Router, _ ...interface{}) (string, error) {
	atomic.AddInt64(&deferredStubAddRouterCount, 1)
	return "1", nil
}
func (e *deferredStubEndpoint) RemoveRouter(_ string, _ ...interface{}) error { return nil }
func (e *deferredStubEndpoint) Start() error                                  { return nil }

// 链上部署的 endpoint 首次由部署方 ApplyRouters 挂路由；之后走配置变更重启型
// 重载时必须自行把路由挂回去，否则端点以零路由重启。
func TestApplyRoutersThenRestartReloadKeepsRouters(t *testing.T) {
	_ = Registry.Register(&deferredStubEndpoint{})

	def := types.EndpointDsl{
		RuleNode: types.RuleNode{
			Id:            "ep_deferred_reload",
			Type:          "deferredStub",
			Configuration: types.Configuration{"k": "v1"},
		},
		Routers: []*types.RouterDsl{
			{Id: "r1", From: types.FromDsl{Path: "/t"}, To: types.ToDsl{Path: "chainX"}},
		},
	}
	ep, err := NewPool().Factory().NewFromDef(def, endpointApi.DynamicEndpointOptions.WithDeferredRouters(true))
	assert.Nil(t, err)

	assert.Nil(t, ep.ApplyRouters())
	before := atomic.LoadInt64(&deferredStubAddRouterCount)
	assert.Equal(t, int64(1), before)

	// 配置变更触发重启型重载，路由应随新实例重建
	def2 := def
	def2.Configuration = types.Configuration{"k": "v2"}
	assert.Nil(t, ep.ReloadFromDef(def2))
	after := atomic.LoadInt64(&deferredStubAddRouterCount)
	assert.Equal(t, before+1, after)

	ep.Destroy()
}
