package fs

import (
	"io/fs"
	"os"
	"path/filepath"
)

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
func isMatch(d fs.DirEntry, patterns ...string) bool {
	for _, item := range patterns {
		if matched, _ := filepath.Match(item, d.Name()); matched {
			return true
		}
	}
	return false
}
