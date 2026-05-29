// Package logger 提供基于配置的日志工厂，支持控制台、文件输出和日志轮转。
package logger

import (
	"io"
	"os"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewFromConfig 根据 config 创建 types.Logger。
//   - LogFile 为空：仅控制台
//   - LogFile 有值：控制台 + 文件（lumberjack 轮转）
func NewFromConfig(cfg *config.Config) types.Logger {
	if cfg == nil {
		return types.DefaultLogger()
	}

	var w io.Writer = os.Stdout
	if cfg.LogFile != "" {
		lw := &lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    cfg.LogMaxSize,
			MaxBackups: cfg.LogMaxBackups,
			MaxAge:     cfg.LogMaxAge,
		}
		w = io.MultiWriter(os.Stdout, lw)
	}

	l := types.NewStdLogger(w)
	l.SetLevel(parseLevel(cfg.LogLevel))
	return l
}

func parseLevel(s string) types.LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return types.DebugLevel
	case "warn", "warning":
		return types.WarnLevel
	case "error":
		return types.ErrorLevel
	default:
		return types.InfoLevel
	}
}
