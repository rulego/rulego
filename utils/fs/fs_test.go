package fs

import (
	"os"
	"path/filepath"
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
