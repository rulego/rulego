/*
 * Copyright 2024 The RuleGo Authors.
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

package impl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/el"
)

// SessionKeyResolver extracts sessionKey using the rulego ${} expression. Configured as string or []string (multiple candidates sequentially take the first non-empty one).
// env includes msg/metadata/data, and injects hex and reFind auxiliary functions (expr is not built-in).
//
// Example expression:
//
//	${msg.deviceId} JSON field
//	${msg.header.sn}
//	${metadata.deviceId}         metadata
//	${data[4:14]}
//	${hex(data[4:14])}
//	${reFind("ID:([a-zA-Z0-9_]+)", data)}
type SessionKeyResolver struct {
	templates []el.Template
}

// NewSessionKeyResolver normalizes the configuration to el.Template list, silently skipping candidates that fail compile.
func NewSessionKeyResolver(cfg interface{}) *SessionKeyResolver {
	r := &SessionKeyResolver{}
	for _, raw := range toStringSlice(cfg) {
		if t, err := el.NewTemplate(raw); err == nil && t != nil {
			r.templates = append(r.templates, t)
		}
	}
	return r
}

// Resolve executes expressions in candidate order and returns the first non-empty result. data is the original uplink byte (for ${data[...]} slicing and reFind).
func (r *SessionKeyResolver) Resolve(msg types.RuleMsg, data []byte) string {
	if len(r.templates) == 0 {
		return ""
	}
	env := buildSessionEnv(msg, data)
	for _, t := range r.templates {
		if s := t.ExecuteAsString(env); s != "" {
			return s
		}
	}
	return ""
}

// buildSessionEnv constructs the expression environment (msg/metadata/data), injecting hex and reFind auxiliary functions.
func buildSessionEnv(msg types.RuleMsg, data []byte) map[string]any {
	env := map[string]any{
		"id":       msg.Id,
		"ts":       msg.Ts,
		"msgType":  msg.Type,
		"dataType": string(msg.DataType),
		"data":     string(data),
	}
	// msg: prioritizes parsing the original byte data (net scenario dataType is usually BINARY but the frame is JSON), followed by parsing the msg itself
	var jsonMap interface{}
	if len(data) > 0 && json.Unmarshal(data, &jsonMap) == nil {
		env["msg"] = jsonMap
	} else if jsonData, err := msg.GetJsonData(); err == nil {
		env["msg"] = jsonData
	} else {
		env["msg"] = msg.GetData()
	}
	// metadata: Insert env["metadata"] and tile (compatible with ${metadata.x} and ${x})
	if msg.Metadata != nil {
		md := map[string]string{}
		msg.Metadata.ForEach(func(k, v string) bool {
			md[k] = v
			env[k] = v
			return true
		})
		env["metadata"] = md
	}
	env["hex"] = hexEncode
	env["reFind"] = regexFind
	return env
}

func hexEncode(s string) string {
	return fmt.Sprintf("%x", s)
}

// reCache caches compiled regex to avoid repeated compilations during high-frequency calls.
var reCache sync.Map

// regexFind regex matches and returns the first capture group (if no grouping, the entire match is returned); if no match is found, it returns an empty string.
// The compiled result is cached according to the pattern, and the same pattern is compiled only once.
func regexFind(pattern, s string) string {
	var re *regexp.Regexp
	if v, ok := reCache.Load(pattern); ok {
		re = v.(*regexp.Regexp)
	} else {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return ""
		}
		reCache.Store(pattern, re)
	}
	m := re.FindStringSubmatch(s)
	if len(m) == 0 {
		return ""
	}
	if len(m) > 1 {
		return m[1]
	}
	return m[0]
}

// toStringSlice normalized configuration (nil/string/[]string/[]interface{}) is set to []string.
func toStringSlice(cfg interface{}) []string {
	switch v := cfg.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
