package filestore

import (
	"path"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
)

const (
	usersSectionName = ""
	usersFileName    = "users.ini"
)

// UserStore 基于 INI 文件的用户存储实现。
// 用户数据存储在 {data_dir}/users.ini 中。
type UserStore struct {
	Config config.Config
	fs     *FileStorage
}

// NewUserStore 创建用户文件存储
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

// CreateUser 创建用户，将用户名和密码写入 INI 文件
func (d *UserStore) CreateUser(user model.User) error {
	return d.fs.Save(usersSectionName, user.Username, user.Password)
}

// ValidatePassword 验证用户名和密码是否匹配
func (d *UserStore) ValidatePassword(username, password string) bool {
	if v := d.fs.Get(usersSectionName, username); v == "" {
		return false
	} else {
		return v == password
	}
}

// Delete 删除用户
func (d *UserStore) Delete(username string) error {
	return d.fs.Delete(usersSectionName, username)
}

// List 列出所有用户
func (d *UserStore) List() []model.User {
	var users []model.User
	values := d.fs.GetAll(usersSectionName)
	for key, value := range values {
		users = append(users, model.User{
			Username: key,
			Password: value,
		})
	}
	return users
}
