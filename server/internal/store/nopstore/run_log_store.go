package nopstore

import (
	"time"

	"github.com/rulego/rulego/server/model"
)

// NopRunLogStore 空操作的运行日志存储，save_run_log=false 时使用，零开销。
type NopRunLogStore struct{}

func (NopRunLogStore) Save(_ string, _ model.Event) error { return nil }

func (NopRunLogStore) List(_ string, _ string, _, _ time.Time, _, _ int) ([]model.Event, int, error) {
	return nil, 0, nil
}

func (NopRunLogStore) Get(_, _ string) (model.Event, error) {
	return model.Event{}, nil
}

func (NopRunLogStore) Delete(_, _ string) error { return nil }

func (NopRunLogStore) DeleteByChainId(_, _ string) error { return nil }
