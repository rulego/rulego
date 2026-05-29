package runlogtest

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/bboltstore"
	"github.com/rulego/rulego/server/internal/store/jsonlstore"
	"github.com/rulego/rulego/server/internal/store/nopstore"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

// BenchmarkThroughput 并发吞吐量测试
func BenchmarkThroughput(b *testing.B) {
	scenarios := []struct {
		name      string
		chainDSL  string
		chainName string
	}{
		{"SimpleChain", simpleChainDSL, "throughput_simple"},
		{"MultiNode", multiNodeChainDSL, "throughput_multi"},
	}

	stores := map[string]func() (store.RunLogStore, func()){
		"NoLog": func() (store.RunLogStore, func()) {
			return nil, func() {}
		},
		"BBolt": func() (store.RunLogStore, func()) {
			dir, _ := os.MkdirTemp("", "throughput-bbolt-*")
			cfg := config.Config{DataDir: dir}
			s, _ := bboltstore.NewRunLogStore(cfg, types.DefaultLogger())
			return s, func() { s.Close(); os.RemoveAll(dir) }
		},
		"Jsonl": func() (store.RunLogStore, func()) {
			dir, _ := os.MkdirTemp("", "throughput-jsonl-*")
			cfg := config.Config{DataDir: dir}
			s, _ := jsonlstore.NewRunLogStore(cfg, types.DefaultLogger())
			return s, func() { s.Close(); os.RemoveAll(dir) }
		},
	}

	for _, sc := range scenarios {
		for storeName, storeFactory := range stores {
			name := fmt.Sprintf("%s/%s", sc.name, storeName)
			b.Run(name, func(b *testing.B) {
				s, cleanup := storeFactory()
				defer cleanup()

				ruleConfig := rulego.NewConfig()
				engine, err := rulego.New(sc.chainName+"_"+storeName, []byte(sc.chainDSL), rulego.WithConfig(ruleConfig))
				if err != nil {
					b.Fatal(err)
				}
				defer engine.Stop(context.Background())

				var opts []types.RuleContextOption
				if s != nil {
					opts = append(opts, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
						_ = s.Save("throughput-user", eventFromSnapshot(snapshot))
					}))
				}

				var count int64
				workers := 8
				var wg sync.WaitGroup
				perWorker := b.N / workers

				b.ResetTimer()
				b.ReportAllocs()
				start := time.Now()

				for w := 0; w < workers; w++ {
					wg.Add(1)
					go func(offset int) {
						defer wg.Done()
						metaData := types.NewMetadata()
						for i := 0; i < perWorker; i++ {
							msg := types.NewMsg(0, "TEST", types.JSON, metaData,
								fmt.Sprintf(`{"id":%d}`, offset*perWorker+i))
							engine.OnMsg(msg, opts...)
							atomic.AddInt64(&count, 1)
						}
					}(w)
				}
				wg.Wait()
				elapsed := time.Since(start)

				// 等待异步日志写入完成
				if s != nil {
					time.Sleep(500 * time.Millisecond)
				}

				n := atomic.LoadInt64(&count)
				b.ReportMetric(float64(n)/elapsed.Seconds(), "ops/sec")
			})
		}
	}
}

func BenchmarkThroughput_NopStore(b *testing.B) {
	s := nopstore.NopRunLogStore{}
	for _, sc := range []struct {
		name      string
		chainDSL  string
		chainName string
	}{
		{"SimpleChain", simpleChainDSL, "tp_nop_simple"},
		{"MultiNode", multiNodeChainDSL, "tp_nop_multi"},
	} {
		b.Run(sc.name, func(b *testing.B) {
			ruleConfig := rulego.NewConfig()
			engine, _ := rulego.New(sc.chainName, []byte(sc.chainDSL), rulego.WithConfig(ruleConfig))
			defer engine.Stop(context.Background())

			var count int64
			workers := 8
			var wg sync.WaitGroup
			perWorker := b.N / workers

			b.ResetTimer()
			start := time.Now()

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(offset int) {
					defer wg.Done()
					metaData := types.NewMetadata()
					for i := 0; i < perWorker; i++ {
						msg := types.NewMsg(0, "TEST", types.JSON, metaData,
							fmt.Sprintf(`{"id":%d}`, offset*perWorker+i))
						engine.OnMsg(msg, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
							_ = s.Save("tp-user", eventFromSnapshot(snapshot))
						}))
						atomic.AddInt64(&count, 1)
					}
				}(w)
			}
			wg.Wait()
			elapsed := time.Since(start)
			time.Sleep(200 * time.Millisecond)

			n := atomic.LoadInt64(&count)
			b.ReportMetric(float64(n)/elapsed.Seconds(), "ops/sec")
			b.ReportAllocs()
		})
	}
}

func eventFromSnapshot(snapshot types.RuleChainRunSnapshot) model.Event {
	return model.Event{
		Id:        snapshot.Id,
		ChainId:   snapshot.RuleChain.RuleChain.ID,
		ChainName: snapshot.RuleChain.RuleChain.Name,
		StartTs:   snapshot.StartTs / int64(time.Millisecond),
		EndTs:     snapshot.EndTs / int64(time.Millisecond),
		Success:   true,
	}
}
