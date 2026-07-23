package file

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WithTimestamp contains the file path and the parsed timestamp
type WithTimestamp struct {
	Path      string
	Timestamp time.Time
}

// SortFilesByTimestamp parses the timestamp in the file list and returns a list of files sorted by timestamps
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

// ByTimestamp implements sort.Interface
type ByTimestamp []WithTimestamp

func (f ByTimestamp) Len() int           { return len(f) }
func (f ByTimestamp) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByTimestamp) Less(i, j int) bool { return f[i].Timestamp.After(f[j].Timestamp) }
