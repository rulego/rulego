package el

import (
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/utils/str"
	"regexp"
	"strings"
)

type Template interface {
	Parse() error
	Execute(data map[string]any) (interface{}, error)
	// Deprecated: Use HasVar instead.
	// IsNotVar 是否是模板变量
	IsNotVar() bool
	// HasVar 是否有变量
	HasVar() bool
}

func NewTemplate(tmpl any, params ...any) (Template, error) {
	if v, ok := tmpl.(string); ok {
		trimV := strings.TrimSpace(v)
		if strings.HasPrefix(trimV, str.VarPrefix) && strings.HasSuffix(trimV, str.VarSuffix) {
			return NewExprTemplate(v)
		} else if str.CheckHasVar(v) {
			return NewMixedTemplate(v)
		} else {
			return &NotTemplate{Tmpl: v}, nil
		}
	} else {
		return &AnyTemplate{Tmpl: tmpl}, nil
	}

}

// ExprTemplate 模板变量支持 这种方式 ${xx},使用expr表达式计算
type ExprTemplate struct {
	Tmpl    string
	Program *vm.Program
	vm      vm.VM
}

// 定义正则表达式，用于匹配形如 ${...} 的占位符
var re = regexp.MustCompile(`\$\{([^}]*)\}`)

func NewExprTemplate(tmpl string) (*ExprTemplate, error) {
	// 使用字符串构建器来处理模板字符串
	var sb strings.Builder
	inQuotes := false // 标记是否在双引号内

	for i := 0; i < len(tmpl); i++ {
		switch tmpl[i] {
		case '"':
			// 翻转 inQuotes 标志
			inQuotes = !inQuotes
			sb.WriteByte(tmpl[i])
		case '\\':
			// 处理转义字符
			if i+1 < len(tmpl) {
				sb.WriteByte(tmpl[i])
				i++
				sb.WriteByte(tmpl[i])
			}
		default:
			if !inQuotes && i+1 < len(tmpl) && tmpl[i] == '$' && tmpl[i+1] == '{' {
				// 如果不在双引号内且遇到${，尝试匹配并替换
				loc := re.FindStringIndex(tmpl[i:])
				if loc != nil {
					// 找到匹配的 ${...}
					start, end := loc[0], loc[1]
					sb.WriteString(tmpl[i : i+start])         // 写入 ${ 前的内容
					sb.WriteString(tmpl[i+start+2 : i+end-1]) // 替换为 $1
					i += end - 1                              // 跳过已处理的部分
					continue
				}
			}
			// 如果在双引号内或未找到匹配项，直接写入字符
			sb.WriteByte(tmpl[i])
		}
	}

	// 替换后的模板字符串
	tmpl = sb.String()

	// 创建 ExprTemplate 实例
	t := &ExprTemplate{Tmpl: tmpl}

	// 调用 Parse 方法解析模板
	if err := t.Parse(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *ExprTemplate) Parse() error {
	if program, err := expr.Compile(t.Tmpl, expr.AllowUndefinedVariables()); err != nil {
		return err
	} else {
		t.Program = program
	}
	return nil
}

func (t *ExprTemplate) Execute(data map[string]any) (interface{}, error) {
	if t.Program != nil {
		return t.vm.Run(t.Program, data)
	}
	return nil, nil
}

func (t *ExprTemplate) ExecuteFn(loadDataFunc func() map[string]any) (interface{}, error) {
	var data map[string]any
	if loadDataFunc != nil {
		data = loadDataFunc()
	}
	return t.Execute(data)
}

func (t *ExprTemplate) IsNotVar() bool {
	return false
}

func (t *ExprTemplate) HasVar() bool {
	return true
}

// NotTemplate 原样输出
type NotTemplate struct {
	Tmpl string
}

func (t *NotTemplate) Parse() error {
	return nil
}

func (t *NotTemplate) Execute(data map[string]any) (interface{}, error) {
	return t.Tmpl, nil
}

func (t *NotTemplate) IsNotVar() bool {
	return true
}

func (t *NotTemplate) HasVar() bool {
	return false
}

type AnyTemplate struct {
	Tmpl any
}

func (t *AnyTemplate) Parse() error {
	return nil
}

func (t *AnyTemplate) Execute(data map[string]any) (interface{}, error) {
	return t.Tmpl, nil
}

func (t *AnyTemplate) IsNotVar() bool {
	return true
}

func (t *AnyTemplate) HasVar() bool {
	return false
}

// MixedTemplate 支持混合字符串和变量的模板，格式如 aa/${xxx}
type MixedTemplate struct {
	Tmpl      string
	variables []struct {
		start int
		end   int
		expr  *vm.Program
	}
	hasVars bool // 是否包含变量
}

func NewMixedTemplate(tmpl string) (*MixedTemplate, error) {
	t := &MixedTemplate{Tmpl: tmpl}
	if err := t.Parse(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *MixedTemplate) Parse() error {
	// 先检查是否包含${}变量
	if !strings.Contains(t.Tmpl, "${") {
		t.hasVars = false
		return nil
	}

	t.hasVars = true
	parts := strings.Split(t.Tmpl, "${")
	if len(parts) == 1 {
		return nil
	}

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		end := strings.Index(part, "}")
		if end == -1 {
			continue
		}

		varName := part[:end]
		program, err := expr.Compile(varName, expr.AllowUndefinedVariables())
		if err != nil {
			return err
		}

		t.variables = append(t.variables, struct {
			start int
			end   int
			expr  *vm.Program
		}{
			start: strings.Index(t.Tmpl, "${"+varName+"}"),
			end:   strings.Index(t.Tmpl, "${"+varName+"}") + len("${"+varName+"}"),
			expr:  program,
		})
	}
	return nil
}

func (t *MixedTemplate) Execute(data map[string]any) (interface{}, error) {
	// 如果没有变量，直接返回原始字符串
	if !t.hasVars {
		return t.Tmpl, nil
	}

	if len(t.variables) == 0 {
		return t.Tmpl, nil
	}

	var sb strings.Builder
	lastPos := 0
	vm := vm.VM{}
	for _, v := range t.variables {
		sb.WriteString(t.Tmpl[lastPos:v.start])
		val, err := vm.Run(v.expr, data)
		if err != nil {
			return nil, err
		}
		sb.WriteString(str.ToString(val))
		lastPos = v.end
	}
	sb.WriteString(t.Tmpl[lastPos:])
	return sb.String(), nil
}

func (t *MixedTemplate) ExecuteFn(loadDataFunc func() map[string]any) (interface{}, error) {
	var data map[string]any
	if loadDataFunc != nil {
		data = loadDataFunc()
	}
	return t.Execute(data)
}

func (t *MixedTemplate) IsNotVar() bool {
	return !t.hasVars
}

func (t *MixedTemplate) HasVar() bool {
	return t.hasVars
}

func (t *MixedTemplate) ExecuteAsString(data map[string]any) string {
	val, _ := t.Execute(data)
	return str.ToString(val)
}

func (t *MixedTemplate) ExecuteFnAsString(loadDataFunc func() map[string]any) string {
	val, _ := t.ExecuteFn(loadDataFunc)
	return str.ToString(val)
}
