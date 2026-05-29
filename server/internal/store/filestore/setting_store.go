package filestore

import (
	"path"
	"strings"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/utils/maps"
)

const (
	settingsSectionName = ""
	settingsFileName    = "settings.ini"
)

// SettingStore 基于 INI 文件的用户设置存储实现。
// 每个用户目录下有一个 settings.ini 文件存储用户设置。
type SettingStore struct {
	Config config.Config
	fs     *FileStorage
}

// NewSettingStore 创建用户设置文件存储
// namespace 为用户目录路径
func NewSettingStore(cfg config.Config, namespace string) (*SettingStore, error) {
	fs, err := NewFileStorage(path.Join(namespace, settingsFileName))
	if err != nil {
		return nil, err
	}
	return &SettingStore{
		Config: cfg,
		fs:     fs,
	}, nil
}

// Save 保存设置键值对
func (d *SettingStore) Save(key, value string) error {
	return d.fs.Save(settingsSectionName, key, value)
}

// Delete 删除设置
func (d *SettingStore) Delete(key string) error {
	return d.fs.Delete(settingsSectionName, key)
}

// Get 获取设置值，自动去除空白字符
func (d *SettingStore) Get(key string) string {
	return strings.TrimSpace(d.fs.Get(settingsSectionName, key))
}

// Setting 获取完整的用户设置结构体
func (d *SettingStore) Setting() model.UserSetting {
	var setting model.UserSetting
	values := d.fs.GetAll(settingsSectionName)
	_ = maps.Map2Struct(values, &setting)
	return setting
}
