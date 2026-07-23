package logger

import "log"

// DefaultLogger is the default loglogger wrapper and implements types.Logger interface
type DefaultLogger struct {
	*log.Logger
}

// Debugf records logs at the debug level
func (l *DefaultLogger) Debugf(format string, v ...interface{}) {
	l.Printf("[DEBUG] "+format, v...)
}

// Infof records information at the level of logs
func (l *DefaultLogger) Infof(format string, v ...interface{}) {
	l.Printf("[INFO] "+format, v...)
}

// Warnf logs warning levels
func (l *DefaultLogger) Warnf(format string, v ...interface{}) {
	l.Printf("[WARN] "+format, v...)
}

// Errorf logs the error level
func (l *DefaultLogger) Errorf(format string, v ...interface{}) {
	l.Printf("[ERROR] "+format, v...)
}

// Logger exposes log instances to the outside
var Logger *DefaultLogger

// Set the global log instance
func Set(logger *log.Logger) {
	Logger = &DefaultLogger{logger}
}

// Get the global log instance
func Get() *DefaultLogger {
	return Logger
}
