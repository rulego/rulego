/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package test

import (
	"context"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// dummyConfigNode exercises the field-matching path of NodeNew/NodeInit with a
// non-empty config struct.
type dummyConfig struct {
	Name  string `json:"name" label:"Name"`
	Count int    `json:"count" label:"Count"`
}

type dummyConfigNode struct {
	Config dummyConfig
}

func (n *dummyConfigNode) Type() string { return "test/dummy" }
func (n *dummyConfigNode) New() types.Node {
	return &dummyConfigNode{Config: dummyConfig{Name: "default", Count: 1}}
}
func (n *dummyConfigNode) Init(types.Config, types.Configuration) error { return nil }
func (n *dummyConfigNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}
func (n *dummyConfigNode) Destroy() {}

// brokenInitNode makes Init fail so InitNode/InitNodeByConfig return nil.
type brokenInitNode struct {
	dummyConfigNode
}

func (n *brokenInitNode) Type() string { return "test/brokenInit" }
func (n *brokenInitNode) New() types.Node {
	return &brokenInitNode{}
}
func (n *brokenInitNode) Init(types.Config, types.Configuration) error {
	return context.Canceled
}
func (n *brokenInitNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func newTestRegistry() *types.SafeComponentSlice {
	registry := &types.SafeComponentSlice{}
	registry.Add(&UpperNode{})
	registry.Add(&TimeNode{})
	registry.Add(&dummyConfigNode{})
	registry.Add(&brokenInitNode{})
	return registry
}

func TestNodeNew(t *testing.T) {
	registry := newTestRegistry()

	NodeNew(t, "test/upper", &UpperNode{}, types.Configuration{}, registry)
	NodeNew(t, "test/dummy", &dummyConfigNode{}, types.Configuration{
		"name":  "default",
		"count": 1,
	}, registry)
}

func TestNodeInit(t *testing.T) {
	registry := newTestRegistry()

	NodeInit(t, "test/dummy", types.Configuration{
		"name":  "default",
		"count": 1,
	}, types.Configuration{
		"name":  "default",
		"count": 1,
	}, registry)
}

func TestCreateAndInitNode(t *testing.T) {
	registry := newTestRegistry()

	node, err := CreateAndInitNode("test/dummy", types.Configuration{
		"name":  "overridden",
		"count": 3,
	}, registry)
	assert.Nil(t, err)
	assert.NotNil(t, node)
	assert.Equal(t, "test/dummy", node.Type())
}

func TestInitNode(t *testing.T) {
	registry := newTestRegistry()

	node := InitNode("test/dummy", types.Configuration{"name": "x"}, registry)
	assert.NotNil(t, node)

	// failing Init makes InitNode return nil
	node = InitNode("test/brokenInit", nil, registry)
	assert.Nil(t, node)
}

func TestInitNodeByConfig(t *testing.T) {
	registry := newTestRegistry()

	config := types.NewConfig()
	node := InitNodeByConfig(config, "test/dummy", types.Configuration{"name": "x"}, registry)
	assert.NotNil(t, node)

	node = InitNodeByConfig(config, "test/brokenInit", nil, registry)
	assert.Nil(t, node)
}

func TestNodeOnMsg(t *testing.T) {
	registry := newTestRegistry()
	node := InitNode("test/upper", nil, registry)
	assert.NotNil(t, node)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test")

	callbackDone := make(chan types.RuleMsg, 3)
	NodeOnMsg(t, node, []Msg{
		{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "aa",
			AfterSleep: 10 * time.Millisecond,
		},
		{
			Id:       "fixed-id",
			Ts:       1700000000000,
			DataType: types.TEXT,
			MsgType:  "OTHER_EVENT",
			Data:     "bb",
			MetaData: types.NewMetadata(),
		},
	}, func(msg types.RuleMsg, relationType string, err error) {
		assert.Nil(t, err)
		assert.Equal(t, types.Success, relationType)
		callbackDone <- msg
	})

	for i := 0; i < 2; i++ {
		select {
		case msg := <-callbackDone:
			// UpperNode upper-cases the payload
			assert.True(t, msg.GetData() == "AA" || msg.GetData() == "BB")
		case <-time.After(time.Second * 3):
			t.Fatal("callback not invoked")
		}
	}
}

func TestNodeOnMsgWithChildren(t *testing.T) {
	registry := newTestRegistry()
	node := InitNode("test/upper", nil, registry)
	assert.NotNil(t, node)

	children := map[string]types.Node{"test/time": &TimeNode{}}

	callbackDone := make(chan types.RuleMsg, 1)
	NodeOnMsgWithChildren(t, node, []Msg{
		{MsgType: "T", Data: "cc", MetaData: types.NewMetadata()},
	}, children, func(msg types.RuleMsg, relationType string, err error) {
		assert.Nil(t, err)
		assert.Equal(t, types.Success, relationType)
		callbackDone <- msg
	})

	select {
	case msg := <-callbackDone:
		assert.Equal(t, "CC", msg.GetData())
	case <-time.After(time.Second * 3):
		t.Fatal("callback not invoked")
	}
}

func TestNodeOnMsgWithChildrenAndConfigNilCallback(t *testing.T) {
	registry := newTestRegistry()
	node := InitNode("test/dummy", nil, registry)
	assert.NotNil(t, node)

	// nil callback must not panic inside the safe wrapper
	NodeOnMsgWithChildrenAndConfig(t, types.NewConfig(), node, []Msg{
		{MsgType: "T", Data: "dd", MetaData: types.NewMetadata()},
	}, nil, nil)

	time.Sleep(100 * time.Millisecond)
}

// UpperNode and TimeNode propagate context values through metadata; the shared
// context must be threaded via SetContext on the test rule context.
func TestUpperTimeNodeContextShare(t *testing.T) {
	done := make(chan types.RuleMsg, 1)
	ctx := NewRuleContextFull(types.NewConfig(), &UpperNode{}, nil, func(msg types.RuleMsg, relationType string, err error) {
		done <- msg
	})
	ctx.SetContext(context.WithValue(context.Background(), shareKey, shareValue))

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "mixed")
	(&UpperNode{}).OnMsg(ctx, msg)

	select {
	case uppered := <-done:
		assert.Equal(t, "MIXED", uppered.GetData())
		assert.Equal(t, shareValue, uppered.Metadata.GetValue(shareKey))
	case <-time.After(time.Second * 3):
		t.Fatal("upper node callback not invoked")
	}

	// UpperNode added addShareKey to the context; TimeNode must see both keys
	done2 := make(chan types.RuleMsg, 1)
	ctx2 := NewRuleContextFull(types.NewConfig(), &TimeNode{}, nil, func(msg types.RuleMsg, relationType string, err error) {
		done2 <- msg
	})
	ctx2.SetContext(ctx.GetContext())
	(&TimeNode{}).OnMsg(ctx2, types.NewMsg(0, "TEST2", types.JSON, types.NewMetadata(), "x"))

	select {
	case stamped := <-done2:
		assert.True(t, stamped.Metadata.GetValue("timestamp") != "")
		assert.Equal(t, shareValue, stamped.Metadata.GetValue(shareKey))
		assert.Equal(t, addShareValue, stamped.Metadata.GetValue(addShareKey))
	case <-time.After(time.Second * 3):
		t.Fatal("time node callback not invoked")
	}
}
