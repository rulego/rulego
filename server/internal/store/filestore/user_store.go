package filestore

import (
	"errors"
	"path"
	"strings"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
)

const (
	usersSectionName = ""
	usersFileName    = "users.ini"
)

// UserStore 基于 INI 文件的用户存储，数据落在 {data_dir}/users.ini。
type UserStore struct {
	Config config.Config
	fs     *FileStorage
}

// NewUserStore 创建用户存储，文件位于 cfg.DataDir/users.ini。
func NewUserStore(cfg config.Config) (*UserStore, error) {
	fs, err := NewFileStorage(path.Join(cfg.DataDir, usersFileName))
	if err != nil {
		return nil, err
	}
	return &UserStore{
		Config: cfg,
		fs:     fs,
	}, nil
}

// 行格式：password,apiKey,role1|role2,disabled
// 与 config.conf 的 [users] 段（password,apiKey）保持同风格，向后兼容只有 password 的老行。
func encodeUser(u model.User) string {
	roles := strings.Join(u.Roles, "|")
	disabled := ""
	if u.Disabled {
		disabled = "1"
	}
	return strings.Join([]string{u.Password, u.ApiKey, roles, disabled}, ",")
}

func decodeUser(username, raw string) model.User {
	parts := strings.Split(raw, ",")
	u := model.User{Username: username}
	if len(parts) > 0 {
		u.Password = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		u.ApiKey = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		if r := strings.TrimSpace(parts[2]); r != "" {
			for _, item := range strings.Split(r, "|") {
				if item = strings.TrimSpace(item); item != "" {
					u.Roles = append(u.Roles, item)
				}
			}
		}
	}
	if len(parts) > 3 {
		u.Disabled = strings.TrimSpace(parts[3]) == "1"
	}
	// 老格式（只有密码）没有角色，视为 editor：能读写自己租户，但不能管用户
	if len(u.Roles) == 0 {
		u.Roles = []string{model.RoleEditor}
	}
	return u
}

// CreateUser 创建或更新用户，写入 INI 文件。
// 密码落盘前散列；已是散列值的原样保留（调用方回填既有密码时不重复散列）。
func (d *UserStore) CreateUser(user model.User) error {
	if user.Username == "" {
		return errors.New("username is required")
	}
	if user.Password != "" && !IsHashedPassword(user.Password) {
		hashed, err := HashPassword(user.Password)
		if err != nil {
			return err
		}
		user.Password = hashed
	}
	return d.fs.Save(usersSectionName, user.Username, encodeUser(user))
}

// GetUser 获取单个用户
func (d *UserStore) GetUser(username string) (model.User, bool) {
	if username == "" {
		return model.User{}, false
	}
	raw := d.fs.Get(usersSectionName, username)
	if raw == "" {
		return model.User{}, false
	}
	return decodeUser(username, raw), true
}

// GetUsernameByApiKey 通过 API Key 反查用户名
func (d *UserStore) GetUsernameByApiKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	for _, u := range d.List() {
		if u.ApiKey == apiKey && !u.Disabled {
			return u.Username
		}
	}
	return ""
}

// ValidatePassword 验证用户名/密码是否匹配，已停用用户一律拒绝。
func (d *UserStore) ValidatePassword(username, password string) bool {
	u, ok := d.GetUser(username)
	if !ok || u.Disabled || u.Password == "" {
		return false
	}
	return VerifyPassword(u.Password, password)
}

// Delete 删除用户
func (d *UserStore) Delete(username string) error {
	return d.fs.Delete(usersSectionName, username)
}

// List 列出所有用户。
// 跳过空值条目以兼容历史遗留的空键——旧版 Get 的写副作用产生过这类幽灵行。
func (d *UserStore) List() []model.User {
	var users []model.User
	values := d.fs.GetAll(usersSectionName)
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		users = append(users, decodeUser(key, value))
	}
	return users
}
