package dao

import (
	"examples/server/config"
	"examples/server/internal/model"
	"github.com/rulego/rulego/utils/maps"
	"path"
	"strings"
)

const (
	SettingsSectionName = ""
	SettingsFileName    = "settings.ini"
)

type UserSettingDao struct {
	Config config.Config
	fs     *FileStorage
}

func NewUserSettingDao(config config.Config, namespace string) (*UserSettingDao, error) {
	fs, err := NewFileStorage(path.Join(namespace, SettingsFileName))
	if err != nil {
		return nil, err
	}
	return &UserSettingDao{
		Config: config,
		fs:     fs,
	}, nil
}

func (d *UserSettingDao) Save(key, value string) error {
	return d.fs.Save(SettingsSectionName, key, value)
}

func (d *UserSettingDao) Delete(key string) error {
	return d.fs.Delete(SettingsSectionName, key)
}
func (d *UserSettingDao) Get(key string) string {
	return strings.TrimSpace(d.fs.Get(SettingsSectionName, key))
}

func (d *UserSettingDao) Setting() model.UserSetting {
	var setting model.UserSetting
	values := d.fs.GetAll(SettingsSectionName)
	_ = maps.Map2Struct(values, &setting)
	return setting
}
