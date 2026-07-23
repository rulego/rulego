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

// UserStore is a user storage implementation based on INI files.
// User data is stored in {data_dir}/users.ini.
type UserStore struct {
	Config config.Config
	fs     *FileStorage
}

// NewUserStore creates user file storage
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

// CreateUser creates a user and writes the username and password into the INI file
func (d *UserStore) CreateUser(user model.User) error {
	return d.fs.Save(usersSectionName, user.Username, user.Password)
}

// ValidatePassword verifies whether the username and password match
func (d *UserStore) ValidatePassword(username, password string) bool {
	if v := d.fs.Get(usersSectionName, username); v == "" {
		return false
	} else {
		return v == password
	}
}

// Delete: Remove the user
func (d *UserStore) Delete(username string) error {
	return d.fs.Delete(usersSectionName, username)
}

// List lists all users
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
