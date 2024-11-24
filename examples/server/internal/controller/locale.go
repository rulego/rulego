package controller

import (
	"examples/server/config"
	"examples/server/internal/constants"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/utils/fs"
	"net/http"
	"os"
	"path/filepath"
)

var Locale = &locale{}

type locale struct {
}

func (c *locale) Locales(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		lang := exchange.In.GetParam(constants.KeyLang)
		if lang == "" {
			lang = "zh_cn"
		}
		path := filepath.Join(config.C.DataDir, constants.DirLocales, lang+".json")
		buf, err := os.ReadFile(path)
		if err != nil {
			exchange.Out.SetBody([]byte("{}"))
		} else {
			exchange.Out.SetBody(buf)
		}
		return true
	}).End()
}

func (c *locale) Save(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		lang := exchange.In.GetParam(constants.KeyLang)
		if lang == "" {
			lang = "zh_cn"
		}
		path := filepath.Join(config.C.DataDir, constants.DirLocales)
		_ = fs.CreateDirs(path)
		body := exchange.In.Body()

		if err := fs.SaveFile(filepath.Join(path, lang+".json"), body); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		}
		return true
	}).End()
}
