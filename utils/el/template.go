package el

import (
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/utils/str"
	"regexp"
)

type Template interface {
	Parse() error
	Execute(data map[string]any) (interface{}, error)
	// IsNotVar 是否是模板变量
	IsNotVar() bool
}

func NewTemplate(tmpl any, params ...any) (Template, error) {
	if v, ok := tmpl.(string); ok {
		if str.CheckHasVar(v) {
			return NewExprTemplate(v)
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

var re = regexp.MustCompile(`\$\{([^}]*)\}`)

func NewExprTemplate(tmpl string) (*ExprTemplate, error) {
	// 移除模板中的${前缀和}后缀，保留里面的内容
	tmpl = re.ReplaceAllString(tmpl, "$1")
	t := &ExprTemplate{Tmpl: tmpl}
	err := t.Parse()
	return t, err
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

// StrTemplate 模板变量支持 这种方式 ${xx} 返回值是string
type StrTemplate struct {
	str.VarTemplate
}

func (t *StrTemplate) Execute(data map[string]any) (interface{}, error) {
	return t.VarTemplate.Execute(data), nil
}

func (t *StrTemplate) ExecuteFn(loadDataFunc func() map[string]any) (interface{}, error) {
	var data map[string]any
	if loadDataFunc != nil {
		data = loadDataFunc()
	}
	return t.Execute(data)
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
