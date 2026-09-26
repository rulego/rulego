/*
 * Copyright 2025 The RuleGo Authors.
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

package cast

import (
	"fmt"
	"testing"
	"time"
)

func TestToInt(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect int
		hasErr bool
	}{
		{"int", 123, 123, false},
		{"int8", int8(123), 123, false},
		{"int16", int16(123), 123, false},
		{"int32", int32(123), 123, false},
		{"int64", int64(123), 123, false},
		{"uint", uint(123), 123, false},
		{"uint8", uint8(123), 123, false},
		{"uint16", uint16(123), 123, false},
		{"uint32", uint32(123), 123, false},
		{"uint64", uint64(123), 123, false},
		{"float64", 1.1, 1, false},
		{"float64", float32(1.1), 1, false},
		{"string", "123", 123, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1, 2, 3}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToInt(tt.input)
			if got != tt.expect {
				t.Errorf("ToInt() = %v, want %v", got, tt.expect)
			}

			_, err := ToIntE(tt.input)
			if (err != nil) != tt.hasErr {
				t.Errorf("ToIntE() error = %v, wantErr %v", err, tt.hasErr)
			}
		})
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect int64
		hasErr bool
	}{
		{"int", 123, 123, false},
		{"int8", int8(123), 123, false},
		{"int16", int16(123), 123, false},
		{"int32", int32(123), 123, false},
		{"int64", int64(123), 123, false},
		{"uint", uint(123), 123, false},
		{"uint8", uint8(123), 123, false},
		{"uint16", uint16(123), 123, false},
		{"uint32", uint32(123), 123, false},
		{"uint64", uint64(123), 123, false},
		{"float64", 1.1, 1, false},
		{"float32", float32(1.1), 1, false},
		{"string", "123", 123, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1, 2, 3}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToInt64(tt.input)
			if got != tt.expect {
				t.Errorf("ToInt64() = %v, want %v", got, tt.expect)
			}

			_, err := ToInt64E(tt.input)
			if (err != nil) != tt.hasErr {
				t.Errorf("ToInt64E() error = %v, wantErr %v", err, tt.hasErr)
			}
		})
	}
}

func TestConvertIntToTime(t *testing.T) {
	base := time.Unix(1700000000, 0)
	tests := []struct {
		name      string
		timestamp int64
		unit      time.Duration
		expect    time.Time
	}{
		{"second", 1700000000, time.Second, base},
		{"millisecond", 1700000000000, time.Millisecond, base},
		{"microsecond", 1700000000000000, time.Microsecond, base},
		{"nanosecond", 1700000000000000000, time.Nanosecond, base},
		// unknown units fall back to seconds
		{"default minute", 1700000000, time.Minute, base},
		{"default hour", 1700000000, time.Hour, base},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertIntToTime(tt.timestamp, tt.unit)
			if !got.Equal(tt.expect) {
				t.Errorf("ConvertIntToTime() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestToDurationE(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect time.Duration
		hasErr bool
	}{
		{"duration", time.Second, time.Second, false},
		{"int", 1000, 1000, false},
		{"int8", int8(100), 100, false},
		{"int16", int16(1000), 1000, false},
		{"int32", int32(1000), 1000, false},
		{"int64", int64(1000), 1000, false},
		{"uint", uint(1000), 1000, false},
		{"uint8", uint8(100), 100, false},
		{"uint16", uint16(1000), 1000, false},
		{"uint32", uint32(1000), 1000, false},
		{"uint64", uint64(1000), 1000, false},
		{"string", "1s", time.Second, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1, 2, 3}, 0, true},
		{"float64 falls to default", 1.5, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dur, err := ToDurationE(tt.input)
			if (err != nil) != tt.hasErr {
				t.Errorf("ToDurationE() error = %v, wantErr %v", err, tt.hasErr)
			}
			if !tt.hasErr && dur != tt.expect {
				t.Errorf("ToDurationE() = %v, want %v", dur, tt.expect)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect bool
		hasErr bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"int 1", 1, true, false},
		{"int 0", 0, false, false},
		{"float64 1.0", 1.0, true, false},
		{"float64 0.0", 0.0, false, false},
		{"string true", "true", true, false},
		{"string false", "false", false, false},
		{"string 1", "1", true, false},
		{"string 0", "0", false, false},
		{"invalid string", "abc", false, true},
		{"invalid type", []int{1, 2, 3}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToBool(tt.input)
			if got != tt.expect {
				t.Errorf("ToBool() = %v, want %v", got, tt.expect)
			}

			_, err := ToBoolE(tt.input)
			if (err != nil) != tt.hasErr {
				t.Errorf("ToBoolE() error = %v, wantErr %v", err, tt.hasErr)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect float64
		hasErr bool
	}{
		{"float64", 3.14, 3.14, false},
		{"float32", float32(3.14), float64(float32(3.14)), false},
		{"int", 123, 123.0, false},
		{"int8", int8(123), 123.0, false},
		{"int16", int16(123), 123.0, false},
		{"int32", int32(123), 123.0, false},
		{"int64", int64(123), 123.0, false},
		{"uint", uint(123), 123.0, false},
		{"uint8", uint8(123), 123.0, false},
		{"uint16", uint16(123), 123.0, false},
		{"uint32", uint32(123), 123.0, false},
		{"uint64", uint64(123), 123.0, false},
		{"string", "3.14", 3.14, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1, 2, 3}, 0, true},
		{"duration is not a float", time.Second, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToFloat64(tt.input)
			if got != tt.expect {
				t.Errorf("ToFloat64() = %v, want %v", got, tt.expect)
			}

			_, err := ToFloat64E(tt.input)
			if (err != nil) != tt.hasErr {
				t.Errorf("ToFloat64E() error = %v, wantErr %v", err, tt.hasErr)
			}
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect string
		hasErr bool
	}{
		{"nil", nil, "", false},
		{"string", "test", "test", false},
		{"bool true", true, "true", false},
		{"bool false", false, "false", false},
		{"int", 123, "123", false},
		{"int8", int8(123), "123", false},
		{"int16", int16(123), "123", false},
		{"int32", int32(123), "123", false},
		{"int64", int64(123), "123", false},
		{"uint", uint(123), "123", false},
		{"uint8", uint8(123), "123", false},
		{"uint16", uint16(123), "123", false},
		{"uint32", uint32(123), "123", false},
		{"uint64", uint64(123), "123", false},
		{"float64", 3.14, "3.14", false},
		{"float32", float32(3.14), "3.14", false},
		{"[]byte", []byte("test"), "test", false},
		{"error", fmt.Errorf("test error"), "test error", false},
		{"stringer", time.Hour, "1h0m0s", false},
		{"map", map[string]int{"a": 1}, "{\"a\":1}", false},
		{"map with interface keys", map[interface{}]interface{}{"k": "v"}, "{\"k\":\"v\"}", false},
		{"invalid type", make(chan int), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToString(tt.input)
			if got != tt.expect {
				t.Errorf("ToString() = %v, want %v", got, tt.expect)
			}

			_, err := ToStringE(tt.input)
			if (err != nil) != tt.hasErr {
				t.Errorf("ToStringE() error = %v, wantErr %v", err, tt.hasErr)
			}
		})
	}
}
