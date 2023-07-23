package filter

//规则链节点配置示例：
//{
//        "id": "s2",
//        "type": "msgTypeSwitch",
//        "name": "消息路由"
//      }
import (
	"rulego/api/types"
)

func init() {
	Registry.Add(&MsgTypeSwitchNode{})
}

//MsgTypeSwitchNode 根据传入的消息类型路由到一个或多个输出链
//把消息通过类型发到正确的链,
type MsgTypeSwitchNode struct {
}

//Type 组件类型
func (x *MsgTypeSwitchNode) Type() string {
	return "msgTypeSwitch"
}

func (x *MsgTypeSwitchNode) New() types.Node {
	return &MsgTypeSwitchNode{}
}

//Init 初始化
func (x *MsgTypeSwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

//OnMsg 处理消息
func (x *MsgTypeSwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	ctx.TellNext(msg, msg.Type)
	return nil
}

//Destroy 销毁
func (x *MsgTypeSwitchNode) Destroy() {
}
