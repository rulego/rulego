package endpoint

import (
	"testing"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/server/internal/modules/runlog"
)

// 同 clientId 重连后，旧连接迟到的断开事件不得误删新连接的登记
func TestWsClientRegistry_StaleDisconnectIgnored(t *testing.T) {
	r := newWsClientRegistry()
	exA := &endpointApi.Exchange{}
	exB := &endpointApi.Exchange{}

	a := &runlog.DebugDataClient{ChainId: "c"}
	b := &runlog.DebugDataClient{ChainId: "c"}

	if old := r.replace("client-1", a, exA); old != nil {
		t.Fatal("first replace should return nil")
	}
	if old := r.replace("client-1", b, exB); old != a {
		t.Fatal("second replace should return the first client")
	}
	// 旧连接 A 的断开先到：登记已属于 B，不能清理
	if c, ok := r.remove("client-1", exA); ok {
		t.Fatalf("stale disconnect should be ignored, got %v", c)
	}
	// 新连接 B 的断开：正常清理
	if c, ok := r.remove("client-1", exB); !ok || c != b {
		t.Fatalf("owner disconnect should remove b, got (%v,%v)", c, ok)
	}
	if _, ok := r.remove("client-1", exB); ok {
		t.Fatal("remove after cleanup should be no-op")
	}
}
