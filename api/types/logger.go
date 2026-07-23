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
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// ============================================
// log level
// ============================================

// LogLevel
type LogLevel int8

const (
	// DebugLevel
	DebugLevel LogLevel = iota - 1
	// InfoLevel
	InfoLevel
	// WarnLevel warning level
	WarnLevel
	// ErrorLevel
	ErrorLevel
)

// String returns a log-level string representation
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ============================================
// Field Structured Fields (optional extension)
// ============================================

// Field: Structured log fields
type Field struct {
	Key   string
	Value any
}

// F. A quick way to create log fields
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// ============================================
// Logger interface
// ============================================

// Logger log interface
// The application layer needs to implement this interface to access its own log framework
type Logger interface {
	// Printf is compatible with older interfaces
	// Deprecated: Please use Debugf/Infof/Warnf/Errorf instead
	Printf(format string, v ...interface{})
	// Debugf debugging log
	Debugf(format string, v ...interface{})
	// Infof Information Log
	Infof(format string, v ...interface{})
	// Warnf warning log
	Warnf(format string, v ...interface{})
	// Errorf error log
	Errorf(format string, v ...interface{})
}

// ============================================
// Implemented by default
// ============================================

// Make sure StdLogger implements the Logger interface
var _ Logger = (*StdLogger)(nil)

// Ensure the standard library log.Logger is compatible with the Printf interface
var _ interface{ Printf(string, ...interface{}) } = (*log.Logger)(nil)

// StdLogger is implemented by default
type StdLogger struct {
	mu    sync.Mutex
	w     io.Writer
	level LogLevel
}

// NewStdLogger creates a standard logger
func NewStdLogger(w io.Writer) *StdLogger {
	if w == nil {
		w = os.Stdout
	}
	return &StdLogger{w: w, level: InfoLevel}
}

// SetLevel sets the log level
func (l *StdLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

// GetLevel Retrieves log levels
func (l *StdLogger) GetLevel() LogLevel {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// SetOutput sets the output target
func (l *StdLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	l.w = w
	l.mu.Unlock()
}

func (l *StdLogger) output(level LogLevel, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if level < l.level {
		return
	}
	fmt.Fprintf(l.w, "[%s] %s\n", level, fmt.Sprintf(format, v...))
}

// Printf implements the Logger interface
func (l *StdLogger) Printf(format string, v ...interface{}) {
	l.output(InfoLevel, format, v...)
}

// Debugf debugging log
func (l *StdLogger) Debugf(format string, v ...interface{}) {
	l.output(DebugLevel, format, v...)
}

// Infof Information Log
func (l *StdLogger) Infof(format string, v ...interface{}) {
	l.output(InfoLevel, format, v...)
}

// Warnf warning log
func (l *StdLogger) Warnf(format string, v ...interface{}) {
	l.output(WarnLevel, format, v...)
}

// Errorf error log
func (l *StdLogger) Errorf(format string, v ...interface{}) {
	l.output(ErrorLevel, format, v...)
}

// ============================================
// Factory function
// ============================================

// DefaultLogger returns the default logger
func DefaultLogger() Logger {
	return NewStdLogger(os.Stdout)
}

// NewLogger creates a logger
func NewLogger(custom Logger) Logger {
	if custom != nil {
		return custom
	}
	return DefaultLogger()
}

// IsNilLogger determines whether the logger is empty or if it uses an empty implementation
// Used for judgment before printing logs, avoiding empty pointers
func IsNilLogger(logger Logger) bool {
	return logger == nil
}
