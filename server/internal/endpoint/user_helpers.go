package endpoint

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/services"
)

// 用户管理相关错误。消息用英文且不含内部细节，可直接回给客户端。
var (
	errUserAdminUnavailable = errors.New("user administration is not enabled")
	errOldPasswordMismatch  = errors.New("old password mismatch")
	errInvalidUsername      = errors.New("invalid username")
	errPasswordRequired     = errors.New("password is required")
	errCannotDeleteSelf     = errors.New("cannot delete the current user")
	// errCannotDeleteDefault 默认租户是 require_auth=false 的匿名兜底身份，删了系统会失去登录入口。
	errCannotDeleteDefault = errors.New("cannot delete the default tenant")
)

// maxUsernameLen username 长度上限
const maxUsernameLen = 64

// getServiceRaw 从容器取服务，不写 HTTP 响应。
// 调用方需要自己兜底（如服务缺失时降级而非 500）时用这个，区别于 getService。
func getServiceRaw[T any](s *Server, name string) (T, error) {
	return app.GetAs[T](s.container, name)
}

// isValidUsername 校验用户名。username 会拼进数据目录 {data_dir}/workflows/{username}，
// 故只允许字母数字、下划线、短横线、点，拒绝 ".." 与路径分隔符以防穿越。
func isValidUsername(username string) bool {
	if username == "" || len(username) > maxUsernameLen {
		return false
	}
	if username == "." || strings.Contains(username, "..") {
		return false
	}
	for i := 0; i < len(username); i++ {
		c := username[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.':
		default:
			return false
		}
	}
	return true
}

// randRead 随机源，抽成变量便于测试失败路径
var randRead = rand.Read

// generateApiKey 生成 32 字符 hex API Key，与 config.conf 既有格式一致。
// 失败返回 error：静默返回空串会把空 Key 落盘，用户以为重置成功实则凭据丢失。
func generateApiKey() (string, error) {
	b := make([]byte, 16)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// stopUserEngine 停止并移除用户的规则引擎。
// 删租户前必须先停引擎，否则残留引擎继续跑 endpoint 占端口。
func (s *Server) stopUserEngine(username string) error {
	mgr, err := getServiceRaw[services.EngineManager](s, services.KeyEngineManager)
	if err != nil {
		// 引擎管理器未注册（如嵌入模式只用部分模块）：无引擎可停，不阻断删除
		return nil
	}
	return mgr.Remove(username)
}
