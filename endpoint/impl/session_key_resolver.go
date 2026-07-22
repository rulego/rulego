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

// SessionKeyResolver 用 rulego ${} 表达式提取 sessionKey。配置为 string 或 []string（多候选按序取首个非空）。
// env 含 msg/metadata/data，注入 hex、reFind 辅助函数（expr 不内置）。
//
// 表达式示例：
//
//	${msg.deviceId}              JSON 字段
//	${msg.header.sn}             嵌套字段
//	${metadata.deviceId}         metadata
//	${data[4:14]}                原始字节切片
//	${hex(data[4:14])}           切片后转十六进制
//	${reFind("ID:([a-zA-Z0-9_]+)", data)}  正则提取捕获组（双引号；正则避免 \w 用 [a-zA-Z0-9_]）
type SessionKeyResolver struct {
	templates []el.Template
}

// NewSessionKeyResolver 把配置归一化为 el.Template 列表，编译失败的候选静默跳过。
func NewSessionKeyResolver(cfg interface{}) *SessionKeyResolver {
	r := &SessionKeyResolver{}
	for _, raw := range toStringSlice(cfg) {
		if t, err := el.NewTemplate(raw); err == nil && t != nil {
			r.templates = append(r.templates, t)
		}
	}
	return r
}

// Resolve 按候选顺序执行表达式，返回首个非空结果。data 为原始上行字节（供 ${data[...]} 切片与 reFind）。
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

// buildSessionEnv 构造表达式环境（msg/metadata/data），注入 hex、reFind 辅助函数。
func buildSessionEnv(msg types.RuleMsg, data []byte) map[string]any {
	env := map[string]any{
		"id":       msg.Id,
		"ts":       msg.Ts,
		"msgType":  msg.Type,
		"dataType": string(msg.DataType),
		"data":     string(data),
	}
	// msg：优先解析原始字节 data（net 场景 dataType 常为 BINARY 但帧是 JSON），其次解析 msg 自身
	var jsonMap interface{}
	if len(data) > 0 && json.Unmarshal(data, &jsonMap) == nil {
		env["msg"] = jsonMap
	} else if jsonData, err := msg.GetJsonData(); err == nil {
		env["msg"] = jsonData
	} else {
		env["msg"] = msg.GetData()
	}
	// metadata：放入 env["metadata"] 并平铺（兼容 ${metadata.x} 与 ${x}）
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

// reCache 缓存已编译的正则，避免高频调用时重复编译。
var reCache sync.Map

// regexFind 正则匹配，返回首个捕获组（无分组返回整条匹配），未匹配返回空串。
// 编译结果按 pattern 缓存，同 pattern 只编译一次。
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

// toStringSlice 归一化配置（nil/string/[]string/[]interface{}）为 []string。
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
