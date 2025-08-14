package test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// TestExtendedTestRuleContext 测试 ExtendedTestRuleContext 的基本功能
func TestExtendedTestRuleContext(t *testing.T) {
	ctx := NewExtendedTestRuleContextWithChannel()

	// 测试设置节点处理器
	ctx.SetNodeHandler("test1", func(msg types.RuleMsg) (string, error) {
		return "Success", nil
	})

	ctx.SetNodeHandler("test2", func(msg types.RuleMsg) (string, error) {
		return "Failure", nil
	})

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	// 测试 TellNode 功能，使用回调来收集结果
	var wg sync.WaitGroup
	wg.Add(2)

	ctx.TellNode(context.Background(), "test1", msg, false, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		ctx.TellNext(msg, relationType)
		wg.Done()
	}, nil)

	ctx.TellNode(context.Background(), "test2", msg, false, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		ctx.TellNext(msg, relationType)
		wg.Done()
	}, nil)

	// 等待异步处理完成
	wg.Wait()

	// 验证结果
	results := ctx.GetResults()
	assert.Equal(t, 2, len(results), "Should have 2 results")
	
	// 验证结果包含期望的值（不依赖顺序）
	resultMap := make(map[string]bool)
	for _, result := range results {
		resultMap[result] = true
	}
	assert.True(t, resultMap["Success"], "Results should contain Success")
	assert.True(t, resultMap["Failure"], "Results should contain Failure")

	// 测试通道功能 - 收集所有结果
	channelResults := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case result := <-ctx.GetResultsChannel():
			channelResults[result.RelationType] = true
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Should receive result %d from channel", i+1)
		}
	}
	assert.True(t, channelResults["Success"], "Channel should receive Success")
	assert.True(t, channelResults["Failure"], "Channel should receive Failure")
}

// TestExtendedTestRuleContextTellMethods 测试 Tell 方法
func TestExtendedTestRuleContextTellMethods(t *testing.T) {
	ctx := NewExtendedTestRuleContextWithChannel()
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	// 测试 TellSuccess
	ctx.TellSuccess(msg)

	// 测试 TellFailure
	ctx.TellFailure(msg, errors.New("test error"))

	// 测试 TellNext
	ctx.TellNext(msg, "Custom")

	// 验证结果
	results := ctx.GetResults()
	assert.Equal(t, 3, len(results), "Should have 3 results")
	assert.Equal(t, "Success", results[0], "First result should be Success")
	assert.Equal(t, "Failure", results[1], "Second result should be Failure")
	assert.Equal(t, "Custom", results[2], "Third result should be Custom")

	// 验证通道结果
	for i := 0; i < 3; i++ {
		select {
		case result := <-ctx.GetResultsChannel():
			switch i {
			case 0:
				assert.Equal(t, "Success", result.RelationType, "Should receive Success")
				assert.Nil(t, result.Err, "Success should have no error")
			case 1:
				assert.Equal(t, "Failure", result.RelationType, "Should receive Failure")
				assert.NotNil(t, result.Err, "Failure should have error")
			case 2:
				assert.Equal(t, "Custom", result.RelationType, "Should receive Custom")
				assert.Nil(t, result.Err, "Custom should have no error")
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Should receive result %d from channel", i)
		}
	}
}