package dao

import (
	"gopkg.in/ini.v1"
	"sync"
)

type FileStorage struct {
	filename string
	file     *ini.File
	lock     sync.RWMutex
}

func NewFileStorage(filename string) (*FileStorage, error) {
	file, err := ini.LooseLoad(filename)
	if err != nil {
		return nil, err
	}
	return &FileStorage{filename: filename, file: file}, nil
}

// GetSection 获取分区
func (d *FileStorage) GetSection(sectionName string) (*ini.Section, error) {
	return d.file.GetSection(sectionName)
}
func (d *FileStorage) Get(sectionName string, keyName string) string {
	if fs, err := d.file.GetSection(sectionName); err != nil {
		return ""
	} else if key := fs.Key(keyName); key != nil {
		return key.Value()
	} else {
		return ""
	}
}
func (d *FileStorage) GetAll(sectionName string) map[string]string {
	values := make(map[string]string)
	if s, _ := d.GetSection(sectionName); s != nil {
		for _, k := range s.Keys() {
			values[k.Name()] = k.Value()
		}
	}
	return values
}

// Save 保存单个值
func (d *FileStorage) Save(sectionName, key, value string) error {
	section := d.file.Section(sectionName) // 如果分区不存在，将会创建一个新的分区
	section.Key(key).SetValue(value)
	return d.SaveToFile()
}

// SaveList 保存多个值
func (d *FileStorage) SaveList(sectionName string, values map[string]string) error {
	section := d.file.Section(sectionName) // 如果分区不存在，将会创建一个新的分区
	for key, value := range values {
		section.Key(key).SetValue(value)
	}
	return d.SaveToFile()
}

// Delete 删除
func (d *FileStorage) Delete(sectionName string, keys ...string) error {
	if !d.file.HasSection(sectionName) {
		return nil
	}
	section := d.file.Section(sectionName)
	for _, key := range keys {
		section.DeleteKey(key)
	}
	return d.SaveToFile()
}

// SaveToFile 保存
func (d *FileStorage) SaveToFile() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.file.SaveTo(d.filename)
}

//var FileStorageManager =NewFileStorageManager()

type FileStorageManager struct {
	// 文件存储 key=路径
	manager map[string]*FileStorage
	lock    sync.RWMutex
}

func NewFileStorageManager() *FileStorageManager {
	return &FileStorageManager{
		manager: make(map[string]*FileStorage),
	}
}

func (f *FileStorageManager) Init(filename string) (*FileStorage, error) {
	fs, err := NewFileStorage(filename)
	if err != nil {
		return nil, err
	} else {
		f.lock.Lock()
		defer f.lock.Unlock()
		f.manager[filename] = fs
	}
	return fs, nil
}

func (f *FileStorageManager) Get(filename string) (*FileStorage, error) {
	f.lock.RLock()
	fs, ok := f.manager[filename]
	f.lock.RUnlock()
	if ok {
		return fs, nil
	}
	return f.Init(filename)
}

func (f *FileStorageManager) Delete(filename string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	delete(f.manager, filename)
}
