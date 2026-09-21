package integration

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/common"
	"github.com/rulego/rulego/utils/pool"
)

// A while node is a tautology loop when its condition references fields that
// no node in the do branch updates, e.g. "msg.count==nil || msg.count < 3"
// with a do branch that only forwards the message. A single message then
// never terminates. The tests below cover both directions of MsgMaxHops:
// without a budget such a chain runs forever (proved via a break-marker
// escape hatch), with a budget it is terminated with ErrMsgHopBudgetExceeded.
const hopsTautologyCondition = "msg.count==nil || msg.count < 3"

// hopsNoopNode forwards the message without touching any field.
type hopsNoopNode struct{}

func (x *hopsNoopNode) Type() string { return "testHopsNoop" }
func (x *hopsNoopNode) New() types.Node {
	return &hopsNoopNode{}
}
func (x *hopsNoopNode) Init(types.Config, types.Configuration) error { return nil }
func (x *hopsNoopNode) Destroy()                                     {}

var hopsNoopRuns int32

func (x *hopsNoopNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	atomic.AddInt32(&hopsNoopRuns, 1)
	ctx.TellSuccess(msg)
}

// hopsBreakNode stops the loop after N iterations by setting the break
// marker the while node honors, so unbounded-loop tests still terminate.
type hopsBreakNode struct{}

func (x *hopsBreakNode) Type() string { return "testHopsBreak" }
func (x *hopsBreakNode) New() types.Node {
	return &hopsBreakNode{}
}
func (x *hopsBreakNode) Init(types.Config, types.Configuration) error { return nil }
func (x *hopsBreakNode) Destroy()                                     {}

var hopsBreakRounds int32

func (x *hopsBreakNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if atomic.AddInt32(&hopsBreakRounds, 1) >= 200 {
		msg.Metadata.PutValue(common.MdKeyBreak, common.MdValueBreak)
	}
	ctx.TellSuccess(msg)
}

// hopsCountNode increments metadata.count each round, driving a finite loop.
type hopsCountNode struct{}

func (x *hopsCountNode) Type() string { return "testHopsCount" }
func (x *hopsCountNode) New() types.Node {
	return &hopsCountNode{}
}
func (x *hopsCountNode) Init(types.Config, types.Configuration) error { return nil }
func (x *hopsCountNode) Destroy()                                     {}

func (x *hopsCountNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	count, _ := strconv.Atoi(msg.Metadata.GetValue("count"))
	count++
	msg.Metadata.PutValue("count", strconv.Itoa(count))
	ctx.TellSuccess(msg)
}

// hopsRecurseNode sends the message back to its own chain head.
type hopsRecurseNode struct{}

func (x *hopsRecurseNode) Type() string { return "testHopsRecurse" }
func (x *hopsRecurseNode) New() types.Node {
	return &hopsRecurseNode{}
}
func (x *hopsRecurseNode) Init(types.Config, types.Configuration) error { return nil }
func (x *hopsRecurseNode) Destroy()                                     {}

var hopsRecurseCount int32

func (x *hopsRecurseNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	atomic.AddInt32(&hopsRecurseCount, 1)
	ctx.TellFlow("hopsRecurseChain", msg)
}

func init() {
	_ = rulego.Registry.Register(&hopsNoopNode{})
	_ = rulego.Registry.Register(&hopsBreakNode{})
	_ = rulego.Registry.Register(&hopsCountNode{})
	_ = rulego.Registry.Register(&hopsRecurseNode{})
}

func hopsWhileDSL(id, doType, condition string) string {
	return fmt.Sprintf(`{
		"ruleChain":{"id":"%s","name":"hops budget test","root":true},
		"metadata":{
			"nodes":[
				{"id":"n1","type":"while","name":"w","configuration":{"condition":"%s","do":"n2","mode":2}},
				{"id":"n2","type":"%s","name":"do"}
			],
			"connections":[]
		}
	}`, id, condition, doType)
}

func newHopsEngine(t *testing.T, id, dsl string, opts ...types.Option) types.RuleEngine {
	t.Helper()
	opts = append([]types.Option{types.WithComponentsRegistry(rulego.Registry)}, opts...)
	config := rulego.NewConfig(opts...)
	engine, err := rulego.New(id, []byte(dsl), rulego.WithConfig(config))
	if err != nil {
		t.Fatalf("new engine %s: %v", id, err)
	}
	return engine
}

// Without a budget a tautology while loop never ends on its own: it runs
// 200 rounds and stops only because the do node sets the break marker.
func TestWhileTautologyNeverTerminatesWithoutBudget(t *testing.T) {
	atomic.StoreInt32(&hopsBreakRounds, 0)
	engine := newHopsEngine(t, "hopsNoBudget", hopsWhileDSL("hopsNoBudget", "testHopsBreak", hopsTautologyCondition))

	done := make(chan error, 1)
	engine.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"count":0}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			done <- err
		}))

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected end error: %v", err)
		}
		if rounds := atomic.LoadInt32(&hopsBreakRounds); rounds < 200 {
			t.Fatalf("tautology loop ended after %d rounds without the break marker", rounds)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tautology loop hangs without budget")
	}
}

// With MsgMaxHops the same tautology chain is circuit broken: the message
// ends with Failure wrapping ErrMsgHopBudgetExceeded and goroutines stay
// bounded instead of growing with the loop.
func TestWhileTautologyCircuitBreaksWithBudget(t *testing.T) {
	engine := newHopsEngine(t, "hopsBudget", hopsWhileDSL("hopsBudget", "testHopsNoop", hopsTautologyCondition),
		types.WithMsgMaxHops(50))

	runtimeBefore := runtime.NumGoroutine()
	var mu sync.Mutex
	var gotErr error
	var gotRel string
	var hops int64
	done := make(chan struct{}, 1)
	engine.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"count":0}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			mu.Lock()
			gotErr = err
			gotRel = relationType
			hops = msg.Hops()
			mu.Unlock()
			done <- struct{}{}
		}))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("budget did not terminate the tautology loop")
	}
	mu.Lock()
	defer mu.Unlock()
	if !errors.Is(gotErr, types.ErrMsgHopBudgetExceeded) {
		t.Fatalf("expected ErrMsgHopBudgetExceeded, got %v", gotErr)
	}
	if gotRel != types.Failure {
		t.Fatalf("expected %s relation, got %s", types.Failure, gotRel)
	}
	if hops < 50 {
		t.Fatalf("expected at least 50 hops, got %d", hops)
	}
	if grown := runtime.NumGoroutine() - runtimeBefore; grown >= 100 {
		t.Fatalf("goroutines grew by %d for one circuit-broken message", grown)
	}
}

// Finite loops behave exactly as before when the budget is disabled.
func TestWhileFiniteLoopUnaffectedWhenBudgetDisabled(t *testing.T) {
	engine := newHopsEngine(t, "hopsDisabled", hopsWhileDSL("hopsDisabled", "testHopsCount", "int(metadata.count) < 5"),
		types.WithMsgMaxHops(0))

	meta := types.NewMetadata()
	meta.PutValue("count", "0")
	done := make(chan types.RuleMsg, 1)
	var gotErr error
	engine.OnMsg(types.NewMsg(0, "TEST", types.JSON, meta, `{}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			gotErr = err
			done <- msg
		}))

	select {
	case msg := <-done:
		if gotErr != nil {
			t.Fatalf("unexpected end error: %v", gotErr)
		}
		if got := msg.Metadata.GetValue("count"); got != "5" {
			t.Fatalf("loop should run 5 rounds, count=%s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("finite loop did not finish with budget disabled")
	}
}

// Fan-out branches share the budget of the same message: copies and the
// original count against one hop total, so the chain trips the budget.
func TestMsgHopsFanOutAccumulates(t *testing.T) {
	dsl := `{
		"ruleChain":{"id":"hopsFanOut","name":"fanout","root":true},
		"metadata":{
			"nodes":[
				{"id":"s","type":"testHopsNoop","name":"s"},
				{"id":"a1","type":"testHopsNoop","name":"a1"},{"id":"a2","type":"testHopsNoop","name":"a2"},
				{"id":"b1","type":"testHopsNoop","name":"b1"},{"id":"b2","type":"testHopsNoop","name":"b2"}
			],
			"connections":[
				{"fromId":"s","toId":"a1","type":"Success"},
				{"fromId":"s","toId":"b1","type":"Success"},
				{"fromId":"a1","toId":"a2","type":"Success"},
				{"fromId":"b1","toId":"b2","type":"Success"}
			]
		}
	}`
	engine := newHopsEngine(t, "hopsFanOut", dsl, types.WithMsgMaxHops(4))
	atomic.StoreInt32(&hopsNoopRuns, 0)

	// End callbacks are dispatched through the pool asynchronously; wait for
	// the first one, then give the remaining branches a beat to land.
	var mu sync.Mutex
	var errs []error
	done := make(chan struct{}, 4)
	engine.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			done <- struct{}{}
		}))

	select {
	case <-done:
		time.Sleep(100 * time.Millisecond)
	case <-time.After(5 * time.Second):
		t.Fatal("fan-out chain did not finish")
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, err := range errs {
		if errors.Is(err, types.ErrMsgHopBudgetExceeded) {
			found = true
		}
	}
	// 5 hops total against a budget of 4: the branches must have expanded
	// (s/a1/a2/b1 at minimum) before the last hop trips the budget.
	runs := atomic.LoadInt32(&hopsNoopRuns)
	if !found || runs < 4 {
		t.Fatalf("branches should share one hop budget and trip it, errs=%v noopRuns=%d", errs, runs)
	}
}

// Sub chain recursion counts against the same message budget and is bounded.
func TestMsgHopsSubChainRecursion(t *testing.T) {
	atomic.StoreInt32(&hopsRecurseCount, 0)
	dsl := `{
		"ruleChain":{"id":"hopsRecurseChain","name":"recurse","root":true},
		"metadata":{
			"nodes":[{"id":"r1","type":"testHopsRecurse","name":"r1"}],
			"connections":[]
		}
	}`
	engine := newHopsEngine(t, "hopsRecurseChain", dsl, types.WithMsgMaxHops(20))
	engine.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`))

	time.Sleep(500 * time.Millisecond)
	first := atomic.LoadInt32(&hopsRecurseCount)
	time.Sleep(300 * time.Millisecond)
	second := atomic.LoadInt32(&hopsRecurseCount)

	if first != second {
		t.Fatalf("recursion still running: %d -> %d rounds", first, second)
	}
	if first == 0 || first >= 200 {
		t.Fatalf("recursion should be bounded by the budget, got %d rounds", first)
	}
}

// A bounded pool degrades to running overflow tasks in the caller goroutine:
// a finite loop still completes and goroutines stay bounded.
func TestBoundedPoolSynchronousFallback(t *testing.T) {
	wp := &pool.WorkerPool{MaxWorkersCount: 1}
	wp.Start()
	defer wp.Stop()

	engine := newHopsEngine(t, "hopsBoundedPool",
		hopsWhileDSL("hopsBoundedPool", "testHopsCount", "int(metadata.count) < 50"),
		types.WithMsgMaxHops(0), types.WithPool(wp))

	runtimeBefore := runtime.NumGoroutine()
	meta := types.NewMetadata()
	meta.PutValue("count", "0")
	done := make(chan types.RuleMsg, 1)
	var gotErr error
	engine.OnMsg(types.NewMsg(0, "TEST", types.JSON, meta, `{}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			gotErr = err
			done <- msg
		}))

	select {
	case msg := <-done:
		if gotErr != nil {
			t.Fatalf("unexpected end error: %v", gotErr)
		}
		if got := msg.Metadata.GetValue("count"); got != "50" {
			t.Fatalf("loop should run 50 rounds, count=%s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not finish with a 1-worker pool and synchronous fallback")
	}
	if grown := runtime.NumGoroutine() - runtimeBefore; grown >= 50 {
		t.Fatalf("goroutines grew by %d under a bounded pool", grown)
	}
}
