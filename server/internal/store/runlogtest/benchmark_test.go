package runlogtest

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/bboltstore"
	"github.com/rulego/rulego/server/internal/store/jsonlstore"
	"github.com/rulego/rulego/server/internal/store/nopstore"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

// 简单规则链 DSL：单节点 jsTransform
const simpleChainDSL = `
{
  "ruleChain": {
    "id": "bench_chain",
    "name": "bench_chain",
    "root": true
  },
  "metadata": {
    "nodes": [
      {"id":"n1","type":"jsTransform","name":"transform","configuration":{"jsScript":"msg.result='ok';return {'msg':msg,'metadata':metadata,'msgType':msgType};"}}
    ],
    "connections": []
  }
}
`

// 10 节点规则链 DSL：模拟实际场景
const multiNodeChainDSL = `
{
  "ruleChain": {
    "id": "bench_multi",
    "name": "bench_multi",
    "root": true
  },
  "metadata": {
    "nodes": [
      {"id":"n1","type":"jsTransform","name":"t1","configuration":{"jsScript":"msg.step=1;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n2","type":"jsTransform","name":"t2","configuration":{"jsScript":"msg.step=2;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n3","type":"jsTransform","name":"t3","configuration":{"jsScript":"msg.step=3;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n4","type":"jsTransform","name":"t4","configuration":{"jsScript":"msg.step=4;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n5","type":"jsTransform","name":"t5","configuration":{"jsScript":"msg.step=5;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n6","type":"jsTransform","name":"t6","configuration":{"jsScript":"msg.step=6;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n7","type":"jsTransform","name":"t7","configuration":{"jsScript":"msg.step=7;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n8","type":"jsTransform","name":"t8","configuration":{"jsScript":"msg.step=8;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n9","type":"jsTransform","name":"t9","configuration":{"jsScript":"msg.step=9;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id":"n10","type":"jsTransform","name":"t10","configuration":{"jsScript":"msg.step=10;return {'msg':msg,'metadata':metadata,'msgType':msgType};"}}
    ],
    "connections": [
      {"fromId":"n1","toId":"n2","type":"Success"},
      {"fromId":"n2","toId":"n3","type":"Success"},
      {"fromId":"n3","toId":"n4","type":"Success"},
      {"fromId":"n4","toId":"n5","type":"Success"},
      {"fromId":"n5","toId":"n6","type":"Success"},
      {"fromId":"n6","toId":"n7","type":"Success"},
      {"fromId":"n7","toId":"n8","type":"Success"},
      {"fromId":"n8","toId":"n9","type":"Success"},
      {"fromId":"n9","toId":"n10","type":"Success"}
    ]
  }
}
`

func makeEvent(id int) model.Event {
	now := time.Now()
	return model.Event{
		Id:        fmt.Sprintf("evt-%d", id),
		ChainId:   "bench-chain",
		ChainName: "benchmark",
		StartTs:   now.UnixMilli(),
		EndTs:     now.Add(10 * time.Millisecond).UnixMilli(),
		Success:   true,
	}
}

// ==================== 存储层 Benchmark ====================

func benchStore(b *testing.B, name string, s store.RunLogStore) {
	b.Run(name+"/Save", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = s.Save("bench-user", makeEvent(i))
		}
	})

	// 先写入 100 条用于查询
	for i := 0; i < 100; i++ {
		_ = s.Save("bench-user", makeEvent(i + 10000))
	}

	b.Run(name+"/List", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, _ = s.List("bench-user", "bench-chain", time.Time{}, time.Time{}, 20, 1)
		}
	})

	b.Run(name+"/Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = s.Get("bench-user", "evt-10050")
		}
	})
}

func BenchmarkStores(b *testing.B) {
	// BBolt
	dir, _ := os.MkdirTemp("", "bench-bbolt-*")
	cfg := config.Config{DataDir: dir}
	bs, _ := bboltstore.NewRunLogStore(cfg, types.DefaultLogger())
	defer func() { bs.Close(); os.RemoveAll(dir) }()
	benchStore(b, "BBolt", bs)

	// JSON Lines
	dir2, _ := os.MkdirTemp("", "bench-jsonl-*")
	cfg2 := config.Config{DataDir: dir2}
	js, _ := jsonlstore.NewRunLogStore(cfg2, types.DefaultLogger())
	defer func() { js.Close(); os.RemoveAll(dir2) }()
	benchStore(b, "Jsonl", js)

	// Nop
	benchStore(b, "Nop", nopstore.NopRunLogStore{})
}

// ==================== 并发写入 Benchmark ====================

func benchConcurrentSave(b *testing.B, name string, s store.RunLogStore, workers int) {
	b.Run(fmt.Sprintf("%s/ConcurrentSave_%dw", name, workers), func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		done := make(chan struct{})
		perWorker := b.N / workers
		for w := 0; w < workers; w++ {
			go func(offset int) {
				for i := 0; i < perWorker; i++ {
					_ = s.Save("concurrent-user", makeEvent(offset*perWorker+i))
				}
				done <- struct{}{}
			}(w)
		}
		for w := 0; w < workers; w++ {
			<-done
		}
	})
}

func BenchmarkConcurrentSave(b *testing.B) {
	// BBolt
	dir, _ := os.MkdirTemp("", "bench-con-bbolt-*")
	cfg := config.Config{DataDir: dir}
	bs, _ := bboltstore.NewRunLogStore(cfg, types.DefaultLogger())
	defer func() { bs.Close(); os.RemoveAll(dir) }()
	benchConcurrentSave(b, "BBolt", bs, 10)

	// JSON Lines
	dir2, _ := os.MkdirTemp("", "bench-con-jsonl-*")
	cfg2 := config.Config{DataDir: dir2}
	js, _ := jsonlstore.NewRunLogStore(cfg2, types.DefaultLogger())
	defer func() { js.Close(); os.RemoveAll(dir2) }()
	benchConcurrentSave(b, "Jsonl", js, 10)

	// Nop
	benchConcurrentSave(b, "Nop", nopstore.NopRunLogStore{}, 10)
}
