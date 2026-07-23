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

// SettingStore is implemented based on user settings storage based on INI files.
// Each user directory has a settings.ini file to store user settings.
type SettingStore struct {
	Config config.Config
	fs     *FileStorage
}

// NewSettingStore creates user settings file storage
// namespace is the path to the user's directory
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

// Save: Save the key-value pair
func (d *SettingStore) Save(key, value string) error {
	return d.fs.Save(settingsSectionName, key, value)
}

// Delete the settings
func (d *SettingStore) Delete(key string) error {
	return d.fs.Delete(settingsSectionName, key)
}

// Get the set value and automatically remove whitespaces
func (d *SettingStore) Get(key string) string {
	return strings.TrimSpace(d.fs.Get(settingsSectionName, key))
}

// Setting: Retrieves the complete user settings structure
func (d *SettingStore) Setting() model.UserSetting {
	var setting model.UserSetting
	values := d.fs.GetAll(settingsSectionName)
	_ = maps.Map2Struct(values, &setting)
	return setting
}
