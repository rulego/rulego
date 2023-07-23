package rulego

import (
	"rulego/api/types"
	string2 "rulego/utils/json"
)

//JsonParser Json
type JsonParser struct {
}

func (p *JsonParser) DecodeRuleChain(config types.Config, dsl []byte) (types.Node, error) {
	if rootRuleChainDef, err := ParserRuleChain(dsl); err == nil {
		//初始化
		return InitRuleChainCtx(config, rootRuleChainDef)
	} else {
		return nil, err
	}
}
func (p *JsonParser) DecodeRuleNode(config types.Config, dsl []byte) (types.Node, error) {
	if node, err := ParserRuleNode(dsl); err == nil {
		return InitRuleNodeCtx(config, node)
	} else {
		return nil, err
	}
}
func (p *JsonParser) EncodeRuleChain(def interface{}) ([]byte, error) {
	return string2.Marshal(def)
}
func (p *JsonParser) EncodeRuleNode(def interface{}) ([]byte, error) {
	return string2.Marshal(def)
}
