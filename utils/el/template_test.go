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

package el

import (
	"reflect"
	"testing"
)

func TestExprTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     map[string]interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name: "simple variable",
			tmpl: `${user.Name}`,
			data: map[string]interface{}{
				"user": struct{ Name string }{Name: "lala"},
			},
			expected: "lala",
			wantErr:  false,
		},
		{
			name: "mixed content",
			tmpl: `{"name":${user.Name}, "age":${user.Age}}`,
			data: map[string]interface{}{
				"user": struct {
					Name string
					Age  int
				}{Name: "lala", Age: 10},
			},
			expected: map[string]interface{}{"name": "lala", "age": 10},
			wantErr:  false,
		},
		{
			name: "quoted variable should not be replaced",
			tmpl: `{"name":"${user.Name}", "age":${user.Age}}`,
			data: map[string]interface{}{
				"user": struct {
					Name string
					Age  int
				}{Name: "lala", Age: 10},
			},
			expected: map[string]interface{}{"name": "${user.Name}", "age": 10},
			wantErr:  false,
		},
		{
			name:    "invalid template",
			tmpl:    `${user.Name`,
			data:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := NewExprTemplate(tt.tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExprTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			got, err := expr.Execute(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Execute() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMixedTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     map[string]interface{}
		expected string
		wantErr  bool
	}{
		{
			name: "simple path template",
			tmpl: "user/${user.id}/profile",
			data: map[string]interface{}{
				"user": map[string]string{"id": "123"},
			},
			expected: "user/123/profile",
			wantErr:  false,
		},
		{
			name: "multiple variables",
			tmpl: "${user.Name}/${user.Id}/${action.Type}",
			data: map[string]interface{}{
				"user": struct {
					Name string
					Id   string
				}{Name: "john", Id: "123"},
				"action": struct{ Type string }{Type: "view"},
			},
			expected: "john/123/view",
			wantErr:  false,
		},
		{
			name: "mixed with static text",
			tmpl: "The user ${user.Name} has ${count} messages",
			data: map[string]interface{}{
				"user":  struct{ Name string }{Name: "alice"},
				"count": 5,
			},
			expected: "The user alice has 5 messages",
			wantErr:  false,
		},
		{
			name: "with escaped quotes",
			tmpl: `{"path":"${user.Id}", "name":"${user.Name}"}`,
			data: map[string]interface{}{
				"user": struct {
					Id   string
					Name string
				}{Id: "123", Name: "bob"},
			},
			expected: `{"path":"123", "name":"bob"}`,
			wantErr:  false,
		},
		{
			name: "with not var",
			tmpl: `010101`,
			data: map[string]interface{}{
				"user": struct {
					Id   string
					Name string
				}{Id: "123", Name: "bob"},
			},
			expected: `010101`,
			wantErr:  false,
		},
		{
			name:     "invalid variable",
			tmpl:     "user/${user..id}/profile",
			data:     map[string]interface{}{},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := NewMixedTemplate(tt.tmpl)
			if err != nil {
				return
			}

			got, err := st.Execute(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.expected {
				t.Errorf("Execute() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNotTemplate(t *testing.T) {
	tmpl := "static content"
	notTemplate := &NotTemplate{Tmpl: tmpl}

	got, err := notTemplate.Execute(nil)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
		return
	}

	if got != tmpl {
		t.Errorf("Execute() = %v, want %v", got, tmpl)
	}
}

func TestAnyTemplate(t *testing.T) {
	tmpl := 123
	anyTemplate := &AnyTemplate{Tmpl: tmpl}

	got, err := anyTemplate.Execute(nil)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
		return
	}

	if got != tmpl {
		t.Errorf("Execute() = %v, want %v", got, tmpl)
	}
}

func TestNewTemplate(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantType string
		wantErr  bool
	}{
		{
			name:     "string with variables",
			input:    "${user.Name}",
			wantType: "*el.ExprTemplate",
			wantErr:  false,
		},
		{
			name:     "string without variables",
			input:    "static content",
			wantType: "*el.NotTemplate",
			wantErr:  false,
		},
		{
			name:     "non-string input",
			input:    123,
			wantType: "*el.AnyTemplate",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			wantType: "*el.NotTemplate",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTemplate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && reflect.TypeOf(got).String() != tt.wantType {
				t.Errorf("NewTemplate() = %v, want %v", reflect.TypeOf(got), tt.wantType)
			}
		})
	}
}
