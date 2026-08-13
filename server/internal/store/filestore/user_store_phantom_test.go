package filestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
)

func newTestUserStore(t *testing.T) (*UserStore, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewUserStore(config.Config{DataDir: dir})
	if err != nil {
		t.Fatalf("NewUserStore error: %v", err)
	}
	return store, filepath.Join(dir, usersFileName)
}

// 回归：Get 不得产生写副作用。
// ini.Section.Key() 在键不存在时会 NewKey 注册空键，曾导致查询过的用户名
// 在下一次 Save 时被当作空条目写进 users.ini（用户列表出现重复/角色错乱）。
func TestGetUser_DoesNotCreatePhantomEntry(t *testing.T) {
	store, iniPath := newTestUserStore(t)

	// 查一个不存在的用户（真实路径：每次认证都会查一次）
	if _, ok := store.GetUser("ghost"); ok {
		t.Fatal("不存在的用户不应返回 ok")
	}
	// 触发落盘
	if err := store.CreateUser(model.User{Username: "real", Password: "pw", Roles: []string{"editor"}}); err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}

	raw, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("读 users.ini 失败: %v", err)
	}
	if strings.Contains(string(raw), "ghost") {
		t.Errorf("users.ini 出现幽灵条目 ghost:\n%s", raw)
	}

	if got := len(store.List()); got != 1 {
		t.Errorf("List() 返回 %d 条，want 1:\n%s", got, raw)
	}
}

// List 跳过空值条目，兼容历史文件里已存在的幽灵行
func TestList_SkipsEmptyLegacyEntries(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, usersFileName)
	// 模拟旧版本留下的文件：admin 是空值幽灵行
	content := "admin = \nreal = pw,key1,editor,\n"
	if err := os.WriteFile(iniPath, []byte(content), 0o600); err != nil {
		t.Fatalf("写测试文件失败: %v", err)
	}
	store, err := NewUserStore(config.Config{DataDir: dir})
	if err != nil {
		t.Fatalf("NewUserStore error: %v", err)
	}

	list := store.List()
	if len(list) != 1 {
		t.Fatalf("List() 返回 %d 条，want 1（应跳过空值 admin）: %+v", len(list), list)
	}
	if list[0].Username != "real" {
		t.Errorf("List()[0].Username = %q, want %q", list[0].Username, "real")
	}
}

// 密码必须散列落盘，且不影响登录校验
func TestCreateUser_HashesPasswordOnDisk(t *testing.T) {
	store, iniPath := newTestUserStore(t)
	if err := store.CreateUser(model.User{Username: "u1", Password: "plain-pw", Roles: []string{"editor"}}); err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}

	raw, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("读 users.ini 失败: %v", err)
	}
	if strings.Contains(string(raw), "plain-pw") {
		t.Errorf("密码以明文落盘:\n%s", raw)
	}
	if !store.ValidatePassword("u1", "plain-pw") {
		t.Error("散列后正确密码应校验通过")
	}
	if store.ValidatePassword("u1", "bad-pw") {
		t.Error("错误密码应拒绝")
	}
}

// 回填既有散列值时不得二次散列，否则用户改完别的字段就登不进来
func TestCreateUser_DoesNotDoubleHash(t *testing.T) {
	store, _ := newTestUserStore(t)
	if err := store.CreateUser(model.User{Username: "u1", Password: "pw", Roles: []string{"editor"}}); err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	existing, ok := store.GetUser("u1")
	if !ok {
		t.Fatal("GetUser 未命中")
	}
	// 模拟 POST /users 的更新路径：密码留空时回填已散列的旧值
	existing.Roles = []string{"viewer"}
	if err := store.CreateUser(existing); err != nil {
		t.Fatalf("CreateUser(更新) error: %v", err)
	}
	if !store.ValidatePassword("u1", "pw") {
		t.Error("回填散列值后原密码应仍可登录")
	}
}

// 停用用户一律拒绝，且散列格式下也生效
func TestValidatePassword_DisabledUser(t *testing.T) {
	store, _ := newTestUserStore(t)
	if err := store.CreateUser(model.User{Username: "u1", Password: "pw", Disabled: true}); err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	if store.ValidatePassword("u1", "pw") {
		t.Error("已停用用户不应通过校验")
	}
}
