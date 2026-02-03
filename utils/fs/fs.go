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

var storage File = DefaultFile

// SaveFile A function that saves a file to a given path, overwriting it if it exists
func SaveFile(path string, data []byte) error {
	return storage.Save(path, data)
}

// LoadFile 加载文件
func LoadFile(filePath string) []byte {
	data, _ := storage.Get(filePath)
	return data
}

// GetFilePaths 返回匹配的文件路径列表
func GetFilePaths(loadFilePattern string, excludedPatterns ...string) ([]string, error) {
	return storage.GetFilePaths(loadFilePattern, excludedPatterns...)
}

// IsExist 判断路径是否存在
func IsExist(path string) bool {
	return storage.IsExist(path)
}

// CreateDirs 创建文件夹
func CreateDirs(path string) error {
	return storage.CreateDirs(path)
}

func SetStorage(fs File) {
	storage = fs
}

func GetStorage() File {
	return storage
}
