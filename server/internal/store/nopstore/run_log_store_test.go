package nopstore

import (
	"errors"
	"testing"
	"time"

	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

func TestNopRunLogStore_Save(t *testing.T) {
	s := NopRunLogStore{}
	if err := s.Save("user", model.Event{Id: "test"}); err != nil {
		t.Fatalf("Save should return nil, got: %v", err)
	}
}

func TestNopRunLogStore_List(t *testing.T) {
	s := NopRunLogStore{}
	events, total, err := s.List("user", "chain", time.Time{}, time.Time{}, 10, 1)
	if err != nil {
		t.Fatalf("List should return nil error, got: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if events != nil {
		t.Errorf("events should be nil")
	}
}

func TestNopRunLogStore_Get(t *testing.T) {
	s := NopRunLogStore{}
	event, err := s.Get("user", "id")
	if !errors.Is(err, store.ErrRunLogNotFound) {
		t.Fatalf("Get should return ErrRunLogNotFound, got: %v", err)
	}
	if event.Id != "" {
		t.Errorf("event.Id should be empty")
	}
}

func TestNopRunLogStore_Delete(t *testing.T) {
	s := NopRunLogStore{}
	if err := s.Delete("user", "id"); err != nil {
		t.Fatalf("Delete should return nil, got: %v", err)
	}
}

func TestNopRunLogStore_DeleteByChainId(t *testing.T) {
	s := NopRunLogStore{}
	if err := s.DeleteByChainId("user", "chain"); err != nil {
		t.Fatalf("DeleteByChainId should return nil, got: %v", err)
	}
}
