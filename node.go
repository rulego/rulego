package rulego

import (
	"errors"
	"rulego/api/types"
)

const (
	defaultNodeIdPrefix = "node"
)

// RuleNodeCtx 节点组件实例定义
type RuleNodeCtx struct {
	//组件实例
	types.Node
	//组件配置
	SelfDefinition RuleNode
	//规则引擎配置
	Config types.Config
}

//InitRuleNodeCtx 初始化RuleNodeCtx
func InitRuleNodeCtx(config types.Config, selfDefinition RuleNode) (*RuleNodeCtx, error) {
	node, err := config.ComponentsRegistry.NewNode(selfDefinition.Type)
	if err != nil {
		return &RuleNodeCtx{}, err
	} else {
		if selfDefinition.Configuration == nil {
			selfDefinition.Configuration = make(types.Configuration)
		}
		if err = node.Init(config, selfDefinition.Configuration); err != nil {
			return &RuleNodeCtx{}, err
		} else {
			return &RuleNodeCtx{
				Node:           node,
				SelfDefinition: selfDefinition,
				Config:         config,
			}, nil
		}
	}

}

func (rn *RuleNodeCtx) IsDebugMode() bool {
	return rn.SelfDefinition.DebugMode
}

func (rn *RuleNodeCtx) GetNodeId() types.RuleNodeId {
	return types.RuleNodeId{Id: rn.SelfDefinition.Id, Type: types.NODE}
}

func (rn *RuleNodeCtx) ReloadSelf(def []byte) error {
	if ruleNodeCtx, err := rn.Config.Parser.DecodeRuleNode(rn.Config, def); err == nil {
		//先销毁
		rn.Destroy()
		//重新加载
		rn.Copy(ruleNodeCtx.(*RuleNodeCtx))
		return nil
	} else {
		return err
	}
}

func (rn *RuleNodeCtx) ReloadChild(_ types.RuleNodeId, _ []byte) error {
	return errors.New("not support this func")
}

func (rn *RuleNodeCtx) GetNodeById(_ types.RuleNodeId) (types.NodeCtx, bool) {
	return nil, false
}

func (rn *RuleNodeCtx) DSL() []byte {
	v, _ := rn.Config.Parser.EncodeRuleNode(rn.SelfDefinition)
	return v
}

// Copy 复制
func (rn *RuleNodeCtx) Copy(newCtx *RuleNodeCtx) {
	rn.Node = newCtx.Node
	rn.SelfDefinition = newCtx.SelfDefinition
}
