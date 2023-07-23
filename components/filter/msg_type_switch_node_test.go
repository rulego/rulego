package filter

import (
	"rulego/api/types"
	"rulego/test"
	"rulego/test/assert"
	"testing"
)

func TestMsgTypeSwitchNodeOnMsg(t *testing.T) {
	var node MsgTypeSwitchNode
	config := types.NewConfig()
	err := node.Init(config, nil)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string) {
		if msg.Type == "ACTIVITY_EVENT" {
			assert.Equal(t, "ACTIVITY_EVENT", relationType)
		} else if msg.Type == "INACTIVITY_EVENT" {
			assert.Equal(t, "INACTIVITY_EVENT", relationType)
		}

	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg := ctx.NewMsg("ACTIVITY_EVENT", metaData, "AA")
	err = node.OnMsg(ctx, msg)
	if err != nil {
		t.Errorf("err=%s", err)
	}

	msg2 := ctx.NewMsg("INACTIVITY_EVENT", metaData, "BB")
	err = node.OnMsg(ctx, msg2)
	if err != nil {
		t.Errorf("err=%s", err)
	}
}
