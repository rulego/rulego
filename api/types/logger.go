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
