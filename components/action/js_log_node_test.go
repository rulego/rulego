package action

import (
	"rulego/api/types"
	"rulego/test"
	"rulego/test/assert"
	"testing"
)

func TestJsTransformNodeOnMsg(t *testing.T) {
	var node LogNode
	var configuration = make(types.Configuration)
	configuration["jsScript"] = `
		//测试注释
		return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
  	`
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		assert.Equal(t, types.Success, relationType)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("productType", "test")
	msg := ctx.NewMsg("ACTIVITY_EVENT", metaData, "AA")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}
