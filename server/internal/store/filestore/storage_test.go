package filestore

import (
	"testing"
)

func TestFileStorage(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStorage(dir + "/test.ini")
	if err != nil {
		t.Fatal(err)
	}

	// Save and Get
	if err := fs.Save("section1", "key1", "value1"); err != nil {
		t.Fatal(err)
	}
	if v := fs.Get("section1", "key1"); v != "value1" {
		t.Errorf("Get = %q, want value1", v)
	}

	// GetAll
	if err := fs.Save("section1", "key2", "value2"); err != nil {
		t.Fatal(err)
	}
	all := fs.GetAll("section1")
	if len(all) != 2 {
		t.Errorf("GetAll count = %d, want 2", len(all))
	}

	// Delete
	if err := fs.Delete("section1", "key1"); err != nil {
		t.Fatal(err)
	}
	if v := fs.Get("section1", "key1"); v != "" {
		t.Errorf("Get after delete = %q, want empty", v)
	}

	// SaveList
	values := map[string]string{"a": "1", "b": "2"}
	if err := fs.SaveList("section2", values); err != nil {
		t.Fatal(err)
	}
	if v := fs.Get("section2", "a"); v != "1" {
		t.Errorf("Get a = %q, want 1", v)
	}
}

func TestFileStorageManager(t *testing.T) {
	dir := t.TempDir()
	mgr := NewFileStorageManager()

	fs1, err := mgr.Init(dir + "/test1.ini")
	if err != nil {
		t.Fatal(err)
	}

	// Get should return cached instance
	fs2, err := mgr.Get(dir + "/test1.ini")
	if err != nil {
		t.Fatal(err)
	}
	if fs1 != fs2 {
		t.Error("Get should return same instance")
	}

	// Delete
	mgr.Delete(dir + "/test1.ini")
	fs3, err := mgr.Get(dir + "/test1.ini")
	if err != nil {
		t.Fatal(err)
	}
	if fs1 == fs3 {
		t.Error("Get after Delete should return new instance")
	}
}
