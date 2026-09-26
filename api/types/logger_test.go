/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rulego/rulego/test/assert"
)

// TestLogLevelString 日志级别字符串表示
func TestLogLevelString(t *testing.T) {
	cases := []struct {
		level LogLevel
		want  string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{LogLevel(9), "UNKNOWN"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.level.String())
	}
}

// TestField 结构化字段快捷构造
func TestField(t *testing.T) {
	f := F("key", 42)
	assert.Equal(t, "key", f.Key)
	assert.Equal(t, 42, f.Value)
}

// TestStdLogger 级别过滤、输出目标切换与各级别方法
func TestStdLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf)
	assert.Equal(t, InfoLevel, logger.GetLevel())

	// 默认 INFO：Debugf 被过滤，其余输出
	logger.Debugf("hidden %d", 1)
	logger.Printf("printf %d", 2)
	logger.Infof("info %d", 3)
	logger.Warnf("warn %d", 4)
	logger.Errorf("error %d", 5)
	out := buf.String()
	assert.False(t, strings.Contains(out, "hidden"))
	assert.True(t, strings.Contains(out, "[INFO] printf 2"))
	assert.True(t, strings.Contains(out, "[INFO] info 3"))
	assert.True(t, strings.Contains(out, "[WARN] warn 4"))
	assert.True(t, strings.Contains(out, "[ERROR] error 5"))

	logger.SetLevel(DebugLevel)
	assert.Equal(t, DebugLevel, logger.GetLevel())
	logger.Debugf("shown")
	assert.True(t, strings.Contains(buf.String(), "[DEBUG] shown"))

	var buf2 bytes.Buffer
	logger.SetOutput(&buf2)
	logger.Infof("redirected")
	assert.True(t, strings.Contains(buf2.String(), "redirected"))
	assert.False(t, strings.Contains(buf.String(), "redirected"))

	assert.NotNil(t, NewStdLogger(nil))
}

// TestLoggerFactories 工厂函数与空日志器判定
func TestLoggerFactories(t *testing.T) {
	assert.NotNil(t, DefaultLogger())

	std := NewStdLogger(nil)
	assert.Equal(t, std, NewLogger(std))
	assert.NotNil(t, NewLogger(nil))

	assert.True(t, IsNilLogger(nil))
	assert.False(t, IsNilLogger(std))
}
