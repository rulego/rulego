package runlogtest

import (
	"context"
	"fmt"
	"os"
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
	"github.com/rulego/rulego/utils/json"
)

// makeOnCompleted creates a WithOnRuleChainCompleted callback based on different stores
func makeOnCompleted(s store.RunLogStore) types.RuleContextOption {
	return types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
		snapshot.Id = time.Now().Format("20060102150405000") + "_" + snapshot.Id
		data, _ := json.Marshal(snapshot)
		var event model.Event
		_ = json.Unmarshal(data, &event)
		_ = s.Save("bench-user", event)
	})
}

func nopOnCompleted() types.RuleContextOption {
	return types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {})
}

// benchEngine tests the performance of the rule engine under specified log callbacks
func benchEngine(b *testing.B, name string, chainDSL string, opts ...types.RuleContextOption) {
	config := rulego.NewConfig()
	engine, err := rulego.New("bench_"+name, []byte(chainDSL), rulego.WithConfig(config))
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Stop(context.Background())

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData,
			fmt.Sprintf(`{"temperature":%d,"sensorId":"sensor-%d"}`, i%100, i%10))
		engine.OnMsgAndWait(msg, opts...)
	}
}

func BenchmarkEngineWithLog(b *testing.B) {
	// === Simple Rule Chain (1 node) ===

	b.Run("SimpleChain/NoLog", func(b *testing.B) {
		benchEngine(b, "simple_nolog", simpleChainDSL)
	})

	b.Run("SimpleChain/NopLog", func(b *testing.B) {
		benchEngine(b, "simple_nop", simpleChainDSL, nopOnCompleted())
	})

	dir, _ := os.MkdirTemp("", "bench-engine-bbolt-*")
	cfg := config.Config{DataDir: dir}
	bs, _ := bboltstore.NewRunLogStore(cfg, types.DefaultLogger())
	defer func() { bs.Close(); os.RemoveAll(dir) }()

	b.Run("SimpleChain/BBoltLog", func(b *testing.B) {
		benchEngine(b, "simple_bbolt", simpleChainDSL, makeOnCompleted(bs))
	})

	dir2, _ := os.MkdirTemp("", "bench-engine-jsonl-*")
	cfg2 := config.Config{DataDir: dir2}
	js, _ := jsonlstore.NewRunLogStore(cfg2, types.DefaultLogger())
	defer func() { js.Close(); os.RemoveAll(dir2) }()

	b.Run("SimpleChain/JsonlLog", func(b *testing.B) {
		benchEngine(b, "simple_jsonl", simpleChainDSL, makeOnCompleted(js))
	})

	// === Multi-node rule chain (10 nodes) ===

	b.Run("MultiNode/NoLog", func(b *testing.B) {
		benchEngine(b, "multi_nolog", multiNodeChainDSL)
	})

	b.Run("MultiNode/NopLog", func(b *testing.B) {
		benchEngine(b, "multi_nop", multiNodeChainDSL, nopOnCompleted())
	})

	b.Run("MultiNode/BBoltLog", func(b *testing.B) {
		benchEngine(b, "multi_bbolt", multiNodeChainDSL, makeOnCompleted(bs))
	})

	b.Run("MultiNode/JsonlLog", func(b *testing.B) {
		benchEngine(b, "multi_jsonl", multiNodeChainDSL, makeOnCompleted(js))
	})

	// === Nop Store Comparison ===

	b.Run("SimpleChain/NopStore", func(b *testing.B) {
		benchEngine(b, "simple_nopstore", simpleChainDSL, makeOnCompleted(nopstore.NopRunLogStore{}))
	})

	b.Run("MultiNode/NopStore", func(b *testing.B) {
		benchEngine(b, "multi_nopstore", multiNodeChainDSL, makeOnCompleted(nopstore.NopRunLogStore{}))
	})
}
