package filestore

import (
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// 回归：ini.File 不是并发安全的。Save/SaveList/Delete 曾只在落盘时加锁，
// 改内存那几行裸跑，与并发的 Get/GetAll 竞争同一批 map
// （可触发 fatal error: concurrent map read and map write）。
// 本测试并发混跑读写，配 -race 使用；无 -race 时靠 map 竞争的运行时检查兜底。
func TestFileStorage_ConcurrentReadWrite(t *testing.T) {
	fs, err := NewFileStorage(filepath.Join(t.TempDir(), "race.ini"))
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}

	const (
		writers = 8
		readers = 8
		rounds  = 60
	)
	var wg sync.WaitGroup

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				key := "k" + strconv.Itoa(id) + "_" + strconv.Itoa(i)
				if err := fs.Save("sec", key, "v"+strconv.Itoa(i)); err != nil {
					t.Errorf("Save error: %v", err)
					return
				}
				if err := fs.SaveList("sec", map[string]string{
					"batch" + strconv.Itoa(id): strconv.Itoa(i),
				}); err != nil {
					t.Errorf("SaveList error: %v", err)
					return
				}
				if err := fs.Delete("sec", key); err != nil {
					t.Errorf("Delete error: %v", err)
					return
				}
			}
		}(w)
	}

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				_ = fs.Get("sec", "k"+strconv.Itoa(id)+"_"+strconv.Itoa(i))
				for range fs.GetAll("sec") {
					// 遍历本身就会读 ini 内部结构
				}
			}
		}(r)
	}

	wg.Wait()

	// 收尾断言：批量键应留在最终状态，证明并发下写入没丢
	for w := 0; w < writers; w++ {
		if got := fs.Get("sec", "batch"+strconv.Itoa(w)); got == "" {
			t.Errorf("batch%d 丢失，期望有值", w)
		}
	}
}

// 并发只读不应互斥出错，也不应产生写副作用
func TestFileStorage_ConcurrentGetNoSideEffect(t *testing.T) {
	fs, err := NewFileStorage(filepath.Join(t.TempDir(), "ro.ini"))
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}
	if err := fs.Save("sec", "real", "v"); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	var wg sync.WaitGroup
	for r := 0; r < 16; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = fs.Get("sec", "missing"+strconv.Itoa(id)+"_"+strconv.Itoa(i))
			}
		}(r)
	}
	wg.Wait()

	if got := len(fs.GetAll("sec")); got != 1 {
		t.Errorf("查询不存在的键产生了写副作用：GetAll 返回 %d 项，want 1", got)
	}
}
