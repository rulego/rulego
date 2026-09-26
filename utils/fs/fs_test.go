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
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/rulego/rulego/test/assert"
)

func TestSaveAndLoadFile(t *testing.T) {
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "testfile.txt")
	testData := []byte("hello world")

	// Test SaveFile
	err := SaveFile(testFilePath, testData)
	assert.Nil(t, err)

	// Test LoadFile
	loadedData := LoadFile(testFilePath)
	assert.Equal(t, testData, loadedData)

	// Test LoadFile with non-existent file
	loadedNonExistent := LoadFile(filepath.Join(tempDir, "nonexistent.txt"))
	assert.Nil(t, loadedNonExistent)

	// Test SaveFile to a path that requires directory creation (though SaveFile itself doesn't create dirs)
	// This test is more about os.Create behavior, but good to be aware.
	// For SaveFile to work, the directory must exist.
	deepPath := filepath.Join(tempDir, "subdir", "testfile2.txt")
	err = os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	assert.Nil(t, err)
	err = SaveFile(deepPath, testData)
	assert.Nil(t, err)
	loadedDeepData := LoadFile(deepPath)
	assert.Equal(t, testData, loadedDeepData)
}

func TestIsExist(t *testing.T) {
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "exists.txt")

	// Test with non-existent file
	assert.False(t, IsExist(testFilePath))

	// Create a file
	file, err := os.Create(testFilePath)
	assert.Nil(t, err)
	file.Close()

	// Test with existing file
	assert.True(t, IsExist(testFilePath))

	// Test with existing directory
	assert.True(t, IsExist(tempDir))

	// Test with non-existent directory
	assert.False(t, IsExist(filepath.Join(tempDir, "nonexistentdir")))
}

// writeFileTree creates a fixed file tree under dir for pattern tests.
func writeFileTree(t *testing.T, dir string) {
	t.Helper()
	files := []string{
		"a.json",
		"b.json",
		"c.txt",
		"skip.log",
		filepath.Join("sub", "d.json"),
		filepath.Join("excluded", "e.json"),
	}
	for _, f := range files {
		p := filepath.Join(dir, f)
		assert.Nil(t, os.MkdirAll(filepath.Dir(p), 0755))
		assert.Nil(t, os.WriteFile(p, []byte(f), 0644))
	}
}

func TestLocalFileStorage(t *testing.T) {
	storage := NewLocalFileStorage()
	assert.Equal(t, "default", storage.Name())

	tempDir := t.TempDir()

	t.Run("SaveAutoCreateDirsAndGet", func(t *testing.T) {
		p := filepath.Join(tempDir, "x", "y", "f.txt")
		assert.Nil(t, storage.Save(p, []byte("data")))
		got, err := storage.Get(p)
		assert.Nil(t, err)
		assert.Equal(t, "data", string(got))
	})

	t.Run("SaveUnderFilePathFails", func(t *testing.T) {
		blocker := filepath.Join(tempDir, "blocker")
		assert.Nil(t, os.WriteFile(blocker, []byte("f"), 0644))
		assert.NotNil(t, storage.Save(filepath.Join(blocker, "nested", "f.txt"), []byte("data")))
		assert.NotNil(t, storage.SaveAppend(filepath.Join(blocker, "nested", "f.txt"), []byte("data")))
	})

	t.Run("SaveAppend", func(t *testing.T) {
		p := filepath.Join(tempDir, "append", "log.txt")
		assert.Nil(t, storage.SaveAppend(p, []byte("a")))
		assert.Nil(t, storage.SaveAppend(p, []byte("b")))
		got, err := storage.Get(p)
		assert.Nil(t, err)
		assert.Equal(t, "ab", string(got))
	})

	t.Run("Delete", func(t *testing.T) {
		p := filepath.Join(tempDir, "del.txt")
		assert.Nil(t, storage.Save(p, []byte("x")))
		assert.Nil(t, storage.Delete(p))
		assert.False(t, storage.IsExist(p))
		// os.Remove on a missing path returns an error
		assert.NotNil(t, storage.Delete(p))
	})

	t.Run("CreateDirs", func(t *testing.T) {
		p := filepath.Join(tempDir, "d1", "d2")
		assert.Nil(t, storage.CreateDirs(p))
		assert.True(t, storage.IsExist(p))
		// create on an existing dir is a no-op
		assert.Nil(t, storage.CreateDirs(p))

		blocker := filepath.Join(tempDir, "blocker2")
		assert.Nil(t, os.WriteFile(blocker, []byte("f"), 0644))
		assert.NotNil(t, storage.CreateDirs(filepath.Join(blocker, "nested")))
	})
}

func TestLocalFileStorageGetFilePaths(t *testing.T) {
	tempDir := t.TempDir()
	writeFileTree(t, tempDir)
	allPattern := filepath.Join(tempDir, "*.json")

	t.Run("MatchByFileName", func(t *testing.T) {
		// file name part matches at every depth, so sub/d.json and excluded/e.json hit too
		paths, err := NewLocalFileStorage().GetFilePaths(allPattern)
		assert.Nil(t, err)
		sort.Strings(paths)
		assert.Equal(t, 4, len(paths))
		assert.Equal(t, filepath.Join(tempDir, "a.json"), paths[0])
		assert.Equal(t, filepath.Join(tempDir, "b.json"), paths[1])
	})

	t.Run("ExcludeFilesByPattern", func(t *testing.T) {
		txtPattern := filepath.Join(tempDir, "*.txt")
		paths, err := NewLocalFileStorage().GetFilePaths(txtPattern, "c.*")
		assert.Nil(t, err)
		assert.Equal(t, 0, len(paths))
	})

	t.Run("ExcludeDirSkipsSubtree", func(t *testing.T) {
		// recursive walk needs a directory-only pattern via the wildcard trick:
		// walk starts at the dir part, file part "*" matches everything
		paths, err := NewLocalFileStorage().GetFilePaths(allPattern, "excluded")
		assert.Nil(t, err)
		assert.Equal(t, 3, len(paths))
		for _, p := range paths {
			assert.False(t, filepath.HasPrefix(p, filepath.Join(tempDir, "excluded")))
		}
	})

	t.Run("NonExistentRootReturnsError", func(t *testing.T) {
		_, err := NewLocalFileStorage().GetFilePaths(filepath.Join(tempDir, "missing", "*.json"))
		assert.NotNil(t, err)
	})
}

func TestPackageLevelDelegates(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("GetFilePaths", func(t *testing.T) {
		writeFileTree(t, tempDir)
		paths, err := GetFilePaths(filepath.Join(tempDir, "*.json"))
		assert.Nil(t, err)
		assert.Equal(t, 4, len(paths))
	})

	t.Run("CreateDirs", func(t *testing.T) {
		p := filepath.Join(tempDir, "pkg", "dir")
		assert.Nil(t, CreateDirs(p))
		assert.True(t, IsExist(p))
	})
}

// memStorage is an in-memory File implementation used to verify SetStorage swaps the delegate.
type memStorage struct {
	data map[string][]byte
	// failOn makes every operation targeting path return an error
	failOn string
}

func newMemStorage() *memStorage {
	return &memStorage{data: map[string][]byte{}}
}

func (m *memStorage) Save(path string, data []byte) error {
	if path == m.failOn {
		return os.ErrInvalid
	}
	m.data[path] = data
	return nil
}

func (m *memStorage) Get(path string) ([]byte, error) {
	if d, ok := m.data[path]; ok {
		return d, nil
	}
	return nil, os.ErrNotExist
}

func (m *memStorage) Delete(path string) error {
	delete(m.data, path)
	return nil
}

func (m *memStorage) SaveAppend(path string, data []byte) error {
	m.data[path] = append(m.data[path], data...)
	return nil
}

func (m *memStorage) GetFilePaths(string, ...string) ([]string, error) {
	return []string{"mem://a"}, nil
}

func (m *memStorage) IsExist(path string) bool {
	_, ok := m.data[path]
	return ok
}

func (m *memStorage) CreateDirs(string) error { return nil }

func (m *memStorage) Name() string { return "mem" }

func TestSetGetStorage(t *testing.T) {
	orig := GetStorage()
	defer SetStorage(orig)

	mem := newMemStorage()
	SetStorage(mem)
	assert.Equal(t, mem, GetStorage())

	assert.Nil(t, SaveFile("k", []byte("v")))
	assert.Equal(t, []byte("v"), LoadFile("k"))

	paths, err := GetFilePaths("any")
	assert.Nil(t, err)
	assert.Equal(t, []string{"mem://a"}, paths)

	assert.Nil(t, CreateDirs("d"))
	assert.True(t, IsExist("k"))

	// failure paths surface through the package-level helpers
	mem.failOn = "bad"
	assert.NotNil(t, SaveFile("bad", []byte("v")))
}
