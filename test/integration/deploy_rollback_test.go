package integration

import (
	"sync/atomic"
	"testing"

	"github.com/rulego/rulego"
)

// TestFailedDeployDestroysCreatedEndpoints verifies a failed chain deploy
// releases the endpoint instances created before the failure: two endpoints,
// the second cannot connect, the first must not leak its connection.
func TestFailedDeployDestroysCreatedEndpoints(t *testing.T) {
	before := atomic.LoadInt64(&testConnLiveCount)
	chain := `{
	  "ruleChain": {"id": "test_ep_deploy_rollback", "root": true},
	  "metadata": {
	    "endpoints": [
	      {"id": "ep_ok", "type": "endpoint/testConn", "configuration": {"server": "rollbackA"}},
	      {"id": "ep_bad", "type": "endpoint/testConn", "configuration": {"server": ""}}
	    ],
	    "nodes": []
	  }
	}`
	if _, err := rulego.New("test_ep_deploy_rollback", []byte(chain)); err == nil {
		rulego.Del("test_ep_deploy_rollback")
		t.Fatal("expected deploy failure for ep_bad")
	}
	if live := atomic.LoadInt64(&testConnLiveCount) - before; live != 0 {
		t.Fatalf("leaked %d connection(s) from failed deploy", live)
	}
}
