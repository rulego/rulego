/*
 * Copyright 2025 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// File defines the interface for file storage
// Provides file based storage and retrieval functionality
// Implementation classes must ensure thread safety
type File interface {
	// Save stores a file in storage
	// Parameters:
	//   - path: file path (string)
	//   - data: file content ([]byte)
	// Returns:
	//   - error: returns error if save fails
	Save(path string, data []byte) error
	// Get retrieves a file from storage by path
	// Parameters:
	//   - path: file path to lookup (string)
	// Returns:
	//   - []byte: file content
	//   - error: returns error if not exists or other error
	Get(path string) ([]byte, error)
	// Delete removes a file by path
	// Parameters:
	//   - path: file path to delete (string)
	// Returns:
	//   - error: returns error if delete fails
	Delete(path string) error
	// SaveAppend appends data to a file in storage
	// Parameters:
	//   - path: file path (string)
	//   - data: file content ([]byte)
	// Returns:
	//   - error: returns error if save fails
	SaveAppend(path string, data []byte) error
	// GetFilePaths returns file paths matching the pattern
	// Parameters:
	//   - loadFilePattern: glob pattern for files
	//   - excludedPatterns: patterns to exclude
	// Returns:
	//   - []string: list of matching file paths
	//   - error: returns error if list fails
	GetFilePaths(loadFilePattern string, excludedPatterns ...string) ([]string, error)
	// IsExist checks if a path exists
	// Parameters:
	//   - path: file path (string)
	// Returns:
	//   - bool: true if exists, false otherwise
	IsExist(path string) bool
	// CreateDirs creates directories recursively
	// Parameters:
	//   - path: directory path (string)
	// Returns:
	//   - error: returns error if creation fails
	CreateDirs(path string) error
	// Name returns the name of the storage
	Name() string
}

// LocalFileStorage implements types.File using local file system
type LocalFileStorage struct {
	name string
}

func NewLocalFileStorage() *LocalFileStorage {
	return &LocalFileStorage{name: "default"}
}

func (f *LocalFileStorage) Name() string {
	return f.name
}

func (f *LocalFileStorage) Get(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *LocalFileStorage) Save(path string, data []byte) error {
	// Create dirs if not exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (f *LocalFileStorage) SaveAppend(path string, data []byte) error {
	// Create dirs if not exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Open file in append mode, create if not exists
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func (f *LocalFileStorage) Delete(path string) error {
	return os.Remove(path)
}

// GetFilePaths returns file paths matching the pattern
func (f *LocalFileStorage) GetFilePaths(loadFilePattern string, excludedPatterns ...string) ([]string, error) {
	// 分割输入参数为目录和文件名
	dir, file := filepath.Split(loadFilePattern)
	var paths []string
	// 遍历目录
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 如果是文件，且文件名匹配输入参数
		if !d.IsDir() {
			matched, _ := filepath.Match(file, d.Name())
			if matched && !isMatch(d, excludedPatterns...) {
				paths = append(paths, path)
			}
		} else {
			for _, item := range excludedPatterns {
				if matched, _ := filepath.Match(item, d.Name()); matched {
					return filepath.SkipDir // 跳过该子目录
				}
			}

		}

		return nil
	})
	return paths, err
}

// IsExist checks if a path exists
func (f *LocalFileStorage) IsExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		} else if os.IsNotExist(err) {
			return false
		} else {
			return false
		}
	}
	return true
}

// CreateDirs creates directories recursively
func (f *LocalFileStorage) CreateDirs(path string) error {
	if !f.IsExist(path) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func isMatch(d os.DirEntry, patterns ...string) bool {
	for _, item := range patterns {
		if matched, _ := filepath.Match(item, d.Name()); matched {
			return true
		}
	}
	return false
}

var DefaultFile = NewLocalFileStorage()
