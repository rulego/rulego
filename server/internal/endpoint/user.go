package endpoint

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

// userAdmin 从容器取用户管理服务
func (s *Server) userAdmin() (services.UserAdmin, bool) {
	if svc, err := getServiceRaw[services.UserAdmin](s, services.KeyUserAdmin); err == nil {
		return svc, true
	}
	return nil, false
}

// rolesOfUser 查询用户角色，供登录响应与 /users/me 使用。
// 取不到用户管理服务时回退：config 内置账号算 admin，其余 editor。
func (s *Server) rolesOfUser(username string) []string {
	if svc, ok := s.userAdmin(); ok {
		if roles := svc.RolesOf(username); len(roles) > 0 {
			return roles
		}
	}
	if s.config != nil && s.config.CheckUserExists(username) {
		return []string{model.RoleAdmin}
	}
	return []string{model.RoleEditor}
}

// defaultUsername 返回 config.conf 的 default_username（默认 admin）。
// 它是 require_auth=false 下的匿名兜底身份，前端也据此识别默认租户。
func (s *Server) defaultUsername() string {
	if s.config != nil {
		return s.config.DefaultUsername
	}
	return ""
}

// sanitizeUser 清掉密码，避免回传给前端
func sanitizeUser(u model.User) model.User {
	u.Password = ""
	return u
}

func (s *Server) registerUserRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /users/me - 当前登录者信息（任何已认证用户）
	ep.GET(endpoint.NewRouter().From(base + "/users/me").Process(s.authProcess()).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		username := metadataUsername(exchange)
		result := model.User{Username: username, Roles: s.rolesOfUser(username)}
		if svc, ok := s.userAdmin(); ok {
			if u, exists := svc.Get(username); exists {
				result = sanitizeUser(u)
				result.Roles = s.rolesOfUser(username)
			}
		}
		if result.ApiKey == "" && s.config != nil {
			result.ApiKey = s.config.GetApiKeyByUsername(username)
		}
		writeJSON(exchange, result)
		return true
	}).End())

	// PATCH /users/me - 改自己密码 / 重置自己的 API Key
	ep.PATCH(endpoint.NewRouter().From(base + "/users/me").Process(s.authProcess()).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		username := metadataUsername(exchange)
		svc, ok := s.userAdmin()
		if !ok {
			writeError(exchange, http.StatusNotImplemented, errUserAdminUnavailable)
			return false
		}
		var req struct {
			OldPassword string `json:"oldPassword"`
			NewPassword string `json:"newPassword"`
			ResetApiKey bool   `json:"resetApiKey"`
		}
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		u, exists := svc.Get(username)
		if !exists {
			// config 内置账号不在 store 里：落一份到 store 再改
			u = model.User{Username: username, Roles: s.rolesOfUser(username)}
			if s.config != nil {
				u.ApiKey = s.config.GetApiKeyByUsername(username)
			}
		}
		if req.NewPassword != "" {
			// 改密必须验旧密码
			authSvc, err := getServiceRaw[services.AuthService](s, services.KeyAuthService)
			if err != nil || !authSvc.CheckPassword(username, req.OldPassword) {
				writeError(exchange, http.StatusForbidden, errOldPasswordMismatch)
				return false
			}
			u.Password = req.NewPassword
		}
		if req.ResetApiKey {
			key, err := generateApiKey()
			if err != nil {
				writeInternalError(exchange, err)
				return false
			}
			u.ApiKey = key
		}
		if err := svc.Save(u); err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, sanitizeUser(u))
		return true
	}).End())

	// GET /users - 用户列表（仅 admin）
	ep.GET(endpoint.NewRouter().From(base + "/users").Process(s.authWithPermission(constants.ResourceUser, "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := s.userAdmin()
		if !ok {
			writeError(exchange, http.StatusNotImplemented, errUserAdminUnavailable)
			return false
		}
		list := svc.List()
		out := make([]model.User, 0, len(list))
		for _, u := range list {
			out = append(out, sanitizeUser(u))
		}
		// config.conf 里的内置账号不在 store 里，但要列出来——它们能登录
		if s.config != nil {
			for username := range s.config.UserNamePasswordMap {
				if _, exists := svc.Get(username); exists {
					continue
				}
				out = append(out, model.User{
					Username: username,
					ApiKey:   s.config.GetApiKeyByUsername(username),
					Roles:    []string{model.RoleAdmin},
				})
			}
		}
		writeJSON(exchange, map[string]interface{}{
			"items":           out,
			"total":           len(out),
			"defaultUsername": s.defaultUsername(), // 匿名兜底账号，前端据此禁用删除
		})
		return true
	}).End())

	// POST /users - 创建/更新用户（仅 admin）
	ep.POST(endpoint.NewRouter().From(base + "/users").Process(s.authWithPermission(constants.ResourceUser, "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := s.userAdmin()
		if !ok {
			writeError(exchange, http.StatusNotImplemented, errUserAdminUnavailable)
			return false
		}
		var req model.User
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		req.Username = strings.TrimSpace(req.Username)
		if !isValidUsername(req.Username) {
			writeBadRequest(exchange, errInvalidUsername)
			return false
		}
		existing, exists := svc.Get(req.Username)
		if exists {
			// 更新：密码留空表示不改
			if req.Password == "" {
				req.Password = existing.Password
			}
			if req.ApiKey == "" {
				req.ApiKey = existing.ApiKey
			}
		} else {
			if req.Password == "" {
				writeBadRequest(exchange, errPasswordRequired)
				return false
			}
			if req.ApiKey == "" {
				key, err := generateApiKey()
				if err != nil {
					writeInternalError(exchange, err)
					return false
				}
				req.ApiKey = key
			}
		}
		if len(req.Roles) == 0 {
			req.Roles = []string{model.RoleEditor}
		}
		if err := svc.Save(req); err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, sanitizeUser(req))
		return true
	}).End())

	// DELETE /users/:targetUsername - 删除用户（仅 admin）
	// 参数名故意叫 :targetUsername 而非 :username：认证中间件会把登录者写进
	// 同名 metadata（constants.KeyUsername），撞名会让这里取到操作者自己，
	// 永远走进「不能删当前用户」分支。
	ep.DELETE(endpoint.NewRouter().From(base + "/users/:targetUsername").Process(s.authWithPermission(constants.ResourceUser, "delete")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		svc, ok := s.userAdmin()
		if !ok {
			writeError(exchange, http.StatusNotImplemented, errUserAdminUnavailable)
			return false
		}
		target := metadataValue(exchange, "targetUsername")
		operator := metadataUsername(exchange)
		if target == "" {
			writeBadRequest(exchange, errInvalidUsername)
			return false
		}
		if target == operator {
			writeError(exchange, http.StatusBadRequest, errCannotDeleteSelf)
			return false
		}
		// 默认租户是 require_auth=false 的匿名兜底身份，删了系统会失去登录入口
		if du := s.defaultUsername(); du != "" && target == du {
			writeError(exchange, http.StatusBadRequest, errCannotDeleteDefault)
			return false
		}
		// 先停引擎再清数据：否则残留引擎继续跑 endpoint 占端口
		if err := s.stopUserEngine(target); err != nil {
			writeInternalError(exchange, err)
			return false
		}
		if err := svc.Delete(target); err != nil {
			writeInternalError(exchange, err)
			return false
		}
		// purge=true 时一并删数据目录；默认保留以便误删找回
		if strings.TrimSpace(exchange.In.GetParam("purge")) == "true" {
			if err := s.purgeUserData(target); err != nil {
				writeInternalError(exchange, err)
				return false
			}
		}
		writeJSON(exchange, map[string]interface{}{"username": target, "deleted": true})
		return true
	}).End())
}

// purgeUserData 删除用户数据目录 {data_dir}/workflows/{username}，不可逆。
// 两层防穿越：isValidUsername 只允许 [A-Za-z0-9_.-]，从字符层挡住 ".." 与路径分隔符；
// 此外解析 symlink 后再比对前缀，避免 workflows/<user> 指向外部被误删。
func (s *Server) purgeUserData(username string) error {
	if !isValidUsername(username) || s.config == nil {
		return errInvalidUsername
	}
	dir := filepath.Join(s.config.DataDir, "workflows", username)
	root, err := filepath.EvalSymlinks(filepath.Join(s.config.DataDir, "workflows"))
	if err != nil {
		return err
	}
	// 目标目录可能不存在（重复删除或未初始化），EvalSymlinks 会失败：
	// 改为解析其父目录确认落点，再交给 RemoveAll。
	abs, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if parent, err1 := filepath.EvalSymlinks(filepath.Dir(dir)); err1 != nil || !strings.HasPrefix(parent+string(os.PathSeparator), root+string(os.PathSeparator)) {
			return errInvalidUsername
		}
		return os.RemoveAll(dir)
	}
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return errInvalidUsername
	}
	return os.RemoveAll(abs)
}
