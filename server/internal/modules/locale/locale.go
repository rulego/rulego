package locale

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"fmt"
	"strings"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/utils/fs"
)

const (
	ModuleName = "locale"
	Priority   = 50
)

// Module locale 业务模块，负责语言包管理。
type Module struct {
	cfg *config.Config
}

// New 创建 locale 模块
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	if err := ctx.Container.Register(services.KeyLocaleService, services.LocaleService(m)); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

func (m *Module) Get(lang string) (interface{}, error) {
	lang = strings.ReplaceAll(lang, "/", "")
	lang = strings.ReplaceAll(lang, "\\", "")
	if lang == "" {
		lang = "en"
	}
	if !strings.HasSuffix(lang, ".json") {
		lang = lang + ".json"
	}
	pathStr := path.Join(m.cfg.DataDir, constants.DirPublic, "locales", lang)
	if _, err := os.Stat(pathStr); err != nil {
		return nil, fmt.Errorf("locale %s not found", lang)
	}
	data, err := os.ReadFile(pathStr)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *Module) Save(lang string, data []byte) error {
	lang = strings.ReplaceAll(lang, "/", "")
	lang = strings.ReplaceAll(lang, "\\", "")
	if !strings.HasSuffix(lang, ".json") {
		lang = lang + ".json"
	}
	pathStr := path.Join(m.cfg.DataDir, constants.DirPublic, "locales")
	_ = fs.CreateDirs(pathStr)
	return fs.SaveFile(filepath.Join(pathStr, lang), data)
}

func (m *Module) List() ([]string, error) {
	pathStr := path.Join(m.cfg.DataDir, constants.DirPublic, "locales")
	_ = fs.CreateDirs(pathStr)
	entries, err := os.ReadDir(pathStr)
	if err != nil {
		return nil, err
	}
	var langs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			langs = append(langs, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return langs, nil
}
