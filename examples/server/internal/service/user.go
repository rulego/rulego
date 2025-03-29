package service

import (
	"examples/server/config"
	"examples/server/internal/dao"
)

var UserServiceImpl *UserService

type UserService struct {
	UserDao *dao.UserDao
	Config  config.Config
}

func NewUserService(config config.Config) (*UserService, error) {
	if userDao, err := dao.NewUserDao(config); err != nil {
		return nil, err
	} else {
		return &UserService{
			UserDao: userDao,
			Config:  config,
		}, nil
	}
}

func (s *UserService) CheckPassword(username, password string) bool {
	if username == "" {
		return false
	}
	return s.Config.CheckPassword(username, password)
}

func (s *UserService) GetUsernameByApiKey(apikey string) string {
	if apikey == "" {
		return ""
	}
	return s.Config.GetUsernameByApiKey(apikey)

}

func (s *UserService) GetApiKeyByUsername(username string) string {
	if username == "" {
		return ""
	}
	return s.Config.GetApiKeyByUsername(username)

}
