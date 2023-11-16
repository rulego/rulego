package main

import (
	"github.com/rulego/rulego/api/types"
	"strings"
	"time"
)

//go build -buildmode=plugin -o plugin.so plugin.go # Compile the plugin and generate the plugin.so file
//need to compile in mac or linux environment

// Plugins plugin entry point
var Plugins MyPlugins

type MyPlugins struct{}

func (p *MyPlugins) Init() error {
	return nil
}
func (p *MyPlugins) Components() []types.Node {
	return []types.Node{&UpperNode{}, &TimeNode{}, &FilterNode{}}
}

// UpperNode A plugin that converts the message data to uppercase
type UpperNode struct{}

func (n *UpperNode) Type() string {
	return "test/upper"
}
func (n *UpperNode) New() types.Node {
	return &UpperNode{}
}
func (n *UpperNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *UpperNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Data = strings.ToUpper(msg.Data)
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
}

func (n *UpperNode) Destroy() {
	// Do some cleanup work
}

// A plugin that adds a timestamp to the message metadata
type TimeNode struct{}

func (n *TimeNode) Type() string {
	return "test/time"
}

func (n *TimeNode) New() types.Node {
	return &TimeNode{}
}

func (n *TimeNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *TimeNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Metadata.PutValue("timestamp", time.Now().Format(time.RFC3339))
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
}

func (n *TimeNode) Destroy() {
	// Do some cleanup work
}

type FilterNode struct {
	blacklist map[string]bool
}

func (n *FilterNode) Type() string {
	return "test/filter"
}
func (n *FilterNode) New() types.Node {
	return &FilterNode{}
}
func (n *FilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *FilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if n.blacklist[msg.Type] || n.blacklist[string(msg.DataType)] {
		// Skip the message and do not send it to the next node
	}
	// Send the message to the next node
	ctx.TellNext(msg)
}

func (n *FilterNode) Destroy() {
	// Do some cleanup work
}
