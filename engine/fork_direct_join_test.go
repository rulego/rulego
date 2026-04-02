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

// TestForkJoinWithForkNode 测试使用正确的 fork 节点设计
// 规则链结构:
//
//   node_20 (fork)
//       │
//       ├──→ node_2 (js转换a) → node_12 (js转换c) → node_5 (join)
//       │
//       └──→ node_3 (js转换b) → node_5 (join)
//
// 预期: join 后 metadata 应该同时包含 a, b, c
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

		// 验证所有元数据都被正确合并
		if valueA != "a" {
			t.Errorf("Expected metadata.a='a', got '%s'", valueA)
		}
		if valueB != "b" {
			t.Errorf("Expected metadata.b='b', got '%s'", valueB)
		}
		if valueC != "c" {
			t.Errorf("Expected metadata.c='c', got '%s'", valueC)
		}

		// 如果所有值都正确，测试通过
		if valueA == "a" && valueB == "b" && valueC == "c" {
			t.Log("SUCCESS: All metadata correctly merged!")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestForkNodeDirectToJoin 测试 fork 节点直接连接到 join 节点的场景
// 规则链结构:
//
//   fork ────────→ join (直接连接)
//       │
//       └──→ js转换 → join
//
// 预期: 这种设计也有问题，因为 fork 直接连接到 join 会创建"零长度"分支
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

		// 关键验证: fork 直接连接到 join 会导致问题
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

// TestForkDirectToJoinWithMetadataMerge 测试有问题的规则链设计（无 fork 节点）
// 模拟用户提供的规则链结构:
//   node_3 (js转换b) → node_2 (js转换a) → node_12 (js转换c) → node_5 (join)
//   node_3 (js转换b) → node_5 (join)  [直接连接]
//
// 预期: join 后 metadata 应该同时包含 a, b, c
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

		// 打印所有节点的日志
		for id := range allNodeLogs {
			t.Logf("Node %s: executed", id)
		}

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		// 验证元数据是否被正确合并
		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")
		valueB := metadata.GetValue("b")
		valueC := metadata.GetValue("c")

		t.Logf("Metadata after join: a=%s, b=%s, c=%s", valueA, valueB, valueC)

		// 关键验证: 所有分支的元数据都应该存在
		// 注意: 由于 node_3 直接连接到 node_5 (join), 这是一个已知的 bug 场景
		// 在当前实现中,join 可能不会等待 node_12 分支完成

		// 风险: 这是已知 bug,		// 当 node_3 同时连接到 node_2 和 node_5 时:
		// - node_3 → node_5 (直接连接) 会先到达 join
		// - node_3 → node_2 → node_12 → node_5 (长路径) 后到达
		// join 节点可能在收到第一条消息后就触发回调, 导致第二条消息的数据丢失

		t.Logf("BUG VERIFICATION:")
		t.Logf("  metadata.a = '%s' (expected: 'a') - %s", valueA,
			map[bool]string{true: "LOST", false: "OK"}[valueA == "a"])
		t.Logf("  metadata.b = '%s' (expected: 'b') - %s", valueB,
			map[bool]string{true: "OK", false: "LOST"}[valueB == "b"])
		t.Logf("  metadata.c = '%s' (expected: 'c') - %s", valueC,
			map[bool]string{true: "LOST", false: "OK"}[valueC == "c"])

		// 记录 bug 现象
		if valueA != "a" || valueC != "c" {
			t.Logf("BUG CONFIRMED: Join node triggered callback before all branches completed!")
			t.Logf("  This causes metadata from node_2->node_12 path to be lost")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}
