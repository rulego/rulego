package filestore

import (
	"sync"

	"gopkg.in/ini.v1"
)

// FileStorage INI file storage base class provides read and write operations for individual INI files.
// Thread-safe, supports grouping key-value pairs by section.
type FileStorage struct {
	filename string
	file     *ini.File
	lock     sync.RWMutex
}

// NewFileStorage creates or loads INI file storage.
// If the file does not exist, an empty file will be automatically created.
func NewFileStorage(filename string) (*FileStorage, error) {
	file, err := ini.LooseLoad(filename)
	if err != nil {
		return nil, err
	}
	return &FileStorage{filename: filename, file: file}, nil
}

// GetSection to get the specified partition
func (d *FileStorage) GetSection(sectionName string) (*ini.Section, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.file.GetSection(sectionName)
}

// Get the key value for the specified section
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

// GetAll retrieves all key-value pairs under the specified section
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

// Save saves a single key-value pair to a specified section; if the section does not exist, it is automatically created
func (d *FileStorage) Save(sectionName, key, value string) error {
	section := d.file.Section(sectionName)
	section.Key(key).SetValue(value)
	return d.SaveToFile()
}

// SaveList batch saves key values to a specified section
func (d *FileStorage) SaveList(sectionName string, values map[string]string) error {
	section := d.file.Section(sectionName)
	for key, value := range values {
		section.Key(key).SetValue(value)
	}
	return d.SaveToFile()
}

// Delete: Deletes the key under the specified section
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

// SaveToFile persists INI data from memory to disk files
func (d *FileStorage) SaveToFile() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.file.SaveTo(d.filename)
}

// FileStorageManager INI is responsible for lazily loading and caching multiple FileStorage instances.
// Avoid creating multiple storage instances for the same file.
type FileStorageManager struct {
	manager map[string]*FileStorage
	lock    sync.RWMutex
}

// NewFileStorageManager creates a file storage manager
func NewFileStorageManager() *FileStorageManager {
	return &FileStorageManager{
		manager: make(map[string]*FileStorage),
	}
}

// Init initializes the FileStorage and cache of the specified path
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

// Get the FileStorage at the specified path; if not cached, it will be automatically initialized
func (f *FileStorageManager) Get(filename string) (*FileStorage, error) {
	f.lock.RLock()
	fs, ok := f.manager[filename]
	f.lock.RUnlock()
	if ok {
		return fs, nil
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	// Double check: Another goroutine may have already been initialized
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

// Delete: removes the specified path from the cache of FileStorage
func (f *FileStorageManager) Delete(filename string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	delete(f.manager, filename)
}
