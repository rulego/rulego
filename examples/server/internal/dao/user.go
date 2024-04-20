package dao

import (
	"examples/server/config"
	"examples/server/internal/model"
	"path"
)

const (
	UsersSectionName = ""
	UsersFileName    = "users.ini"
)

type UserDao struct {
	Config config.Config
	fs     *FileStorage
}

func NewUserDao(config config.Config) (*UserDao, error) {
	fs, err := NewFileStorage(path.Join(config.DataDir, UsersFileName))
	if err != nil {
		return nil, err
	}
	return &UserDao{
		Config: config,
		fs:     fs,
	}, nil
}

func (d *UserDao) CreateUser(user model.User) error {
	return d.fs.Save(UsersSectionName, user.Username, user.Password)
}

// ValidatePassword 验证密码
func (d *UserDao) ValidatePassword(username, password string) bool {
	if v := d.fs.Get(UsersSectionName, username); v == "" {
		return false
	} else {
		return v == password
	}
}

func (d *UserDao) Delete(username string) error {
	return d.fs.Delete(UsersSectionName, username)
}

func (d *UserDao) List() []model.User {
	var users []model.User
	values := d.fs.GetAll(UsersSectionName)
	for key, value := range values {
		users = append(users, model.User{
			Username: key,
			Password: value,
		})
	}
	return users
}
