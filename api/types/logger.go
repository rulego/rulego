package types

import (
	"log"
	"os"
)

type Logger interface {
	Printf(format string, v ...interface{})
}

// this is a safeguard, breaking on compile time in case
// `log.Logger` does not adhere to our `Logger` interface.
// see https://golang.org/doc/faq#guarantee_satisfies_interface
var _ Logger = &log.Logger{}

// DefaultLogger returns a `Logger` implementation
func DefaultLogger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags)
}

func NewLogger(custom Logger) Logger {
	if custom != nil {
		return custom
	}

	return DefaultLogger()
}
