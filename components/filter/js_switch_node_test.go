package filter

import (
	"rulego/api/types"
	"rulego/test"
	"rulego/test/assert"
	"testing"
)

func TestJsSwitchNodeOnMsg(t *testing.T) {
	var node JsSwitchNode
	var configuration = make(types.Configuration)
	configuration["jsScript"] = `
		//测试注释
		return ['one','two'];
  	`
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	var i = 0
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		if i == 0 {
			assert.Equal(t, "one", relationType)
		} else if i == 1 {
			assert.Equal(t, "two", relationType)
		}
		i++

	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("ACTIVITY_EVENT", metaData, "AA")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}

}
