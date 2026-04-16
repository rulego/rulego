package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test/assert"
)

func TestLCAComplexBug(t *testing.T) {
	// 读取规则链文件
	ruleChainFile := filepath.Join("..", "testdata", "rule", "test_lca_complex_bug.json")
	buf, err := os.ReadFile(ruleChainFile)
	if err != nil {
		t.Skip("Skip test because rule chain file not found:", err)
		return
	}

	config := engine.NewConfig(types.WithDefaultPool())
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if err != nil {
			t.Logf("Node error! flowType=%s, nodeId=%s, relationType=%s, err=%v", flowType, nodeId, relationType, err)
		}
	}

	ruleEngine, err := engine.New("test_lca_complex_bug", buf, engine.WithConfig(config))
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	defer ruleEngine.Stop(context.Background())

	runTest := func(runIndex int) {
		metaData := types.NewMetadata()
		msgData := `{}`

		msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, msgData)

		var wg sync.WaitGroup
		wg.Add(1)

		var joinNodeLog *types.RuleNodeRunLog
		var endNodeLog *types.RuleNodeRunLog

		ruleEngine.OnMsg(msg,
			types.WithContext(context.Background()),
			types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
				if nodeRunLog.Id == "node_86" { // join node
					joinNodeLog = &nodeRunLog
				}
				if nodeRunLog.Id == "node_122" { // end node
					endNodeLog = &nodeRunLog
				}
			}),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				wg.Done()
			}),
		)

		// 等待执行完成或超时
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Logf("Run %d: rule chain finished", runIndex)
		case <-time.After(5 * time.Second):
			t.Fatalf("Run %d: rule chain timeout!", runIndex)
		}

		if joinNodeLog != nil {
			metadataStr := joinNodeLog.OutMsg.Metadata.Values()
			_, hasLevelInfo := metadataStr["level_info"]
			t.Logf("Run %d: node_86 join node metadata has level_info: %v", runIndex, hasLevelInfo)
			if !hasLevelInfo {
				t.Errorf("Run %d: level_info missing from join node metadata!", runIndex)
			}
		} else {
			t.Errorf("Run %d: node_86 join node did not run!", runIndex)
		}

		if endNodeLog == nil {
			t.Errorf("Run %d: end node node_122 did not run!", runIndex)
		}
	}

	// 跑2次以复现问题
	for i := 1; i <= 2; i++ {
		t.Logf("--- Starting Run %d ---", i)
		runTest(i)
	}
}
