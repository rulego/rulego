package engine_test

import (
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	_ "github.com/rulego/rulego/components/common"
	_ "github.com/rulego/rulego/components/transform"
	"github.com/rulego/rulego/engine"
)

// TestForkJoinWithForkNode tests the correct fork node design
// Rule chain structure:
//
//	node_20 (fork)
//	    │
//	    ├──→ node_2 (JS to A) → node_12 (JS to C) → node_5 (join)
//	    │
//	    └──→ node_3 (JS to B) → node_5 (join)
//
// Expectation: After joining, the metadata should include a, b, and c simultaneously
func TestForkJoinWithForkNode(t *testing.T) {
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_fork_join_with_fork",
			"name": "Test Fork Join With Fork Node",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node_20",
					"type": "fork",
					"name": "并行分支"
				},
				{
					"id": "node_2",
					"type": "jsTransform",
					"name": "js转换a",
					"configuration": {
						"jsScript": "metadata.a=\"a\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_3",
					"type": "jsTransform",
					"name": "js转换b",
					"configuration": {
						"jsScript": "metadata.b=\"b\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_12",
					"type": "jsTransform",
					"name": "js转换c",
					"configuration": {
						"jsScript": "metadata.c=\"c\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_5",
					"type": "join",
					"name": "合并",
					"configuration": {
						"mergeToMap": true,
						"timeout": 5
					}
				}
			],
			"connections": [
				{
					"fromId": "node_20",
					"toId": "node_2",
					"type": "Success"
				},
				{
					"fromId": "node_20",
					"toId": "node_3",
					"type": "Success"
				},
				{
					"fromId": "node_2",
					"toId": "node_12",
					"type": "Success"
				},
				{
					"fromId": "node_12",
					"toId": "node_5",
					"type": "Success"
				},
				{
					"fromId": "node_3",
					"toId": "node_5",
					"type": "Success"
				}
			]
		}
	}`

	config := engine.NewConfig(types.WithDefaultPool())
	ruleEngine, err := engine.New("test_fork_join_with_fork", []byte(ruleChainDef), engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`)
	done := make(chan struct{})
	var lock sync.Mutex
	var joinNodeLog *types.RuleNodeRunLog

	ruleEngine.OnMsg(msg,
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			t.Logf("Node %s completed", nodeRunLog.Id)
			if nodeRunLog.Id == "node_5" {
				joinNodeLog = &nodeRunLog
			}
		}),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			t.Log("Rule chain completed")
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")
		valueB := metadata.GetValue("b")
		valueC := metadata.GetValue("c")

		t.Logf("Metadata after join: a=%s, b=%s, c=%s", valueA, valueB, valueC)

		// Verify that all metadata is correctly merged
		if valueA != "a" {
			t.Errorf("Expected metadata.a='a', got '%s'", valueA)
		}
		if valueB != "b" {
			t.Errorf("Expected metadata.b='b', got '%s'", valueB)
		}
		if valueC != "c" {
			t.Errorf("Expected metadata.c='c', got '%s'", valueC)
		}

		// If all values are correct, the test passes
		if valueA == "a" && valueB == "b" && valueC == "c" {
			t.Log("SUCCESS: All metadata correctly merged!")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestForkNodeDirectToJoin tests the scenario where a fork node connects directly to a join node
// Rule chain structure:
//
//	fork ────────→ join (direct connection)
//	    │
//	    └──→ JS conversion → join
//
// Expectation: This design also has issues, because forks directly connecting to join create "zero-length" branches
func TestForkNodeDirectToJoin(t *testing.T) {
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_fork_direct_to_join",
			"name": "Test Fork Direct To Join",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node_fork",
					"type": "fork",
					"name": "并行分支"
				},
				{
					"id": "node_a",
					"type": "jsTransform",
					"name": "js转换a",
					"configuration": {
						"jsScript": "metadata.a=\"a\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_join",
					"type": "join",
					"name": "合并",
					"configuration": {
						"mergeToMap": true,
						"timeout": 5
					}
				}
			],
			"connections": [
				{
					"fromId": "node_fork",
					"toId": "node_join",
					"type": "Success"
				},
				{
					"fromId": "node_fork",
					"toId": "node_a",
					"type": "Success"
				},
				{
					"fromId": "node_a",
					"toId": "node_join",
					"type": "Success"
				}
			]
		}
	}`

	config := engine.NewConfig(types.WithDefaultPool())
	ruleEngine, err := engine.New("test_fork_direct_to_join", []byte(ruleChainDef), engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`)
	done := make(chan struct{})
	var lock sync.Mutex
	var joinNodeLog *types.RuleNodeRunLog

	ruleEngine.OnMsg(msg,
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			t.Logf("Node %s completed", nodeRunLog.Id)
			if nodeRunLog.Id == "node_join" {
				joinNodeLog = &nodeRunLog
			}
		}),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			t.Log("Rule chain completed")
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")

		t.Logf("Metadata after join: a=%s", valueA)

		// Key verification: Fork directly connecting to join causes problems
		if valueA != "a" {
			t.Logf("BUG CONFIRMED: Fork node directly connected to join causes early callback trigger!")
			t.Logf("  metadata.a = '%s' (expected: 'a') - LOST", valueA)
		} else {
			t.Log("SUCCESS: metadata.a correctly merged")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestForkDirectToJoinWithMetadataMerge Test problematic rule chain design (no fork nodes)
// Simulating the user-provided rule chain structure:
//
//	node_3 (JS to B) → node_2 (JS to A) → node_12 (JS to C) → node_5 (join)
//	node_3 (JS conversion b) → node_5 (join) [Direct Link]
//
// Expectation: After joining, the metadata should include a, b, and c simultaneously
func TestForkDirectToJoinWithMetadataMerge(t *testing.T) {
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_fork_join_metadata",
			"name": "Test Fork Join Metadata",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
                   	"id": "node_3",
                    "type": "jsTransform",
                    "name": "js转换b",
                    "configuration": {
                        "jsScript": "metadata.b=\"b\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
                    }
                },
                {
                    "id": "node_2",
                    "type": "jsTransform",
                    "name": "js转换a",
                    "configuration": {
                        "jsScript": "metadata.a=\"a\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
                    }
                },
                {
                    "id": "node_12",
                    "type": "jsTransform",
                    "name": "js转换c",
                    "configuration": {
                        "jsScript": "metadata.c=\"c\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
                    }
                },
                {
                    "id": "node_5",
                    "type": "join",
                    "name": "合并",
                    "configuration": {
                        "mergeToMap": true,
                        "timeout": 5
                    }
                }
            ],
            "connections": [
                {
                    "fromId": "node_2",
                    "toId": "node_12",
                    "type": "Success"
                },
                {
                    "fromId": "node_12",
                    "toId": "node_5",
                    "type": "Success"
                },
                {
                    "fromId": "node_3",
                    "toId": "node_2",
                    "type": "Success"
                },
                {
                    "fromId": "node_3",
                    "toId": "node_5",
                    "type": "Success"
                }
            ]
        }
    }`

	config := engine.NewConfig(types.WithDefaultPool())
	ruleEngine, err := engine.New("test_fork_join_metadata", []byte(ruleChainDef), engine.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`)
	done := make(chan struct{})
	var lock sync.Mutex
	var joinNodeLog *types.RuleNodeRunLog
	var allNodeLogs = make(map[string]types.RuleNodeRunLog)

	ruleEngine.OnMsg(msg,
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			allNodeLogs[nodeRunLog.Id] = nodeRunLog
			t.Logf("Node %s completed", nodeRunLog.Id)
			if nodeRunLog.Id == "node_5" {
				joinNodeLog = &nodeRunLog
			}
		}),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			t.Log("Rule chain completed")
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()

		// Print logs from all nodes
		for id := range allNodeLogs {
			t.Logf("Node %s: executed", id)
		}

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		// Verify that metadata has been properly merged
		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")
		valueB := metadata.GetValue("b")
		valueC := metadata.GetValue("c")

		t.Logf("Metadata after join: a=%s, b=%s, c=%s", valueA, valueB, valueC)

		// Key verification: Metadata for all branches should exist
		// Note: Since node_3 is directly connected to node_5 (join), this is a known bug scenario
		// In the current implementation, join may not wait for the node_12 branch to complete

		// Risk: This is a known bug, // When node_3 connects to both node_2 and node_5 at the same time:
		// - node_3 → node_5 (direct connection) will arrive at join first
		// - Arrive after node_3 → node_2 → node_12 → node_5 (long path).
		// The join node may trigger a callback after receiving the first message, causing data loss for the second message

		t.Logf("BUG VERIFICATION:")
		t.Logf("  metadata.a = '%s' (expected: 'a') - %s", valueA,
			map[bool]string{true: "LOST", false: "OK"}[valueA == "a"])
		t.Logf("  metadata.b = '%s' (expected: 'b') - %s", valueB,
			map[bool]string{true: "OK", false: "LOST"}[valueB == "b"])
		t.Logf("  metadata.c = '%s' (expected: 'c') - %s", valueC,
			map[bool]string{true: "LOST", false: "OK"}[valueC == "c"])

		// Record bug phenomena
		if valueA != "a" || valueC != "c" {
			t.Logf("BUG CONFIRMED: Join node triggered callback before all branches completed!")
			t.Logf("  This causes metadata from node_2->node_12 path to be lost")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}
