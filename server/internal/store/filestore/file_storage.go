package filestore

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/ini.v1"
)

// FileStorage 单个 INI 文件的线程安全读写，按 section 分组键值对。
type FileStorage struct {
	filename string
	file     *ini.File
	lock     sync.RWMutex
}

// NewFileStorage 加载 INI 文件，文件不存在时按空文件处理（ini.LooseLoad 不报错）。
func NewFileStorage(filename string) (*FileStorage, error) {
	file, err := ini.LooseLoad(filename)
	if err != nil {
		return nil, err
	}
	return &FileStorage{filename: filename, file: file}, nil
}

// GetSection 返回指定分区的 *ini.Section。
// 返回值指向锁内共享结构，脱离本类型锁保护：调用方只读，改值走 Save/SaveList/Delete，
// 否则与并发写竞争。
func (d *FileStorage) GetSection(sectionName string) (*ini.Section, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.file.GetSection(sectionName)
}

// Get 读取键值，键不存在返回空串。
// 必须先 HasKey 再取值：ini.Section.Key() 对不存在键会 NewKey 注册空键，
// 让读产生写副作用——下次 Save 会把这些空键落盘（users.ini 曾因此冒出幽灵条目），
// 且在 RLock 下改写 ini 还构成数据竞争。
func (d *FileStorage) Get(sectionName string, keyName string) string {
	d.lock.RLock()
	defer d.lock.RUnlock()
	fs, err := d.file.GetSection(sectionName)
	if err != nil || fs == nil {
		return ""
	}
	if !fs.HasKey(keyName) {
		return ""
	}
	return fs.Key(keyName).Value()
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

// Save 写入单个键值对（section 不存在自动创建），改内存与落盘在同一写锁内。
// ini.File 非并发安全：若只在落盘时加锁，改写会与并发 Get/GetAll 竞争内部 map。
func (d *FileStorage) Save(sectionName, key, value string) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.file.Section(sectionName).Key(key).SetValue(value)
	return d.saveToFileLocked()
}

// SaveList 批量保存键值对到指定 section
func (d *FileStorage) SaveList(sectionName string, values map[string]string) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	section := d.file.Section(sectionName)
	for key, value := range values {
		section.Key(key).SetValue(value)
	}
	return d.saveToFileLocked()
}

// Delete 删除指定 section 下的键
func (d *FileStorage) Delete(sectionName string, keys ...string) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if !d.file.HasSection(sectionName) {
		return nil
	}
	section := d.file.Section(sectionName)
	for _, key := range keys {
		section.DeleteKey(key)
	}
	return d.saveToFileLocked()
}

// SaveToFile 将内存中的 INI 数据持久化到磁盘文件
func (d *FileStorage) SaveToFile() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.saveToFileLocked()
}

// saveToFileLocked 落盘，调用方须已持写锁：序列化遍历 ini 内部结构，
// 必须与改写处同处一个临界区。经临时文件+rename 原子替换，
// 避免崩溃时 users.ini 半写导致凭据丢失。
func (d *FileStorage) saveToFileLocked() error {
	tmp, err := os.CreateTemp(filepath.Dir(d.filename), filepath.Base(d.filename)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = d.file.WriteTo(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, d.filename)
}

// FileStorageManager 按文件路径懒加载并缓存 FileStorage，避免对同一文件重复建实例。
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
