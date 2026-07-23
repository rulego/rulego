// Package Logger provides a configuration-based log factory, supporting console, file output, and log rotation.
package logger

import (
	"io"
	"os"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewFromConfig Create types.Logger based on config.
//   - LogFile is empty: console only
//   - LogFile with value: Console + Files (Lumberjack rotation)
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
