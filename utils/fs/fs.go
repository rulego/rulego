package fs

import (
	"io/fs"
	"os"
	"path/filepath"
)

//SaveFile A function that saves a file to a given path, overwriting it if it exists
func SaveFile(path string, data []byte) error {
	// Open the file with write-only and create flags, and 0666 permission
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	// Write the data to the file
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}

//LoadFile 加载文件
func LoadFile(filePath string) []byte {
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	} else {
		return buf
	}
}

//GetFilePaths 返回匹配的文件路径列表
func GetFilePaths(loadFilePattern string, excludedPatterns ...string) ([]string, error) {
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

//IsExist 判断路径是否存在
func IsExist(path string) bool {
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

//CreateDirs 创建文件夹
func CreateDirs(path string) error {
	if !IsExist(path) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
func isMatch(d fs.DirEntry, patterns ...string) bool {
	for _, item := range patterns {
		if matched, _ := filepath.Match(item, d.Name()); matched {
			return true
		}
	}
	return false
}
