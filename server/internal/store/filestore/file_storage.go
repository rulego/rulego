package filestore

import (
	"sync"

	"gopkg.in/ini.v1"
)

// FileStorage INI 文件存储基类，提供对单个 INI 文件的读写操作。
// 线程安全，支持按 section 分组存储键值对。
type FileStorage struct {
	filename string
	file     *ini.File
	lock     sync.RWMutex
}

// NewFileStorage 创建或加载 INI 文件存储。
// 如果文件不存在则自动创建空文件。
func NewFileStorage(filename string) (*FileStorage, error) {
	file, err := ini.LooseLoad(filename)
	if err != nil {
		return nil, err
	}
	return &FileStorage{filename: filename, file: file}, nil
}

// GetSection 获取指定分区
func (d *FileStorage) GetSection(sectionName string) (*ini.Section, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.file.GetSection(sectionName)
}

// Get 获取指定 section 下的键值
func (d *FileStorage) Get(sectionName string, keyName string) string {
	d.lock.RLock()
	defer d.lock.RUnlock()
	if fs, err := d.file.GetSection(sectionName); err != nil {
		return ""
	} else if key := fs.Key(keyName); key != nil {
		return key.Value()
	} else {
		return ""
	}
}

// GetAll 获取指定 section 下的所有键值对
func (d *FileStorage) GetAll(sectionName string) map[string]string {
	d.lock.RLock()
	defer d.lock.RUnlock()
	values := make(map[string]string)
	if s, _ := d.file.GetSection(sectionName); s != nil {
		for _, k := range s.Keys() {
			values[k.Name()] = k.Value()
		}
	}
	return values
}

// Save 保存单个键值对到指定 section，如果 section 不存在则自动创建
func (d *FileStorage) Save(sectionName, key, value string) error {
	section := d.file.Section(sectionName)
	section.Key(key).SetValue(value)
	return d.SaveToFile()
}

// SaveList 批量保存键值对到指定 section
func (d *FileStorage) SaveList(sectionName string, values map[string]string) error {
	section := d.file.Section(sectionName)
	for key, value := range values {
		section.Key(key).SetValue(value)
	}
	return d.SaveToFile()
}

// Delete 删除指定 section 下的键
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

// SaveToFile 将内存中的 INI 数据持久化到磁盘文件
func (d *FileStorage) SaveToFile() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.file.SaveTo(d.filename)
}

// FileStorageManager INI 文件存储管理器，负责懒加载和缓存多个 FileStorage 实例。
// 避免对同一文件重复创建存储实例。
type FileStorageManager struct {
	manager map[string]*FileStorage
	lock    sync.RWMutex
}

// NewFileStorageManager 创建文件存储管理器
func NewFileStorageManager() *FileStorageManager {
	return &FileStorageManager{
		manager: make(map[string]*FileStorage),
	}
}

// Init 初始化指定路径的 FileStorage 并缓存
func (f *FileStorageManager) Init(filename string) (*FileStorage, error) {
	fs, err := NewFileStorage(filename)
	if err != nil {
		return nil, err
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	f.manager[filename] = fs
	return fs, nil
}

// Get 获取指定路径的 FileStorage，如果未缓存则自动初始化
func (f *FileStorageManager) Get(filename string) (*FileStorage, error) {
	f.lock.RLock()
	fs, ok := f.manager[filename]
	f.lock.RUnlock()
	if ok {
		return fs, nil
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	// 双重检查：可能另一个 goroutine 已经初始化
	if fs, ok := f.manager[filename]; ok {
		return fs, nil
	}
	fs, err := NewFileStorage(filename)
	if err != nil {
		return nil, err
	}
	f.manager[filename] = fs
	return fs, nil
}

// Delete 从缓存中移除指定路径的 FileStorage
func (f *FileStorageManager) Delete(filename string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	delete(f.manager, filename)
}
