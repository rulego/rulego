package file

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WriteFileAtomic 原子写文件：同目录临时文件 + Sync + Rename。
// 崩溃时刻目标文件只可能是旧内容或完整新内容，不会留下半写状态
//（直接 O_TRUNC 写在断电/崩溃时会损坏 index/DSL 等无法自愈的文件）。
func WriteFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
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
	// CreateTemp 创建的文件固定 0600，rename 前恢复调用方期望的权限
	if err = os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// WithTimestamp 包含文件路径和解析出的时间戳
type WithTimestamp struct {
	Path      string
	Timestamp time.Time
}

// SortFilesByTimestamp 解析文件列表中的时间戳，返回按时间戳排序的文件列表
func SortFilesByTimestamp(files []string) []WithTimestamp {
	var fileWithTimestamps []WithTimestamp
	for _, f := range files {
		timestamp, err := parseTimestampFromFilename(f)
		if err != nil {
			timestamp = time.Now()
		}
		fileWithTimestamps = append(fileWithTimestamps, WithTimestamp{Path: f, Timestamp: timestamp})
	}
	sort.Sort(ByTimestamp(fileWithTimestamps))
	return fileWithTimestamps
}

func parseTimestampFromFilename(filename string) (time.Time, error) {
	lastPart := filepath.Base(filename)
	timestampStr := strings.Split(lastPart, "_")[0]
	return time.Parse("20060102150405000", timestampStr)
}

// ByTimestamp 实现 sort.Interface 接口
type ByTimestamp []WithTimestamp

func (f ByTimestamp) Len() int           { return len(f) }
func (f ByTimestamp) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByTimestamp) Less(i, j int) bool { return f[i].Timestamp.After(f[j].Timestamp) }
