package file

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WithTimestamp 包含文件路径和解析出的时间戳
type WithTimestamp struct {
	Path      string
	Timestamp time.Time
}

// SortFilesByTimestamp 解析文件列表中的时间戳，返回按时间戳排序的文件列表
func SortFilesByTimestamp(files []string) []WithTimestamp {
	var fileWithTimestamps []WithTimestamp
	for _, file := range files {
		timestamp, err := parseTimestampFromFilename(file)
		if err != nil {
			timestamp = time.Now()
		}
		fileWithTimestamps = append(fileWithTimestamps, WithTimestamp{Path: file, Timestamp: timestamp})
	}
	// 使用 sort.Sort 进行排序
	sort.Sort(ByTimestamp(fileWithTimestamps))
	return fileWithTimestamps
}

// 解析文件名中的时间戳
func parseTimestampFromFilename(filename string) (time.Time, error) {
	// 使用filepath.Base获取文件名
	lastPart := filepath.Base(filename)
	timestampStr := strings.Split(lastPart, "_")[0]
	return time.Parse("20060102150405000", timestampStr)
}

// ByTimestamp 实现 sort.Interface 接口
type ByTimestamp []WithTimestamp

func (f ByTimestamp) Len() int           { return len(f) }
func (f ByTimestamp) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByTimestamp) Less(i, j int) bool { return f[i].Timestamp.After(f[j].Timestamp) }
