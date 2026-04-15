package logger

import "log"

// DefaultLogger 默认日志记录器包装，实现了 types.Logger 接口
type DefaultLogger struct {
	*log.Logger
}

// Debugf 记录调试级别的日志
func (l *DefaultLogger) Debugf(format string, v ...interface{}) {
	l.Printf("[DEBUG] "+format, v...)
}

// Infof 记录信息级别的日志
func (l *DefaultLogger) Infof(format string, v ...interface{}) {
	l.Printf("[INFO] "+format, v...)
}

// Warnf 记录警告级别的日志
func (l *DefaultLogger) Warnf(format string, v ...interface{}) {
	l.Printf("[WARN] "+format, v...)
}

// Errorf 记录错误级别的日志
func (l *DefaultLogger) Errorf(format string, v ...interface{}) {
	l.Printf("[ERROR] "+format, v...)
}

// Logger 暴露给外部的日志实例
var Logger *DefaultLogger

// Set 设置全局日志实例
func Set(logger *log.Logger) {
	Logger = &DefaultLogger{logger}
}

// Get 获取全局日志实例
func Get() *DefaultLogger {
	return Logger
}
