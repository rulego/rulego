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

package assert

import (
	"errors"
	"fmt"
	"testing"
)

type isNilCase struct {
	name  string
	value interface{}
	want  bool
}

func TestIsNil(t *testing.T) {
	var nilMap map[string]string
	var nilSlice []int
	var nilChan chan int
	var nilFunc func()
	var nilPtr *int
	var nilInterface interface{} = nilPtr
	cases := []isNilCase{
		{"nil literal", nil, true},
		{"nil ptr", nilPtr, true},
		{"nil map", nilMap, true},
		{"nil slice", nilSlice, true},
		{"nil chan", nilChan, true},
		{"nil interface holding nil ptr", nilInterface, true},
		{"non-nil int", 42, false},
		{"non-nil string", "s", false},
		{"non-nil struct", struct{}{}, false},
		{"non-nil ptr", &nilPtr, false},
		{"non-nil func", nilFunc, false},
		{"non-nil bool", true, false},
	}
	for _, c := range cases {
		if got := isNil(c.value); got != c.want {
			t.Errorf("isNil(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMessageFromMsgAndArgs(t *testing.T) {
	cases := []struct {
		name       string
		msgAndArgs []interface{}
		want       string
	}{
		{"no args", nil, ""},
		{"single string", []interface{}{"custom message"}, "custom message"},
		{"single non-string", []interface{}{errors.New("boom")}, "boom"},
		{"single struct", []interface{}{struct{ A int }{1}}, "{A:1}"},
		{"format args", []interface{}{"expected %d but got %s", 3, "three"}, "expected 3 but got three"},
	}
	for _, c := range cases {
		got := messageFromMsgAndArgs(c.msgAndArgs...)
		if got != c.want {
			t.Errorf("messageFromMsgAndArgs(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCallerInfo(t *testing.T) {
	callers := CallerInfo()
	if callers == nil {
		t.Errorf("CallerInfo() should never return nil")
	}
}

func TestEqual(t *testing.T) {
	Equal(t, "a", "a")
	Equal(t, 1, 1)
	Equal(t, []string{"a", "b"}, []string{"a", "b"})
	Equal(t, nil, nil)
	Equal(t, map[string]int{"k": 1}, map[string]int{"k": 1})
	Equal(t, 1, 1, "with message")
}

func TestEqualCleanString(t *testing.T) {
	EqualCleanString(t, "a b\nc\td", "abcd")
	EqualCleanString(t, "{\n  \"a\": 1\n}", "{\"a\":1}")
	EqualCleanString(t, "same", "same", "with message")
}

func TestNotEqual(t *testing.T) {
	NotEqual(t, "a", "b")
	NotEqual(t, 1, 2)
	NotEqual(t, nil, "x")
	NotEqual(t, []string{"a"}, []string{"b"})
	NotEqual(t, 1, 2, "with message")
}

func TestTrueFalse(t *testing.T) {
	True(t, true)
	True(t, 1 == 1, "with message")
	False(t, false)
	False(t, 1 != 1, "with message")
}

func TestNilNotNil(t *testing.T) {
	var nilPtr *int
	var nilMap map[string]string

	Nil(t, nil)
	Nil(t, nil, "with message")
	Nil(t, nilPtr)
	Nil(t, nilMap)

	NotNil(t, "value")
	NotNil(t, 0)
	NotNil(t, false)
	NotNil(t, &nilPtr)
	NotNil(t, t, "with message")
}

// failure cases run against a zero-value testing.T so the recorded failure
// does not propagate to the real test result
func TestFailureReporting(t *testing.T) {
	cases := []struct {
		name string
		call func(ft *testing.T)
	}{
		{"Equal", func(ft *testing.T) { Equal(ft, "a", "b") }},
		{"Equal with message", func(ft *testing.T) { Equal(ft, 1, 2, "values differ") }},
		{"Equal with format message", func(ft *testing.T) { Equal(ft, 1, 2, "got %d", 2) }},
		{"EqualCleanString", func(ft *testing.T) { EqualCleanString(ft, "a b", "cd") }},
		{"EqualCleanString with message", func(ft *testing.T) { EqualCleanString(ft, "a b", "cd", "cleaned strings differ") }},
		{"NotEqual", func(ft *testing.T) { NotEqual(ft, "a", "a") }},
		{"NotEqual with message", func(ft *testing.T) { NotEqual(ft, 1, 1, "should differ") }},
		{"True", func(ft *testing.T) { True(ft, false) }},
		{"True with message", func(ft *testing.T) { True(ft, false, "should be true") }},
		{"False", func(ft *testing.T) { False(ft, true) }},
		{"False with message", func(ft *testing.T) { False(ft, true, "should be false") }},
		{"NotNil", func(ft *testing.T) { NotNil(ft, nil) }},
		{"NotNil with message", func(ft *testing.T) { NotNil(ft, nil, "should not be nil") }},
		{"Nil", func(ft *testing.T) { Nil(ft, "value") }},
		{"Nil with message", func(ft *testing.T) { Nil(ft, "value", "should be nil") }},
	}
	for _, c := range cases {
		ft := new(testing.T)
		c.call(ft)
		if !ft.Failed() {
			t.Errorf("%s failure case should mark the test as failed", c.name)
		}
	}
}

func TestConcurrentAssertions(t *testing.T) {
	done := make(chan struct{})
	go func() {
		Equal(t, 1, 1)
		True(t, true)
		done <- struct{}{}
	}()
	<-done
	Equal(t, fmt.Sprint("ok"), "ok")
}
