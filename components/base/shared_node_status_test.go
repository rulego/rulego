/*
 * Copyright 2026 The RuleGo Authors.
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

package base

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
)

// TestSharedNodeConnectionStatus verifies the connection state machine:
// a successful sync init -> Connected; a failed init -> Reconnecting (with message); SetStatus supports external updates.
func TestSharedNodeConnectionStatus(t *testing.T) {
	t.Run("init_now_success_sets_connected", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "client", nil
		}, nil)
		if info := x.ConnectionStatus(); info.Status != types.StatusConnected {
			t.Fatalf("after successful init: status=%s, want connected", info.Status)
		}
	})

	t.Run("init_now_failure_strict_returns_error", func(t *testing.T) {
		// InitWithClose is strict: an initNow failure returns the error as-is (NodeClientInitNow gate).
		x := &SharedNode[string]{InitFailRetryInterval: 100 * time.Millisecond}
		err := x.InitWithClose(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "", errors.New("dial: connection refused")
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "refused") {
			t.Fatalf("strict init: err=%v, want contains 'refused'", err)
		}
		// Status is still set to Reconnecting for diagnostics; the caller decides via the returned error.
		if info := x.ConnectionStatus(); info.Status != types.StatusReconnecting {
			t.Fatalf("after failed strict init: status=%s, want reconnecting", info.Status)
		}
	})

	t.Run("init_now_failure_softfail_sets_reconnecting", func(t *testing.T) {
		// InitWithCloseSoftFail swallows the error and sets Reconnecting, for tolerant endpoint startup.
		x := &SharedNode[string]{InitFailRetryInterval: 100 * time.Millisecond}
		err := x.InitWithCloseSoftFail(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "", errors.New("dial: connection refused")
		}, nil)
		if err != nil {
			t.Fatalf("soft-fail init: err=%v, want nil", err)
		}
		info := x.ConnectionStatus()
		if info.Status != types.StatusReconnecting {
			t.Fatalf("after failed soft-fail init: status=%s, want reconnecting", info.Status)
		}
		if !strings.Contains(info.Message, "refused") {
			t.Fatalf("status message=%q, want contains 'refused'", info.Message)
		}
	})

	t.Run("set_status_then_query", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "client", nil
		}, nil)
		x.SetStatus(types.StatusDisconnected, "graceful stop")
		if info := x.ConnectionStatus(); info.Status != types.StatusDisconnected {
			t.Fatalf("after SetStatus: status=%s, want disconnected", info.Status)
		}
		if info := x.ConnectionStatus(); info.Message != "graceful stop" {
			t.Fatalf("status message=%q, want 'graceful stop'", info.Message)
		}
	})
}
