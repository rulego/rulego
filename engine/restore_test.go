package engine_test

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	_ "github.com/rulego/rulego/components/common"
	_ "github.com/rulego/rulego/components/transform"
	"github.com/rulego/rulego/engine"
)

// TestRestoreFromMultipleBranches verifies that we can restore execution from multiple branches
// (C1 and C2) that are children of a Fork node (A), and successfully merge at a Join node (D).
func TestRestoreFromMultipleBranches(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())

	// Read rule chain from file
	buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := engine.New("complex_restore_chain", buf, engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")

	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lastMsg types.RuleMsg
	var lock sync.Mutex

	// Use WithRestoreNodes to restore from C1 and C2, specifying A as the common parent (Fork node).
	// This simulates that A has spawned C1 and C2, and now we are resuming them.
	// The engine will create a parent context for A, and child contexts for C1 and C2.
	// When C1 and C2 complete, A will be marked as executed (via childDone),
	// satisfying D's Join condition (which waits for A's completion).
	ruleEngine.OnMsg(msg,
		types.WithRestoreNodes(types.ExecuteNode("node_c1"), types.ExecuteNode("node_c2")),
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			executedNodes[nodeRunLog.Id] = true
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if lastMsg.GetMetadata() == nil {
			t.Error("metadata is nil")
		} else if lastMsg.GetMetadata().GetValue("f") != "executed" {
			t.Error("node_f was not executed")
		}
		// Verify execution path
		if !executedNodes["node_c1"] {
			t.Error("node_c1 was not executed")
		}
		if !executedNodes["node_c2"] {
			t.Error("node_c2 was not executed")
		}
		if !executedNodes["node_d"] {
			t.Error("node_d was not executed")
		}
		if !executedNodes["node_e"] {
			t.Error("node_e was not executed")
		}
		if !executedNodes["node_f"] {
			t.Error("node_f was not executed")
		}

		// node_b1 and node_b2 should NOT be executed because we restored directly from C1/C2
		if executedNodes["node_b1"] {
			t.Error("node_b1 should not be executed")
		}
		if executedNodes["node_b2"] {
			t.Error("node_b2 should not be executed")
		}

	case <-time.After(time.Second * 6):
		t.Log("Executed nodes:", executedNodes)
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestRestoreFromMultipleBranchesAutoParent verifies automatic parent detection.
func TestRestoreFromMultipleBranchesAutoParent(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())

	buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := engine.New("complex_restore_chain_auto", buf, engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{\"from\":\"inputMsg\"}")
	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lastMsg types.RuleMsg
	var lock sync.Mutex

	// Omit parentNodeId ("") to test auto detection via WithStartNode
	ruleEngine.OnMsg(msg,
		types.WithStartNode("node_c1", "node_c2"),
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			executedNodes[nodeRunLog.Id] = true
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if lastMsg.GetMetadata() == nil {
			t.Error("metadata is nil")
		} else if lastMsg.GetMetadata().GetValue("f") != "executed" {
			t.Error("node_f was not executed")
		}
		if !executedNodes["node_d"] {
			t.Error("node_d was not executed")
		}
	case <-time.After(time.Second * 6):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestStartNode verifies the basic functionality of WithStartNode (single node start).
// It tests starting from a middle node in a simple chain.
func TestStartNode(t *testing.T) {
	// Re-use the complex chain but we will just test a linear path segment if possible,
	// or we can test starting from 'node_e' which is the end node.

	config := engine.NewConfig(types.WithDefaultPool())
	buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := engine.New("complex_restore_chain_start", buf, engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{\"from\":\"inputMsg\"}")
	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lastMsg types.RuleMsg
	var lock sync.Mutex
	// Start from node_e (End Node).
	// node_d should NOT be executed.
	// node_f SHOULD be executed (it's after node_e).
	ruleEngine.OnMsg(msg,
		types.WithStartNode("node_e"),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			lock.Lock()
			defer lock.Unlock()
			for _, log := range snapshot.Logs {
				executedNodes[log.Id] = true
			}
			close(done)
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if lastMsg.GetMetadata().GetValue("f") != "executed" {
			t.Error("node_f was not executed")
		}
		if lastMsg.GetData() != "{\"from\":\"inputMsg\"}" {
			t.Error("Unexpected message data")
		}
		if !executedNodes["node_e"] {
			t.Error("node_e was not executed")
		}
		if !executedNodes["node_f"] {
			t.Error("node_f was not executed")
		}
		if executedNodes["node_d"] {
			t.Error("node_d should not be executed")
		}
	case <-time.After(time.Second * 2):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestRestoreForkNoJoin verifies restoration from multiple branches without a subsequent join node.
// It ensures that independent branches can be restored and execute to completion.
func TestRestoreForkNoJoin(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())

	buf, err := os.ReadFile("../testdata/rule/test_restore_fork_no_join.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := engine.New("fork_no_join_chain", buf, engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{\"from\":\"inputMsg\"}")
	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lock sync.Mutex
	var count int32
	// Restore from B1 and B2 (children of A).
	// They should execute independently and then trigger C1 and C2 respectively.
	ruleEngine.OnMsg(msg,
		types.WithRestoreNodes(types.ExecuteNode("node_b1"), types.ExecuteNode("node_b2")),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			for _, log := range snapshot.Logs {
				executedNodes[log.Id] = true
			}
			close(done)
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&count, 1)
		}),
	)
	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if atomic.LoadInt32(&count) != 2 {
			t.Error("Unexpected number of end callbacks")
		}
		if !executedNodes["node_b1"] {
			t.Error("node_b1 was not executed")
		}
		if !executedNodes["node_c1"] {
			t.Error("node_c1 was not executed")
		}
		if !executedNodes["node_b2"] {
			t.Error("node_b2 was not executed")
		}
		if !executedNodes["node_c2"] {
			t.Error("node_c2 was not executed")
		}
	case <-time.After(time.Second * 6):
		t.Fatal("Timeout waiting for execution to complete")
	}

}

func TestRestoreWithOptions(t *testing.T) {
	executeNextFunc := func(t *testing.T, useWithTellNext bool) {
		config := engine.NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := engine.New("complex_restore_chain_next", buf, engine.WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")
		done := make(chan struct{})
		var executedNodes = make(map[string]bool)
		// Start from node_a's children via "True" and "False" relations.
		// node_a connects to node_b1 (Success) and node_b2 (Success).
		// So ExecuteNext("node_a", "Success") should trigger both b1 and b2.
		var opts []types.RuleContextOption
		opts = append(opts, types.WithRestoreNodes(
			types.ExecuteNext("node_a", "Success"),
		))
		if useWithTellNext {
			opts = append(opts, types.WithTellNext("node_a", "Success"))
		} else {
			opts = append(opts, types.WithRestoreNodes(
				types.ExecuteNext("node_a", "Success"),
			))
		}
		opts = append(opts, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			for _, log := range snapshot.Logs {
				executedNodes[log.Id] = true
				// node_a 只有输出没有输入
				if log.Id == "node_a" {
					if log.InMsg.Id != "" {
						t.Error("node_a should not have an input message")
					}
					if log.OutMsg.Id == "" {
						t.Error("node_a should have an output message")
					}
				}
			}
			close(done)
		}))
		ruleEngine.OnMsg(msg, opts...)

		select {
		case <-done:
			if !executedNodes["node_b1"] {
				t.Error("node_b1 was not executed")
			}
			if !executedNodes["node_b2"] {
				t.Error("node_b2 was not executed")
			}
			if !executedNodes["node_c1"] {
				t.Error("node_c1 was not executed")
			}
			if !executedNodes["node_d"] {
				t.Error("node_d was not executed")
			}
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for ExecuteNext test")
		}
	}
	// Test restore from child nodes (ExecuteNext)
	t.Run("ExecuteNext", func(t *testing.T) {
		executeNextFunc(t, false)
	})
	t.Run("TellNext", func(t *testing.T) {
		executeNextFunc(t, true)
	})
	// Test restore from disjoint branches that don't merge immediately but share common ancestor
	t.Run("DisjointBranches", func(t *testing.T) {
		// Using fork_no_join_chain
		config := engine.NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_fork_no_join.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := engine.New("fork_no_join_chain_disjoint", buf, engine.WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")
		done := make(chan struct{})
		var executedNodes = make(map[string]bool)

		// Restore B1 and B2. Common ancestor is A.
		ruleEngine.OnMsg(msg,
			types.WithRestoreNodes(types.ExecuteNode("node_b1"), types.ExecuteNode("node_b2")),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				for _, log := range snapshot.Logs {
					executedNodes[log.Id] = true
				}
				close(done)
			}),
		)
		select {
		case <-done:
			if !executedNodes["node_b1"] || !executedNodes["node_b2"] {
				t.Error("b1/b2 not executed")
			}
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout disjoint")
		}
	})

	// TestRestoreFromPartialBranches verifies restoration from a single branch (C1)
	// when other branches (C2) are NOT restored.
	// In this case, Join node (D) will trigger but will wait for C2's input.
	// Since C2 is not restored, D will eventually TIMEOUT and fail.
	// This test confirms that to successfully complete a Join, you must restore ALL branches
	// that contribute to it, or the Join node must be configured to accept partial results.
	t.Run("PartialRestore", func(t *testing.T) {
		config := engine.NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := engine.New("complex_restore_chain_partial", buf, engine.WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")
		done := make(chan struct{})
		var executedNodes = make(map[string]bool)

		// Restore ONLY from C1.
		ruleEngine.OnMsg(msg,
			types.WithRestoreNodes(types.ExecuteNode("node_c1")),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				for _, log := range snapshot.Logs {
					executedNodes[log.Id] = true
					if log.Err != "" {
						t.Logf("Node %s error: %s", log.Id, log.Err)
					}
				}
				close(done)
			}),
		)

		select {
		case <-done:
			// C1 should be executed
			if !executedNodes["node_c1"] {
				t.Error("node_c1 was not executed")
			}
			// D, E, F should be executed
			// D will wait for C2 until timeout (5s) because C2 is not provided in restore context.
			// After timeout, D fails with "context deadline exceeded" (standard Join node behavior when missing inputs).
			// So E and F are NOT executed.
			if !executedNodes["node_d"] {
				t.Error("node_d was not executed")
			}
			if executedNodes["node_e"] {
				t.Error("node_e should not be executed (D failed)")
			}
			if executedNodes["node_f"] {
				t.Error("node_f should not be executed (D failed)")
			}

			// C2, B1, B2 should NOT be executed
			if executedNodes["node_c2"] {
				t.Error("node_c2 should not be executed")
			}
			if executedNodes["node_b1"] {
				t.Error("node_b1 should not be executed")
			}
			if executedNodes["node_b2"] {
				t.Error("node_b2 should not be executed")
			}
		case <-time.After(time.Second * 6):
			t.Fatal("Timeout waiting for PartialRestore test")
		}
	})

	// TestRestoreWithDifferentMessages verifies that we can restore nodes with different messages.
	t.Run("TestRestoreWithDifferentMessages", func(t *testing.T) {
		config := engine.NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := engine.New("complex_restore_chain_diff_msg", buf, engine.WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		// Define messages for B1 and B2
		msgB1 := types.NewMsg(0, "MSG_B1", types.JSON, nil, `{"source":"b1"}`)
		msgB2 := types.NewMsg(0, "MSG_B2", types.JSON, nil, `{"source":"b2"}`)
		// Default message for the engine (will be used if node request doesn't have one, but here we provide one for all)
		defaultMsg := types.NewMsg(0, "DEFAULT_MSG", types.JSON, nil, "{}")

		done := make(chan struct{})
		var executedNodes = make(map[string]bool)
		var nodeLogs = make(map[string]types.RuleNodeRunLog)

		// Restore B1 and B2 with different messages
		ruleEngine.OnMsg(defaultMsg,
			types.WithRestoreNodes(
				types.ExecuteNodeWithMsg("node_b1", msgB1),
				types.ExecuteNodeWithMsg("node_b2", msgB2),
			),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				for _, log := range snapshot.Logs {
					executedNodes[log.Id] = true
					nodeLogs[log.Id] = log
				}
				close(done)
			}),
		)

		select {
		case <-done:
			if !executedNodes["node_b1"] || !executedNodes["node_b2"] {
				t.Error("b1/b2 not executed")
			}
			// Check if B1 received the correct message
			if log, ok := nodeLogs["node_b1"]; ok {
				if log.InMsg.Type != "MSG_B1" {
					t.Errorf("node_b1 expected MSG_B1, got %s", log.InMsg.Type)
				}
				if log.InMsg.Data.Get() != `{"source":"b1"}` {
					t.Errorf("node_b1 expected data {\"source\":\"b1\"}, got %s", log.InMsg.Data.Get())
				}
			} else {
				t.Error("node_b1 log not found")
			}

			// Check if B2 received the correct message
			if log, ok := nodeLogs["node_b2"]; ok {
				if log.InMsg.Type != "MSG_B2" {
					t.Errorf("node_b2 expected MSG_B2, got %s", log.InMsg.Type)
				}
				if log.InMsg.Data.Get() != `{"source":"b2"}` {
					t.Errorf("node_b2 expected data {\"source\":\"b2\"}, got %s", log.InMsg.Data.Get())
				}
				if log.RelationType != types.Success {
					t.Errorf("node_b2 expected relation type %s, got %s", types.Success, log.RelationType)
				}
			} else {
				t.Error("node_b2 log not found")
			}

			// Verify that Join node (D) executed successfully
			if !executedNodes["node_d"] {
				t.Error("node_d was not executed")
			}

		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for TestRestoreWithDifferentMessages")
		}
	})
}
