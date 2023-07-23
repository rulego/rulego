package filter

import (
	"rulego/api/types"
	"rulego/test"
	"rulego/test/assert"
	"testing"
)

func TestJsFilterNodeOnMsg(t *testing.T) {
	var node JsFilterNode
	var configuration = make(types.Configuration)
	configuration["jsScript"] = `
		//测试注释
		return msg=='AA';
  	`
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		if msg.Type == "TEST_MSG_TYPE_AA" {
			assert.Equal(t, "True", relationType)
		} else if msg.Type == "TEST_MSG_TYPE_BB" {
			assert.Equal(t, "False", relationType)
		}

	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "AA")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, "BB")
	err = node.OnMsg(ctx, msg2)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}
