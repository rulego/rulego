package endpoint

import (
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerLocaleRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /locales - 获取语言包
	ep.GET(endpoint.NewRouter().From(base+"/locales").Process(s.authWithPermission("locale", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		localeSvc, ok := getService[services.LocaleService](s, exchange, services.KeyLocaleService)
		if !ok {
			return false
		}
		lang := exchange.In.GetParam("lang")
		if lang != "" {
			data, err := localeSvc.Get(lang)
			if err != nil {
				writeInternalError(exchange, err)
				return false
			}
			if data != nil {
				writeJSON(exchange, data)
			}
			return true
		}
		langs, err := localeSvc.List()
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, langs)
		return true
	}).End())

	// POST /locales - 保存语言包
	ep.POST(endpoint.NewRouter().From(base+"/locales").Process(s.authWithPermission("locale", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		localeSvc, ok := getService[services.LocaleService](s, exchange, services.KeyLocaleService)
		if !ok {
			return false
		}
		lang := exchange.In.GetParam("lang")
		if lang == "" {
			lang = "en"
		}
		if err := localeSvc.Save(lang, exchange.In.Body()); err != nil {
			writeBadRequest(exchange, err)
		}
		return true
	}).End())
}
