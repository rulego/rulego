package endpoint

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/server/app"
	serverconfig "github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	skillmodule "github.com/rulego/rulego/server/internal/modules/skill"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

// registerSkillRoutes exposes global skill management APIs used by the editor's
// AI assistant settings panel.
func (s *Server) registerSkillRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	ep.GET(endpoint.NewRouter().From(base + "/skills").Process(s.authWithPermission("skill", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		skillSvc, ok := getService[services.SkillService](s, exchange, services.KeySkillService)
		if !ok {
			return false
		}
		scope, err := skillScopeFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		items, err := skillSvc.ListSkills(metadataUsername(exchange), scope)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		msg := exchange.In.GetMsg()
		page := intParam(msg, constants.KeyPage, 1)
		size := intParam(msg, constants.KeySize, 20)
		writeJSON(exchange, map[string]interface{}{
			"path":  configuredSkillPath(s.config),
			"total": len(items),
			"page":  page,
			"size":  size,
			"items": items,
		})
		return true
	}).End())

	ep.GET(endpoint.NewRouter().From(base + "/skills/:id").Process(s.authWithPermission("skill", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		skillSvc, ok := getService[services.SkillService](s, exchange, services.KeySkillService)
		if !ok {
			return false
		}
		scope, err := skillScopeFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		skillName, err := skillIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		item, err := skillSvc.GetSkill(metadataUsername(exchange), skillName, scope)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		writeJSON(exchange, item)
		return true
	}).End())

	ep.POST(endpoint.NewRouter().From(base + "/skills").Process(s.authWithPermission("skill", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		skillSvc, ok := getService[services.SkillService](s, exchange, services.KeySkillService)
		if !ok {
			return false
		}
		var req model.Skill
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		scope, err := skillmodule.NormalizeSkillScope(req.Scope)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		req.Scope = scope
		item, err := skillSvc.CreateSkill(metadataUsername(exchange), req)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		exchange.Out.SetStatusCode(201)
		s.reloadBuiltInAssistant()
		writeJSON(exchange, item)
		return true
	}).End())

	ep.PUT(endpoint.NewRouter().From(base + "/skills/:id").Process(s.authWithPermission("skill", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		skillSvc, ok := getService[services.SkillService](s, exchange, services.KeySkillService)
		if !ok {
			return false
		}
		var req model.Skill
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		scope, err := skillmodule.NormalizeSkillScope(req.Scope)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		skillName, err := skillIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		req.Scope = scope
		req.Name = skillName
		item, err := skillSvc.UpdateSkill(metadataUsername(exchange), skillName, req)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		s.reloadBuiltInAssistant()
		writeJSON(exchange, item)
		return true
	}).End())

	ep.DELETE(endpoint.NewRouter().From(base + "/skills/:id").Process(s.authWithPermission("skill", "delete")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		skillSvc, ok := getService[services.SkillService](s, exchange, services.KeySkillService)
		if !ok {
			return false
		}
		scope, err := skillScopeFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		skillName, err := skillIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := skillSvc.DeleteSkill(metadataUsername(exchange), skillName, scope); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		s.reloadBuiltInAssistant()
		writeNoContent(exchange)
		return true
	}).End())

	ep.POST(endpoint.NewRouter().From(base + "/skills/upload").Process(s.authWithPermission("skill", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		skillSvc, ok := getService[services.SkillService](s, exchange, services.KeySkillService)
		if !ok {
			return false
		}
		scope, err := skillScopeFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		archive, err := readUploadedSkillArchive(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		items, err := skillSvc.ImportSkills(metadataUsername(exchange), scope, archive)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		s.reloadBuiltInAssistant()
		writeJSON(exchange, items)
		return true
	}).End())
}

// skillIDFromExchange validates the skill path parameter before passing it to
// the skill service so path traversal cannot escape the configured root.
func skillIDFromExchange(exchange *endpointApi.Exchange) (string, error) {
	name := strings.TrimSpace(metadataValue(exchange, constants.KeyId))
	if !validateId(name) {
		return "", errors.New("invalid skill id")
	}
	return name, nil
}

// configuredSkillPath returns the current effective global skill directory.
func configuredSkillPath(cfg *serverconfig.Config) string {
	if cfg == nil {
		return "./skills"
	}
	path := strings.TrimSpace(cfg.SkillPath)
	if path == "" {
		return "./skills"
	}
	return path
}

// readUploadedSkillArchive extracts the uploaded zip file from multipart form
// data and returns its raw bytes.
func readUploadedSkillArchive(exchange *endpointApi.Exchange) ([]byte, error) {
	req, ok := exchange.In.(*rest.RequestMessage)
	if !ok || req.Request() == nil {
		return nil, errors.New("unsupported request")
	}
	if err := req.Request().ParseMultipartForm(64 << 20); err != nil {
		return nil, errors.New("invalid multipart form data")
	}
	file, _, err := req.Request().FormFile("file")
	if err != nil {
		return nil, errors.New("uploaded file is required")
	}
	defer file.Close()
	return io.ReadAll(file)
}

// skillScopeFromExchange normalizes the scope query parameter.
func skillScopeFromExchange(exchange *endpointApi.Exchange) (string, error) {
	scope := strings.TrimSpace(exchange.In.GetMsg().Metadata.GetValue("scope"))
	return skillmodule.NormalizeSkillScope(scope)
}

// reloadBuiltInAssistant 重载内置智能体，使全局技能变更立即生效。
// 重载失败不阻塞响应，静默忽略。
func (s *Server) reloadBuiltInAssistant() {
	admin, err := app.GetAs[services.RuleAdminService](s.container, services.KeyRuleManager)
	if err != nil {
		return
	}
	reloadAssistantRuleChain(s.config.DataDir, s.config.DefaultUsername, defaultAssistantID, admin)
}
