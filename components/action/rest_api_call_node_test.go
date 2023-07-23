package action

import (
	"rulego/api/types"
	"rulego/test"
	"rulego/test/assert"
	"testing"
)

func TestRestApiCallNodeOnMsg(t *testing.T) {
	var node RestApiCallNode
	var configuration = make(types.Configuration)
	configuration["restEndpointUrlPattern"] = "https://gitee.com"
	configuration["requestMethod"] = "POST"
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		code, ok := msg.Metadata.GetValue(statusCode)
		assert.Equal(t, "404", code)
		assert.True(t, ok)
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "{\"test\":\"AA\"}")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}

}
