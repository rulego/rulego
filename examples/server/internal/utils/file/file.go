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
	for _, file := range files {
		timestamp, err := parseTimestampFromFilename(file)
		if err != nil {
			timestamp = time.Now()
		}
		fileWithTimestamps = append(fileWithTimestamps, WithTimestamp{Path: file, Timestamp: timestamp})
	}
	// Use sort.Sort to sort the order
	sort.Sort(ByTimestamp(fileWithTimestamps))
	return fileWithTimestamps
}

// Parse timestamps in the filename
func parseTimestampFromFilename(filename string) (time.Time, error) {
	// Using filepath.Base to get the file name
	lastPart := filepath.Base(filename)
	timestampStr := strings.Split(lastPart, "_")[0]
	return time.Parse("20060102150405000", timestampStr)
}

// ByTimestamp implements sort.Interface
type ByTimestamp []WithTimestamp

func (f ByTimestamp) Len() int           { return len(f) }
func (f ByTimestamp) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByTimestamp) Less(i, j int) bool { return f[i].Timestamp.After(f[j].Timestamp) }
