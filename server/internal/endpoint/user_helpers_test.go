package endpoint

import (
	"errors"
	"testing"

	"github.com/rulego/rulego/server/config"
)

func TestGenerateApiKey(t *testing.T) {
	key, err := generateApiKey()
	if err != nil {
		t.Fatalf("generateApiKey() error = %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key 长度 = %d, want 32", len(key))
	}
}

// rand 失败必须返回 error 而非空串：静默落盘空 ApiKey 会让用户以为重置成功，
// 实则凭据丢失。
func TestGenerateApiKey_RandFailure(t *testing.T) {
	orig := randRead
	randRead = func(b []byte) (int, error) { return 0, errors.New("entropy source broken") }
	defer func() { randRead = orig }()

	key, err := generateApiKey()
	if err == nil {
		t.Error("rand 失败时应返回 error")
	}
	if key != "" {
		t.Errorf("rand 失败时 key 应为空, got %q", key)
	}
}

// 删除用户的目标守卫：操作者自身、默认租户、config.conf 内置账号都不可删。
// 内置账号的 store 删除是假成功（无键可删），密码仍在配置里可登录。
func TestValidateDeleteTarget(t *testing.T) {
	s := &Server{config: &config.Config{DefaultUsername: "admin"}}
	s.config.Users = map[string]string{"admin": "pass,key", "ops": "pass2"}
	s.config.InitUserMap()

	cases := []struct {
		name     string
		target   string
		operator string
		wantErr  bool
	}{
		{"空用户名", "", "admin", true},
		{"删自己", "admin", "admin", true},
		{"删默认租户", "admin", "other", true},
		{"删 config 账号", "ops", "other", true},
		{"删普通 store 用户", "storeuser", "other", false},
	}
	for _, c := range cases {
		err := s.validateDeleteTarget(c.target, c.operator)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: validateDeleteTarget(%q,%q) = %v, wantErr %v", c.name, c.target, c.operator, err, c.wantErr)
		}
	}
}
