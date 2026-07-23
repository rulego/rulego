package el

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/utils/str"
)

// remoteHTTPClient is used for remote requests and multiplexing connections of the include function
var remoteHTTPClient = &http.Client{Timeout: 10 * time.Second}

// remoteCache caches the contents of remote URLs to avoid repeated requests via hot paths
var remoteCache struct {
	sync.RWMutex
	items map[string]cacheEntry
}

type cacheEntry struct {
	content   string
	expiresAt time.Time
}

const remoteCacheTTL = 30 * time.Second

func init() {
	remoteCache.items = make(map[string]cacheEntry)
}

// fetchRemoteContent fetchRemoteContent fetchs remote content via HTTP GET with TTL cache
func fetchRemoteContent(url string) string {
	// Check the cache
	remoteCache.RLock()
	if entry, ok := remoteCache.items[url]; ok && time.Now().Before(entry.expiresAt) {
		remoteCache.RUnlock()
		return entry.content
	}
	remoteCache.RUnlock()

	resp, err := remoteHTTPClient.Get(url)
	if err != nil {
		log.Printf("template include: fetch remote %s failed: %v", url, err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("template include: fetch remote %s returned status %d", url, resp.StatusCode)
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("template include: read remote %s body failed: %v", url, err)
		return ""
	}
	content := string(body)

	// Write to the cache
	remoteCache.Lock()
	remoteCache.items[url] = cacheEntry{content: content, expiresAt: time.Now().Add(remoteCacheTTL)}
	remoteCache.Unlock()

	return content
}

// isRemoteURL checks whether the path is a remote URL
func isRemoteURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

type Template interface {
	Parse() error
	Execute(data map[string]any) (interface{}, error)
	ExecuteFn(loadDataFunc func() map[string]any) (interface{}, error)
	ExecuteAsString(data map[string]any) string
	// Deprecated: Use HasVar instead.
	// Is IsNotVar a template variable?
	IsNotVar() bool
	// Does HasVar have variables?
	HasVar() bool
}

// TemplateConfig Template configuration options
type TemplateConfig struct {
	IncludeFunc IncludeFunc // Customize the include function
}

// Option template option function
type Option func(*TemplateConfig)

// WithIncludeFunc sets up a custom include function
func WithIncludeFunc(fn IncludeFunc) Option {
	return func(cfg *TemplateConfig) {
		cfg.IncludeFunc = fn
	}
}

// The IncludeFunc file contains function types
type IncludeFunc func(path string) string

// NewTemplate creates corresponding template instances based on the template content
// Identification rules:
// 1. If it is a complete single expression ${...}, create ExprTemplate
// 2. If the variables are included but not a single expression, create a MixedTemplate
// 3. If the variable is not included, create a NotTemplate
// 4. If not of the string type, create an AnyTemplate
//
// Supported options:
//   - WithIncludeFunc(fn IncludeFunc): Sets the custom include function
//
// Example of using the include function:
//   - ${include("/path/to/file.txt")}: Contains file content (use absolute path)
//   - ${upper(include("/path/to/file.txt"))}: Contains the file content and converts it to uppercase
//   - ${include("/path/to/file.txt") + suffix}: Contains file content and concatenates the suffix
func NewTemplate(tmpl any, opts ...Option) (Template, error) {
	// Analyze configuration
	cfg := &TemplateConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if v, ok := tmpl.(string); ok {
		trimV := strings.TrimSpace(v)
		// Check if it is a complete single expression: start with ${, end with }, and have no other ${ or } in between.
		if strings.HasPrefix(trimV, str.VarPrefix) && strings.HasSuffix(trimV, str.VarSuffix) {
			// Check if it is a single complete expression (without any extra ${ or } in between).
			middle := trimV[2 : len(trimV)-1] // Remove the ${ at the beginning and the } at the end.
			if !strings.Contains(middle, "${") && !strings.Contains(middle, "}") {
				return NewExprTemplateWithConfig(v, cfg)
			}
		}
		// If it contains a variable but not a single expression, use MixedTemplate
		if str.CheckHasVar(v) {
			return NewMixedTemplateWithConfig(v, cfg)
		} else {
			return &NotTemplate{Tmpl: v}, nil
		}
	} else {
		return &AnyTemplate{Tmpl: tmpl}, nil
	}
}

// The ExprTemplate template variable supports this method of ${xx}, calculated using the expr expression
type ExprTemplate struct {
	Tmpl    string
	Program *vm.Program
	config  *TemplateConfig
}

// Define regular expressions to match placeholders like ${...}
var re = regexp.MustCompile(`\$\{([^}]*)\}`)

// NewExprTemplate Creating an expression template (backward compatible)
func NewExprTemplate(tmpl string) (*ExprTemplate, error) {
	return NewExprTemplateWithConfig(tmpl, &TemplateConfig{})
}

// NewExprTemplateWithConfig creates an expression template with configuration
func NewExprTemplateWithConfig(tmpl string, cfg *TemplateConfig) (*ExprTemplate, error) {
	// Use a string builder to handle template strings
	var sb strings.Builder
	inQuotes := false // Check whether the mark is in double quotations

	for i := 0; i < len(tmpl); i++ {
		switch tmpl[i] {
		case '"':
			// Flip the inQuotes logo
			inQuotes = !inQuotes
			sb.WriteByte(tmpl[i])
		case '\\':
			// Handling escape characters
			if i+1 < len(tmpl) {
				sb.WriteByte(tmpl[i])
				i++
				sb.WriteByte(tmpl[i])
			}
		default:
			if !inQuotes && i+1 < len(tmpl) && tmpl[i] == '$' && tmpl[i+1] == '{' {
				// If it is not in double quotes and encounters ${, try matching and replace
				loc := re.FindStringIndex(tmpl[i:])
				if loc != nil {
					// Find the matching ${...}
					start, end := loc[0], loc[1]
					sb.WriteString(tmpl[i : i+start])         // Write the content before ${
					sb.WriteString(tmpl[i+start+2 : i+end-1]) // Replace with $1
					i += end - 1                              // Skip the processed sections
					continue
				}
			}
			// If the match is inside double quotes or no match is found, write the character directly
			sb.WriteByte(tmpl[i])
		}
	}

	// Replaced template string
	tmpl = sb.String()

	// Create an ExprTemplate instance
	t := &ExprTemplate{Tmpl: tmpl, config: cfg}

	// Call Parse methods to parse templates
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

// buildEnv builds an environment that includes the include function
func (t *ExprTemplate) buildEnv(data map[string]any) map[string]any {
	env := make(map[string]any)

	// Replicate the original data
	for k, v := range data {
		env[k] = v
	}

	// Add the include function (add it as long as the config is not nil, supports absolute paths)
	if t.config != nil {
		env["include"] = t.includeFunc()
		env["fileExists"] = t.fileExistsFunc()
	}

	return env
}

// includeFunc returns the include function implementation
func (t *ExprTemplate) includeFunc() func(string) string {
	return func(path string) string {
		// Use custom functions or default implementations
		if t.config.IncludeFunc != nil {
			return t.config.IncludeFunc(path)
		}

		// Remote URL support
		if isRemoteURL(path) {
			return fetchRemoteContent(path)
		}

		// Default implementation: Read local files
		content, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(content)
	}
}

// fileExistsFunc returns the fileExists function implementation
func (t *ExprTemplate) fileExistsFunc() func(string) bool {
	return func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
}

func (t *ExprTemplate) Execute(data map[string]any) (interface{}, error) {
	if t.Program != nil {
		// Build an environment that includes the include function
		env := t.buildEnv(data)
		var vm vm.VM
		return vm.Run(t.Program, env)
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

// ExecuteAsString executes the template and returns the string result
func (t *ExprTemplate) ExecuteAsString(data map[string]any) string {
	result, err := t.Execute(data)
	if err != nil {
		return ""
	}
	if result == nil {
		return ""
	}
	return str.ToString(result)
}

// NotTemplate outputs as is
type NotTemplate struct {
	Tmpl string
}

func (t *NotTemplate) Parse() error {
	return nil
}

func (t *NotTemplate) Execute(data map[string]any) (interface{}, error) {
	return t.Tmpl, nil
}

// ExecuteFn executes template functions
func (t *NotTemplate) ExecuteFn(loadDataFunc func() map[string]any) (interface{}, error) {
	return t.Tmpl, nil
}

// ExecuteAsString executes the template and returns the string result
func (t *NotTemplate) ExecuteAsString(data map[string]any) string {
	return t.Tmpl
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

// ExecuteFn executes template functions
func (t *AnyTemplate) ExecuteFn(loadDataFunc func() map[string]any) (interface{}, error) {
	return t.Tmpl, nil
}

// ExecuteAsString executes the template and returns the string result
func (t *AnyTemplate) ExecuteAsString(data map[string]any) string {
	return str.ToString(t.Tmpl)
}

func (t *AnyTemplate) IsNotVar() bool {
	return true
}

func (t *AnyTemplate) HasVar() bool {
	return false
}

// MixedTemplate supports templates for mixing strings and variables, such as aa/${xxx}
type MixedTemplate struct {
	Tmpl      string
	variables []struct {
		start int
		end   int
		expr  string // Save the original expression string for dynamic compilation
	}
	hasVars bool // Whether variables are included
	config  *TemplateConfig
}

// NewMixedTemplate creates a hybrid template (backward compatible)
func NewMixedTemplate(tmpl string) (*MixedTemplate, error) {
	return NewMixedTemplateWithConfig(tmpl, &TemplateConfig{})
}

// NewMixedTemplateWithConfig creates a hybrid template with configuration
func NewMixedTemplateWithConfig(tmpl string, cfg *TemplateConfig) (*MixedTemplate, error) {
	t := &MixedTemplate{Tmpl: tmpl, config: cfg}
	if err := t.Parse(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *MixedTemplate) Parse() error {
	// First, check if the ${} variable is included
	if !strings.Contains(t.Tmpl, "${") {
		t.hasVars = false
		return nil
	}

	t.hasVars = true
	tmpl := t.Tmpl
	start := 0

	for {
		idx := strings.Index(tmpl[start:], "${")
		if idx == -1 {
			break
		}

		varStart := start + idx
		endIdx := strings.Index(tmpl[varStart+2:], "}")
		if endIdx == -1 {
			break
		}

		varEnd := varStart + 2 + endIdx
		varName := tmpl[varStart+2 : varEnd]

		// Save the original expression string and compile it dynamically at runtime
		t.variables = append(t.variables, struct {
			start int
			end   int
			expr  string
		}{
			start: varStart,
			end:   varEnd + 1,
			expr:  varName,
		})

		start = varEnd + 1
	}

	return nil
}

func (t *MixedTemplate) Execute(data map[string]any) (interface{}, error) {
	return t.execute(data)
}

// buildEnv builds an environment that includes the include function
func (t *MixedTemplate) buildEnv(data map[string]any) map[string]any {
	env := make(map[string]any)
	for k, v := range data {
		env[k] = v
	}

	// Add the include function (add it as long as the config is not nil)
	if t.config != nil {
		env["include"] = func(path string) string {
			if t.config.IncludeFunc != nil {
				return t.config.IncludeFunc(path)
			}
			// Remote URL support
			if isRemoteURL(path) {
				return fetchRemoteContent(path)
			}
			content, _ := os.ReadFile(path)
			return string(content)
		}
		env["fileExists"] = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}

	return env
}

func (t *MixedTemplate) execute(data map[string]any) (string, error) {
	// If there are no variables, it directly returns the original string
	if !t.hasVars {
		return t.Tmpl, nil
	}

	if len(t.variables) == 0 {
		return t.Tmpl, nil
	}

	// Build an environment that includes the include function
	env := t.buildEnv(data)

	var sb strings.Builder
	lastPos := 0
	vmInstance := &vm.VM{}

	for _, v := range t.variables {
		sb.WriteString(t.Tmpl[lastPos:v.start])

		// Dynamically compile expressions using environment variables
		program, err := expr.Compile(v.expr, expr.Env(env), expr.AllowUndefinedVariables())
		if err != nil {
			return "", fmt.Errorf("failed to compile expression '%s': %v", v.expr, err)
		}

		val, err := vmInstance.Run(program, env)
		if err != nil {
			return "", err
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
	return t.execute(data)
}

func (t *MixedTemplate) ExecuteAsString(data map[string]any) string {
	val, _ := t.execute(data)
	return val
}

func (t *MixedTemplate) ExecuteFnAsString(loadDataFunc func() map[string]any) string {
	var data map[string]any
	if loadDataFunc != nil {
		data = loadDataFunc()
	}
	val, _ := t.execute(data)
	return val
}

func (t *MixedTemplate) IsNotVar() bool {
	return !t.hasVars
}

func (t *MixedTemplate) HasVar() bool {
	return t.hasVars
}
