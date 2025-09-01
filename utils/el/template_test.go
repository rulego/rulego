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
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/utils/json"
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
			name: "object content",
			tmpl: `{"name":"lala", "age":10}`,
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
			name: "object content with ${}",
			tmpl: `${{"name":"lala", "age":10}}`,
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
			name: "simple variable with no ${}",
			tmpl: `user.Name`,
			data: map[string]interface{}{
				"user": struct{ Name string }{Name: "lala"},
			},
			expected: "lala",
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

func TestExprTemplate_ExecuteFn(t *testing.T) {
	tmpl, _ := NewExprTemplate("${a + b}")
	dataFunc := func() map[string]any {
		return map[string]any{"a": 1, "b": 2}
	}
	result, err := tmpl.ExecuteFn(dataFunc)
	if err != nil {
		t.Fatalf("ExecuteFn failed: %v", err)
	}
	if result.(int) != 3 {
		t.Errorf("ExecuteFn result = %v, want 3", result)
	}

	// Test with nil dataFunc
	tmplNoVar, _ := NewExprTemplate("1+1")
	resultNoVar, err := tmplNoVar.ExecuteFn(nil)
	if err != nil {
		t.Fatalf("ExecuteFn with nil dataFunc failed: %v", err)
	}
	if resultNoVar.(int) != 2 {
		t.Errorf("ExecuteFn with nil dataFunc result = %v, want 2", resultNoVar)
	}
}

func TestMixedTemplate_Execute_NoVars(t *testing.T) {
	tmplStr := "this is a string without variables"
	mixedTmpl, err := NewMixedTemplate(tmplStr)
	if err != nil {
		t.Fatalf("NewMixedTemplate failed: %v", err)
	}
	if mixedTmpl.HasVar() {
		t.Errorf("Expected HasVar to be false for template without variables")
	}
	result, err := mixedTmpl.Execute(nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != tmplStr {
		t.Errorf("Execute() = %v, want %v", result, tmplStr)
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
			name:     "single expression template",
			input:    "${user.Name}",
			wantType: "*el.ExprTemplate",
			wantErr:  false,
		},
		{
			name:     "mixed template with multiple variables",
			input:    "${metadata.prefix}_${metadata.suffix}",
			wantType: "*el.MixedTemplate",
			wantErr:  false,
		},
		{
			name:     "mixed template with text and variable",
			input:    "Hello ${user.Name}!",
			wantType: "*el.MixedTemplate",
			wantErr:  false,
		},
		{
			name:     "mixed template with variable in middle",
			input:    "prefix_${id}_suffix",
			wantType: "*el.MixedTemplate",
			wantErr:  false,
		},
		{
			name:     "expression with additional text should be mixed",
			input:    "${user.name}_test",
			wantType: "*el.MixedTemplate",
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
		{
			name:     "single expression with spaces",
			input:    "  ${user.Age}  ",
			wantType: "*el.ExprTemplate",
			wantErr:  false,
		},
		{
			name:     "complex single expression",
			input:    "${user.Name + ' - ' + user.Email}",
			wantType: "*el.ExprTemplate",
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

// TestMixedTemplateExecution 测试混合模板的执行功能
func TestMixedTemplateExecution(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		expected string
	}{
		{
			name:     "multiple variables with underscore",
			template: "${metadata.prefix}_${metadata.suffix}",
			data: map[string]interface{}{
				"metadata": map[string]interface{}{
					"prefix": "user",
					"suffix": "123",
				},
			},
			expected: "user_123",
		},
		{
			name:     "text with variable",
			template: "Hello ${user.name}!",
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
				},
			},
			expected: "Hello Alice!",
		},
		{
			name:     "multiple variables in path",
			template: "/api/${version}/users/${userId}/profile",
			data: map[string]interface{}{
				"version": "v1",
				"userId":  "12345",
			},
			expected: "/api/v1/users/12345/profile",
		},
		{
			name:     "variable with prefix and suffix",
			template: "prefix_${id}_suffix",
			data: map[string]interface{}{
				"id": "test",
			},
			expected: "prefix_test_suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := NewTemplate(tt.template)
			if err != nil {
				t.Fatalf("NewTemplate() error = %v", err)
			}

			// 验证模板类型
			if _, ok := tmpl.(*MixedTemplate); !ok {
				t.Errorf("Expected MixedTemplate, got %T", tmpl)
			}

			// 测试执行结果
			result := tmpl.ExecuteAsString(tt.data)
			if result != tt.expected {
				t.Errorf("ExecuteAsString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExprTemplate2(t *testing.T) {
	var params = []string{
		"${msg.customer_id}",
		"${msg.station_id}",
		"${msg.cabinet_id}",
		"${msg.SubCode}",
		"${msg.PcsCode}",
		"${msg.DirectVol}",
		"${msg.DirectCur}",
		"${msg.DirectActPower}",
		"${msg.DirectReactPower}",
		"${msg.DeviceTemp}",
		"${msg.ABLineVol}",
		"${msg.BCLineVol}",
		"${msg.CALineVol}",
		"${msg.AGridCur}",
		"${msg.BGridCur}",
		"${msg.CGridCur}",
		"${msg.AGridActPower}",
		"${msg.BGridActPower}",
		"${msg.CGridActPower}",
		"${msg.AGridAppPower}",
		"${msg.BGridAppPower}",
		"${msg.CGridAppPower}",
		"${msg.AGridNoactPower}",
		"${msg.BGridNoactPower}",
		"${msg.CGridNoactPower}",
		"${msg.GridFrequency}",
		"${msg.TotActPower}",
		"${msg.TotNoactPower}",
		"${msg.TotAppPower}",
		"${msg.APhasePowerFactor}",
		"${msg.BPhasePowerFactor}",
		"${msg.CPhasePowerFactor}",
		"${msg.GridPowerFac}",
		"${msg.APhaseFreq}",
		"${msg.BPhaseFreq}",
		"${msg.CPhaseFreq}",
		"${msg.TotalFreq}",
		"${msg.RatedVol}",
		"${msg.RatedCur}",
		"${msg.RatedFreq}",
		"${msg.PcsMaxChgActPower}",
		"${msg.PcsMaxDisChgReactPower}",
		"${msg.PcsMaxDisChgVol}",
		"${msg.PcsMaxDisChgCur}",
		"${msg.CommSt}",
		"${msg.RunSt}",
		"${msg.FaultSt}",
		"${msg.StopSt}",
		"${msg.PlcState}",
		"${msg.PlcStateReq}",
		"${msg.PlcIo}",
		"${msg.OnOffGridSt}",
		"${msg.AlarmSt}",
		"${msg.SwitchSt}",
		"${msg.RunMode}",
		"${msg.PowerFactor}",
		"${msg.NoactPowerMode}",
		"${msg.WatchDog}",
		"${msg.PcsState}",
		"${msg.PcsFault}",
		"${msg.FltCode7}",
		"${msg.FltCode8}",
		"${msg.FltCode9}",
		"${msg.FltCode10}",
		"${msg.FltCode11}",
		"${msg.ALineVol}",
		"${msg.BLineVol}",
		"${msg.CLineVol}",
		"${msg.TotPF}",
		"${msg.BackPowerGridEQ}",
		"${msg.PowerGridEQ}",
		"${msg.DeviceTemp}",
		"${msg.ActPowerDispatch}",
		"${msg.DirectChargeEQ}",
		"${msg.DirectDischargeEQ}",
		"${msg.GridTied}",
		"${msg.OffGrid}",
		"${msg.AllPcsCrntSum}",
		"${msg.AllPcsCrntSum}",
		"${msg.AllPcsVolAvg}",
	}
	var list []*vm.Program
	for _, item := range params {
		item = strings.Replace(item, "${", "", -1)
		item = strings.Replace(item, "}", "", -1)
		if program, err := expr.Compile(item, expr.AllowUndefinedVariables()); err == nil {
			list = append(list, program)
		}
	}
	var msgStr = `
{
  "ABLineVol": 0,
  "AGridActPower": 12.3,
  "AGridAppPower": 12.3,
  "AGridCur": 55.1,
  "AGridNoactPower": -0.5,
  "ALineVol": 227.4,
  "APhaseFreq": 0,
  "APhasePowerFactor": 1,
  "ActPowerDispatch": 0,
  "AlarmSt": 0,
  "AllPcsCrntSum": 55.03333333333334,
  "AllPcsPowerSum": 37.1,
  "AllPcsVolAvg": 227.13333333333335,
  "BCLineVol": 0,
  "BGridActPower": 12.3,
  "BGridAppPower": 12.3,
  "BGridCur": 55.6,
  "BGridNoactPower": -0.4,
  "BLineVol": 225.2,
  "BPhaseFreq": 0,
  "BPhasePowerFactor": 1,
  "BackPowerGridEQ": 0,
  "CALineVol": 0,
  "CGridActPower": 12.2,
  "CGridAppPower": 12.2,
  "CGridCur": 54.400000000000006,
  "CGridNoactPower": -0.5,
  "CLineVol": 228.8,
  "CPhaseFreq": 0,
  "CPhasePowerFactor": 1,
  "CommSt": 0,
  "DeviceTemp": 40,
  "DirectActPower": 38.8,
  "DirectChargeEQ": 0,
  "DirectCur": 45.900000000000006,
  "DirectDischargeEQ": 0,
  "DirectReactPower": 0,
  "DirectVol": 848.1,
  "FaultSt": 0,
  "FltCode10": 0,
  "FltCode11": 0,
  "FltCode7": 0,
  "FltCode8": 0,
  "FltCode9": 0,
  "GridFrequency": 49.99,
  "GridPowerFac": 0,
  "GridTied": 0,
  "NoactPowerMode": 0,
  "OffGrid": 0,
  "OnOffGridSt": 0,
  "PcsCode": "Pcs1",
  "PcsFault": 0,
  "PcsMaxChgActPower": 0,
  "PcsMaxDisChgCur": 0,
  "PcsMaxDisChgReactPower": 0,
  "PcsMaxDisChgVol": 0,
  "PcsState": 0,
  "PlcIo": 0,
  "PlcState": 0,
  "PlcStateReq": 0,
  "PowerFactor": 0,
  "PowerGridEQ": 0,
  "RatedCur": 0,
  "RatedFreq": 0,
  "RatedVol": 0,
  "RunMode": 0,
  "RunSt": 0,
  "StkCode": "Stk1",
  "StopSt": 0,
  "SubCode": "",
  "SwitchSt": 0,
  "Time": 1746009114192,
  "TotActPower": 37.1,
  "TotAppPower": 36.8,
  "TotNoactPower": -1.4,
  "TotPF": 1,
  "TotalFreq": 0,
  "WatchDog": 0,
  "cabinet_id": 48,
  "customer_id": 66,
  "id": "8c32232cc41c:2",
  "station_id": 169
}
`
	var msg map[string]interface{}
	_ = json.Unmarshal([]byte(msgStr), &msg)
	var data = map[string]any{
		"msg": msg,
	}
	var i = 0
	for i < 100000 {
		for _, item := range list {
			go func(program *vm.Program) {
				var vm vm.VM
				_, _ = vm.Run(program, data)
			}(item)
		}
		i++
	}

}

// 性能对比测试：单个表达式 vs 合并表达式
func TestExprPerformanceComparison(t *testing.T) {
	var params = []string{
		"${msg.customer_id}",
		"${msg.station_id}",
		"${msg.cabinet_id}",
		"${msg.SubCode}",
		"${msg.PcsCode}",
		"${msg.DirectVol}",
		"${msg.DirectCur}",
		"${msg.DirectActPower}",
		"${msg.DirectReactPower}",
		"${msg.DeviceTemp}",
		"${msg.ABLineVol}",
		"${msg.BCLineVol}",
		"${msg.CALineVol}",
		"${msg.AGridCur}",
		"${msg.BGridCur}",
		"${msg.CGridCur}",
		"${msg.AGridActPower}",
		"${msg.BGridActPower}",
		"${msg.CGridActPower}",
		"${msg.AGridAppPower}",
	}

	// 测试数据
	var msgStr = `{
		"customer_id": 66,
		"station_id": 169,
		"cabinet_id": 48,
		"SubCode": "SUB001",
		"PcsCode": "Pcs1",
		"DirectVol": 848.1,
		"DirectCur": 45.9,
		"DirectActPower": 38.8,
		"DirectReactPower": 0,
		"DeviceTemp": 40,
		"ABLineVol": 0,
		"BCLineVol": 0,
		"CALineVol": 0,
		"AGridCur": 55.1,
		"BGridCur": 55.6,
		"CGridCur": 54.4,
		"AGridActPower": 12.3,
		"BGridActPower": 12.3,
		"CGridActPower": 12.2,
		"AGridAppPower": 12.3
	}`

	var msg map[string]interface{}
	_ = json.Unmarshal([]byte(msgStr), &msg)
	var data = map[string]any{
		"msg": msg,
	}

	// 方法1：单个表达式分别执行
	var individualPrograms []*vm.Program
	for _, param := range params {
		cleanParam := strings.Replace(strings.Replace(param, "${", "", -1), "}", "", -1)
		if program, err := expr.Compile(cleanParam, expr.AllowUndefinedVariables()); err == nil {
			individualPrograms = append(individualPrograms, program)
		}
	}

	// 方法2：合并为数组表达式
	var arrayExprParts []string
	for _, param := range params {
		cleanParam := strings.Replace(strings.Replace(param, "${", "", -1), "}", "", -1)
		arrayExprParts = append(arrayExprParts, cleanParam)
	}
	arrayExpr := "[" + strings.Join(arrayExprParts, ", ") + "]"
	arrayProgram, _ := expr.Compile(arrayExpr, expr.AllowUndefinedVariables())

	// 方法3：合并为map表达式
	var mapExprParts []string
	for i, param := range params {
		cleanParam := strings.Replace(strings.Replace(param, "${", "", -1), "}", "", -1)
		mapExprParts = append(mapExprParts, fmt.Sprintf(`"field%d": %s`, i, cleanParam))
	}
	mapExpr := "{" + strings.Join(mapExprParts, ", ") + "}"
	mapProgram, _ := expr.Compile(mapExpr, expr.AllowUndefinedVariables())

	// 性能测试参数
	const iterations = 10000

	// 测试1：单个表达式分别执行
	t.Run("Individual Expressions", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < iterations; i++ {
			var vm vm.VM
			for _, program := range individualPrograms {
				_, _ = vm.Run(program, data)
			}
		}
		duration := time.Since(start)
		t.Logf("Individual expressions: %v (avg: %v per iteration)", duration, duration/iterations)
	})

	// 测试2：数组表达式
	t.Run("Array Expression", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < iterations; i++ {
			var vm vm.VM
			_, _ = vm.Run(arrayProgram, data)
		}
		duration := time.Since(start)
		t.Logf("Array expression: %v (avg: %v per iteration)", duration, duration/iterations)
	})

	// 测试3：map表达式
	t.Run("Map Expression", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < iterations; i++ {
			var vm vm.VM
			_, _ = vm.Run(mapProgram, data)
		}
		duration := time.Since(start)
		t.Logf("Map expression: %v (avg: %v per iteration)", duration, duration/iterations)
	})

}

// Benchmark测试
func BenchmarkIndividualExpressions(b *testing.B) {
	params := []string{
		"${msg.customer_id}", "${msg.station_id}", "${msg.cabinet_id}",
		"${msg.DirectVol}", "${msg.DirectCur}", "${msg.DirectActPower}",
	}

	var programs []*vm.Program
	for _, param := range params {
		cleanParam := strings.Replace(strings.Replace(param, "${", "", -1), "}", "", -1)
		if program, err := expr.Compile(cleanParam, expr.AllowUndefinedVariables()); err == nil {
			programs = append(programs, program)
		}
	}

	data := map[string]any{
		"msg": map[string]interface{}{
			"customer_id": 66, "station_id": 169, "cabinet_id": 48,
			"DirectVol": 848.1, "DirectCur": 45.9, "DirectActPower": 38.8,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vm vm.VM
		for _, program := range programs {
			_, _ = vm.Run(program, data)
		}
	}
}

func BenchmarkArrayExpression(b *testing.B) {
	params := []string{
		"${msg.customer_id}", "${msg.station_id}", "${msg.cabinet_id}",
		"${msg.DirectVol}", "${msg.DirectCur}", "${msg.DirectActPower}",
	}

	var arrayExprParts []string
	for _, param := range params {
		cleanParam := strings.Replace(strings.Replace(param, "${", "", -1), "}", "", -1)
		arrayExprParts = append(arrayExprParts, cleanParam)
	}
	arrayExpr := "[" + strings.Join(arrayExprParts, ", ") + "]"
	arrayProgram, _ := expr.Compile(arrayExpr, expr.AllowUndefinedVariables())

	data := map[string]any{
		"msg": map[string]interface{}{
			"customer_id": 66, "station_id": 169, "cabinet_id": 48,
			"DirectVol": 848.1, "DirectCur": 45.9, "DirectActPower": 38.8,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vm vm.VM
		_, _ = vm.Run(arrayProgram, data)
	}
}

func BenchmarkMapExpression(b *testing.B) {
	params := []string{
		"${msg.customer_id}", "${msg.station_id}", "${msg.cabinet_id}",
		"${msg.DirectVol}", "${msg.DirectCur}", "${msg.DirectActPower}",
	}

	var mapExprParts []string
	for i, param := range params {
		cleanParam := strings.Replace(strings.Replace(param, "${", "", -1), "}", "", -1)
		mapExprParts = append(mapExprParts, fmt.Sprintf(`"field%d": %s`, i, cleanParam))
	}
	mapExpr := "{" + strings.Join(mapExprParts, ", ") + "}"
	mapProgram, _ := expr.Compile(mapExpr, expr.AllowUndefinedVariables())

	data := map[string]any{
		"msg": map[string]interface{}{
			"customer_id": 66, "station_id": 169, "cabinet_id": 48,
			"DirectVol": 848.1, "DirectCur": 45.9, "DirectActPower": 38.8,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vm vm.VM
		_, _ = vm.Run(mapProgram, data)
	}
}
