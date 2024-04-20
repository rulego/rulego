package logger

import "log"

var Logger *log.Logger

func Set(logger *log.Logger) {
	Logger = logger
}

func Get() *log.Logger {
	return Logger
}
