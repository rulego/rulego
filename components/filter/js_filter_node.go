package filter

//规则链节点配置示例：
//{
//        "id": "s2",
//        "type": "jsFilter",
//        "name": "过滤",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "var msg2=JSON.parse(msg);return msg2.temperature > 50;"
//        }
//      }
import (
	"fmt"
	"rulego/api/types"
	"rulego/components/js"
	"rulego/utils/maps"
)

func init() {
	Registry.Add(&JsFilterNode{})
}

//JsFilterNodeConfiguration 节点配置
type JsFilterNodeConfiguration struct {
	//JsScript 配置函数体脚本内容
	// 使用js脚本进行过滤
	//完整脚本函数：
	//function Filter(msg, metadata, msgType) { ${JsScript} }
	//return bool
	JsScript string
}

//JsFilterNode 使用js脚本过滤传入信息
//如果 `True`发送信息到`True`链, `False`发到`False`链。
//如果 脚本执行失败则发送到`Failure`链
//消息体可以通过`msg`变量访问，msg 是string类型。例如:`var msg2=JSON.parse(msg);return msg2.temperature > 50;`
//消息元数据可以通过`metadata`变量访问。例如 `metadata.customerName === 'Lala';`
//消息类型可以通过`msgType`变量访问.
type JsFilterNode struct {
	config   JsFilterNodeConfiguration
	jsEngine types.JsEngine
}

//Type 组件类型
func (x *JsFilterNode) Type() string {
	return "jsFilter"
}

func (x *JsFilterNode) New() types.Node {
	return &JsFilterNode{}
}

//Init 初始化
func (x *JsFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.config)
	if err == nil {
		jsScript := fmt.Sprintf("function Filter(msg, metadata, msgType) { %s }", x.config.JsScript)
		x.jsEngine = js.NewGojaJsEngine(ruleConfig, jsScript, nil)
	}
	return err
}

//OnMsg 处理消息
func (x *JsFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	out, err := x.jsEngine.Execute("Filter", msg.Data, msg.Metadata.Values(), msg.Type)
	if err != nil {
		ctx.TellFailure(msg, err)
		return err
	} else {
		if formatData, ok := out.(bool); ok && formatData {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
		return nil
	}
}

//Destroy 销毁
func (x *JsFilterNode) Destroy() {
	x.jsEngine.Stop()
}
