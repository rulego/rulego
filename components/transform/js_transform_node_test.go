package transform

import (
	"rulego/api/types"
	"rulego/test"
	"rulego/test/assert"
	"testing"
)

func TestJsTransformNodeOnMsg(t *testing.T) {
	var node JsTransformNode
	var configuration = make(types.Configuration)
	configuration["jsScript"] = `
		metadata['test']='test02';
		metadata['index']=52;
		msgType='TEST_MSG_TYPE2';
		var msg2={};
		msg2['bb']=22
		return {'msg':msg2,'metadata':metadata,'msgType':msgType};
  	`
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		assert.Equal(t, "{\"bb\":22}", msg.Data)
		assert.Equal(t, "TEST_MSG_TYPE2", msg.Type)
		assert.Equal(t, types.Success, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "AA")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}

}
