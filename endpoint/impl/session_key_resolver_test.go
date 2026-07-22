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
	"testing"

	"github.com/rulego/rulego/api/types"
)

func jsonMsg(s string) types.RuleMsg {
	return types.NewMsg(0, "", types.JSON, types.NewMetadata(), s)
}

// binaryMsg 构造一个非 JSON 的占位 msg（${data[...]} 类测试只用 data 参数）
func binaryMsg() types.RuleMsg {
	return types.NewMsg(0, "", types.BINARY, types.NewMetadata(), "")
}

func TestResolveMsgField(t *testing.T) {
	r := NewSessionKeyResolver("${msg.deviceId}")
	got := r.Resolve(jsonMsg(`{"deviceId":"DEV_001","temp":26}`), nil)
	if got != "DEV_001" {
		t.Fatalf("got %q, want DEV_001", got)
	}
}

func TestResolveMsgNested(t *testing.T) {
	r := NewSessionKeyResolver("${msg.header.sn}")
	got := r.Resolve(jsonMsg(`{"header":{"sn":"SN99"}}`), nil)
	if got != "SN99" {
		t.Fatalf("got %q, want SN99", got)
	}
}

func TestResolveMsgMissing(t *testing.T) {
	r := NewSessionKeyResolver("${msg.deviceId}")
	if got := r.Resolve(jsonMsg(`{"temp":26}`), nil); got != "" {
		t.Fatalf("got %q, want empty (field missing)", got)
	}
}

func TestResolveMetadata(t *testing.T) {
	m := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{}`)
	m.Metadata.PutValue("deviceId", "M_DEV")
	r := NewSessionKeyResolver("${metadata.deviceId}")
	if got := r.Resolve(m, nil); got != "M_DEV" {
		t.Fatalf("got %q, want M_DEV", got)
	}
}

// expr 原生字符串切片（byte 级）
func TestResolveDataSlice(t *testing.T) {
	r := NewSessionKeyResolver("${data[4:11]}")
	got := r.Resolve(binaryMsg(), []byte("XXXXDEV_001_YY"))
	if got != "DEV_001" {
		t.Fatalf("got %q, want DEV_001", got)
	}
}

// 切片 + 注入的 hex 函数
func TestResolveHex(t *testing.T) {
	r := NewSessionKeyResolver("${hex(data[2:4])}")
	got := r.Resolve(binaryMsg(), []byte{0x00, 0x00, 0xAB, 0xCD, 0xFF})
	if got != "abcd" {
		t.Fatalf("got %q, want abcd", got)
	}
}

// 注入的 reFind 函数：返回捕获组
func TestResolveReFindGroup(t *testing.T) {
	r := NewSessionKeyResolver(`${reFind("ID:([A-Z0-9]+)", data)}`)
	got := r.Resolve(binaryMsg(), []byte("log ID:DEV001 end"))
	if got != "DEV001" {
		t.Fatalf("got %q, want DEV001", got)
	}
}

// reFind 无分组 → 返回整条匹配
func TestResolveReFindNoGroup(t *testing.T) {
	r := NewSessionKeyResolver(`${reFind("DEV_[0-9]+", data)}`)
	got := r.Resolve(binaryMsg(), []byte("log DEV_001 end"))
	if got != "DEV_001" {
		t.Fatalf("got %q, want DEV_001", got)
	}
}

// 多候选跨类型回退：JSON miss → 字节切片
func TestResolveMultiCandidateFallback(t *testing.T) {
	r := NewSessionKeyResolver([]string{"${msg.deviceId}", "${data[4:10]}"})
	// 帧1：JSON 有 deviceId
	if got := r.Resolve(jsonMsg(`{"deviceId":"JSON_DEV"}`), nil); got != "JSON_DEV" {
		t.Fatalf("frame1 got %q, want JSON_DEV", got)
	}
	// 帧2：JSON 无 deviceId（返回空），回退 data[4:10]
	if got := r.Resolve(jsonMsg(`{"x":1}`), []byte("XXXXHEXDEV_")); got != "HEXDEV" {
		t.Fatalf("frame2 got %q, want HEXDEV", got)
	}
}

// 无效候选（表达式编译失败）被跳过，不影响后续候选
func TestResolveInvalidCandidateSkipped(t *testing.T) {
	r := NewSessionKeyResolver([]string{"${this is invalid !!!}", "${msg.deviceId}"})
	got := r.Resolve(jsonMsg(`{"deviceId":"FALLBACK"}`), nil)
	if got != "FALLBACK" {
		t.Fatalf("got %q, want FALLBACK (bad expr skipped)", got)
	}
}

func TestResolveEmptyConfig(t *testing.T) {
	r := NewSessionKeyResolver(nil)
	if got := r.Resolve(jsonMsg(`{"deviceId":"X"}`), nil); got != "" {
		t.Fatalf("got %q, want empty for nil config", got)
	}
}

// 验证 el 模板只编译一次：多次 Resolve 复用（间接证明）
func TestResolveReuseAcrossCalls(t *testing.T) {
	r := NewSessionKeyResolver(`${reFind("DEV_[0-9]+", data)}`)
	for i := 0; i < 3; i++ {
		if got := r.Resolve(binaryMsg(), []byte("DEV_001")); got != "DEV_001" {
			t.Fatalf("call %d got %q, want DEV_001", i, got)
		}
	}
}
