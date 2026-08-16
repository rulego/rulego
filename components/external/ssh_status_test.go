package external

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"golang.org/x/crypto/ssh"
)

func TestIsSshCmdError(t *testing.T) {
	assert.True(t, isSshCmdError(&ssh.ExitError{}))
	assert.True(t, isSshCmdError(fmt.Errorf("run: %w", &ssh.ExitMissingError{})))
	assert.False(t, isSshCmdError(errors.New("read tcp 1.2.3.4:22: connection reset")))
	assert.False(t, isSshCmdError(nil))
}

func TestSshNodeNotInit(t *testing.T) {
	node := &SshNode{}
	err := node.Init(types.NewConfig(), types.Configuration{})
	assert.Equal(t, SshConfigEmptyErr.Error(), err.Error())
	ctx := test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {
		assert.Equal(t, types.Failure, relationType)
		assert.Equal(t, SshClientNotInitErr.Error(), err.Error())
	})
	node.OnMsg(ctx, types.RuleMsg{})
}

func TestSshNodeConnectionStatusUnreachable(t *testing.T) {
	node := &SshNode{}
	config := types.NewConfig()
	// 懒初始化：目标机不可达不影响 Init
	err := node.Init(config, types.Configuration{
		"host":     "127.0.0.1",
		"port":     1,
		"username": "root",
		"password": "password",
		"cmd":      "echo test",
	})
	assert.Nil(t, err)
	assert.Equal(t, types.StatusNone, node.ConnectionStatus().Status)

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
		assert.Equal(t, types.Failure, relationType)
		assert.True(t, err != nil)
	})
	node.OnMsg(ctx, ctx.NewMsg("AA", types.NewMetadata(), ""))

	info := node.ConnectionStatus()
	assert.Equal(t, types.StatusReconnecting, info.Status)
	assert.True(t, info.Message != "")

	node.Destroy()
	assert.Equal(t, types.StatusDisconnected, node.ConnectionStatus().Status)
}
