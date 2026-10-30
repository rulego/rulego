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

package external

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// recordingDriver is a minimal database/sql driver that records whether a
// statement was run through Query (rows are read back) or Exec (rows are
// discarded). It lets the opType routing be tested without a database server.
type recordingDriver struct {
	mu    sync.Mutex
	calls []string
}

func (d *recordingDriver) record(call string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, call)
}

func (d *recordingDriver) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = nil
}

func (d *recordingDriver) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

func (d *recordingDriver) Open(string) (driver.Conn, error) { return &recordingConn{d: d}, nil }

type recordingConn struct{ d *recordingDriver }

func (c *recordingConn) Prepare(query string) (driver.Stmt, error) {
	return &recordingStmt{d: c.d}, nil
}
func (c *recordingConn) Close() error              { return nil }
func (c *recordingConn) Begin() (driver.Tx, error) { return nil, errors.New("not supported") }

type recordingStmt struct{ d *recordingDriver }

func (s *recordingStmt) Close() error  { return nil }
func (s *recordingStmt) NumInput() int { return -1 }
func (s *recordingStmt) Exec([]driver.Value) (driver.Result, error) {
	s.d.record("exec")
	return driver.RowsAffected(1), nil
}
func (s *recordingStmt) Query([]driver.Value) (driver.Rows, error) {
	s.d.record("query")
	return &singleRow{}, nil
}

// singleRow returns one row with one column: id=1.
type singleRow struct{ read bool }

func (r *singleRow) Columns() []string { return []string{"id"} }
func (r *singleRow) Close() error      { return nil }
func (r *singleRow) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	dest[0] = int64(1)
	return nil
}

var (
	optypeDriver     = &recordingDriver{}
	optypeDriverOnce sync.Once
)

const optypeDriverName = "rulego_optype_recording"

func registerOptypeDriver() {
	optypeDriverOnce.Do(func() { sql.Register(optypeDriverName, optypeDriver) })
}

// runDbClientNode initializes a dbClient node on the recording driver, sends one
// message and returns the statement calls the driver saw plus the result message.
func runDbClientNode(t *testing.T, config types.Configuration, metadata map[string]string) ([]string, types.RuleMsg, string) {
	t.Helper()
	registerOptypeDriver()
	optypeDriver.reset()

	config["driverName"] = optypeDriverName
	config["dsn"] = "recording"
	node, err := test.CreateAndInitNode("dbClient", config, Registry)
	assert.Nil(t, err)

	var result types.RuleMsg
	var relationType string
	done := make(chan struct{})
	test.NodeOnMsg(t, node, []test.Msg{{
		MetaData: types.BuildMetadata(metadata),
		MsgType:  "TEST_MSG_TYPE",
		Data:     "{}",
	}}, func(msg types.RuleMsg, rel string, err error) {
		assert.Nil(t, err)
		result = msg
		relationType = rel
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the dbClient node to finish")
	}
	return optypeDriver.recorded(), result, relationType
}

// opType AUTO (any case) is documented as "auto-detect from SQL", the same as
// leaving it empty. It used to be passed through as a literal "AUTO" type, which
// matched no case in OnMsg and sent a SELECT down the Exec path, discarding rows.
func TestDbClientNodeOpTypeAutoDetect(t *testing.T) {
	for _, opType := range []string{"", "AUTO", "auto", "Auto", " auto "} {
		t.Run("select/opType="+opType, func(t *testing.T) {
			calls, msg, rel := runDbClientNode(t, types.Configuration{
				"sql":    "select id from users",
				"opType": opType,
			}, nil)
			assert.Equal(t, []string{"query"}, calls)
			assert.Equal(t, types.Success, rel)
			assert.Equal(t, `[{"id":1}]`, msg.GetData())
		})

		t.Run("select via template/opType="+opType, func(t *testing.T) {
			calls, msg, rel := runDbClientNode(t, types.Configuration{
				"sql":    "${metadata.sql}",
				"opType": opType,
			}, map[string]string{"sql": "select id from users"})
			assert.Equal(t, []string{"query"}, calls)
			assert.Equal(t, types.Success, rel)
			assert.Equal(t, `[{"id":1}]`, msg.GetData())
		})

		t.Run("update/opType="+opType, func(t *testing.T) {
			calls, msg, rel := runDbClientNode(t, types.Configuration{
				"sql":    "update users set age = 1",
				"opType": opType,
			}, nil)
			assert.Equal(t, []string{"exec"}, calls)
			assert.Equal(t, types.Success, rel)
			assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
		})
	}
}

// An explicit opType keeps routing by that type, whatever the SQL text says.
func TestDbClientNodeOpTypeExplicit(t *testing.T) {
	calls, _, rel := runDbClientNode(t, types.Configuration{
		"sql":    "select id from users",
		"opType": "select",
	}, nil)
	assert.Equal(t, []string{"query"}, calls)
	assert.Equal(t, types.Success, rel)

	calls, _, rel = runDbClientNode(t, types.Configuration{
		"sql":    "update users set age = 1",
		"opType": "UPDATE",
	}, nil)
	assert.Equal(t, []string{"exec"}, calls)
	assert.Equal(t, types.Success, rel)
}
