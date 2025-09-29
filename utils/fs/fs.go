/*
 * Copyright 2024 The RuleGo Authors.
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

// Package fs provides file system related utilities for the RuleGo project.
// It includes functions for saving and loading files, as well as retrieving file paths
// based on patterns. These utilities are designed to simplify file operations
// within the RuleGo ecosystem.
package fs

import (
	"bufio"
	"fmt"
	"github.com/spf13/afero"
	"os"
	"path/filepath"
)

var vfs = afero.NewOsFs() //local file

func SetFileAdapter(fs afero.Fs) {
	fmt.Printf("change vfs: %v -> %v \n", vfs.Name(), fs.Name())
	vfs = fs
}
func GetFileAdapter() afero.Fs {
	return vfs
}

// SaveFile A function that saves a file to a given path, overwriting it if it exists
func SaveFile(path string, data []byte) error {
	// Create or truncate the file
	file, err := vfs.Create(path) //os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the data to the file using buffered writer for better performance
	writer := bufio.NewWriter(file)
	_, err = writer.Write(data)
	if err != nil {
		return err
	}

	// Flush the buffered data to the file
	err = writer.Flush()
	if err != nil {
		return err
	}

	return nil
}

// LoadFile 加载文件
func LoadFile(filePath string) []byte {
	buf, err := afero.ReadFile(vfs, filePath) //os.ReadFile(filePath)
	if err != nil {
		return nil
	} else {
		return buf
	}
}

// GetFilePaths 返回匹配的文件路径列表
func GetFilePaths(loadFilePattern string, excludedPatterns ...string) ([]string, error) {
	// 分割输入参数为目录和文件名
	dir, file := filepath.Split(loadFilePattern)
	var paths []string
	// 遍历目录
	err := afero.Walk(vfs, dir, func(path string, d os.FileInfo, err error) error {
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

// IsExist 判断路径是否存在
func IsExist(path string) bool {
	_, err := vfs.Stat(path) //os.Stat(path)
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

// CreateDirs 创建文件夹
func CreateDirs(path string) error {
	if !IsExist(path) {
		err := vfs.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
func isMatch(d os.FileInfo, patterns ...string) bool {
	for _, item := range patterns {
		if matched, _ := filepath.Match(item, d.Name()); matched {
			return true
		}
	}
	return false
}
