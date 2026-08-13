package endpoint

import (
	"errors"
	"testing"
)

func TestGenerateApiKey(t *testing.T) {
	key, err := generateApiKey()
	if err != nil {
		t.Fatalf("generateApiKey() error = %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key 长度 = %d, want 32", len(key))
	}
}

// rand 失败必须返回 error 而非空串：静默落盘空 ApiKey 会让用户以为重置成功，
// 实则凭据丢失。
func TestGenerateApiKey_RandFailure(t *testing.T) {
	orig := randRead
	randRead = func(b []byte) (int, error) { return 0, errors.New("entropy source broken") }
	defer func() { randRead = orig }()

	key, err := generateApiKey()
	if err == nil {
		t.Error("rand 失败时应返回 error")
	}
	if key != "" {
		t.Errorf("rand 失败时 key 应为空, got %q", key)
	}
}
