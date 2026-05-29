package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"net/textproto"
)

// testMessage 测试用 Message 实现
type testMessage struct {
	msg        *types.RuleMsg
	body       []byte
	statusCode int
	headers    textproto.MIMEHeader
	err        error
}

func newTestOutMessage() *testMessage {
	return &testMessage{headers: make(textproto.MIMEHeader)}
}

func (m *testMessage) Body() []byte                  { return m.body }
func (m *testMessage) Headers() textproto.MIMEHeader { return m.headers }
func (m *testMessage) From() string                  { return "test" }
func (m *testMessage) GetParam(key string) string    { return "" }
func (m *testMessage) SetMsg(msg *types.RuleMsg)     { m.msg = msg }
func (m *testMessage) GetMsg() *types.RuleMsg        { return m.msg }
func (m *testMessage) SetStatusCode(code int)        { m.statusCode = code }
func (m *testMessage) SetBody(body []byte)           { m.body = body }
func (m *testMessage) SetError(err error)            { m.err = err }
func (m *testMessage) GetError() error               { return m.err }

func newTestExchange(t *testing.T) *endpointApi.Exchange {
	t.Helper()
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")
	in := &testMessage{headers: make(textproto.MIMEHeader), msg: &msg}
	return &endpointApi.Exchange{
		In:  in,
		Out: newTestOutMessage(),
	}
}

func outStatus(exchange *endpointApi.Exchange) int {
	return exchange.Out.(*testMessage).statusCode
}

func outBody(exchange *endpointApi.Exchange) []byte {
	return exchange.Out.Body()
}

func TestWriteError(t *testing.T) {
	exchange := newTestExchange(t)
	writeError(exchange, http.StatusBadRequest, errors.New("test error"))

	if outStatus(exchange) != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", outStatus(exchange), http.StatusBadRequest)
	}
	var resp map[string]string
	if err := json.Unmarshal(outBody(exchange), &resp); err != nil {
		t.Fatalf("response body should be JSON: %v", err)
	}
	if resp["error"] != "test error" {
		t.Errorf("error = %q, want %q", resp["error"], "test error")
	}
}

func TestWriteBadRequest(t *testing.T) {
	exchange := newTestExchange(t)
	writeBadRequest(exchange, errors.New("bad input"))
	if outStatus(exchange) != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", outStatus(exchange), http.StatusBadRequest)
	}
}

func TestWriteInternalError(t *testing.T) {
	exchange := newTestExchange(t)
	writeInternalError(exchange, errors.New("secret db error"))

	if outStatus(exchange) != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", outStatus(exchange), http.StatusInternalServerError)
	}
	var resp map[string]string
	json.Unmarshal(outBody(exchange), &resp)
	if resp["error"] != "internal server error" {
		t.Errorf("error = %q, should not expose internal details", resp["error"])
	}
}

func TestWriteJSON(t *testing.T) {
	exchange := newTestExchange(t)
	writeJSON(exchange, map[string]string{"status": "ok"})

	var resp map[string]string
	if err := json.Unmarshal(outBody(exchange), &resp); err != nil {
		t.Fatalf("response should be valid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestWriteJSON_Unserializable(t *testing.T) {
	exchange := newTestExchange(t)
	writeJSON(exchange, make(chan int))
	if outStatus(exchange) != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d for unserializable value", outStatus(exchange), http.StatusInternalServerError)
	}
}

func TestWriteNoContent(t *testing.T) {
	exchange := newTestExchange(t)
	writeNoContent(exchange)
	if outStatus(exchange) != 204 {
		t.Errorf("status = %d, want 204", outStatus(exchange))
	}
}

func TestWriteListResult(t *testing.T) {
	exchange := newTestExchange(t)
	items := []map[string]string{{"name": "item1"}, {"name": "item2"}}
	writeListResult(exchange, items, 10, 1, 20)

	var resp map[string]interface{}
	if err := json.Unmarshal(outBody(exchange), &resp); err != nil {
		t.Fatalf("response should be valid JSON: %v", err)
	}
	if resp["total"].(float64) != 10 {
		t.Errorf("total = %v, want 10", resp["total"])
	}
	if resp["page"].(float64) != 1 {
		t.Errorf("page = %v, want 1", resp["page"])
	}
	arr := resp["items"].([]interface{})
	if len(arr) != 2 {
		t.Errorf("items count = %d, want 2", len(arr))
	}
}

func TestIntParam(t *testing.T) {
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")
	msg.Metadata.PutValue("count", "42")
	msg.Metadata.PutValue("invalid", "abc")

	if v := intParam(&msg, "count", 0); v != 42 {
		t.Errorf("intParam(count) = %d, want 42", v)
	}
	if v := intParam(&msg, "invalid", 99); v != 99 {
		t.Errorf("intParam(invalid) = %d, want 99 (default)", v)
	}
	if v := intParam(&msg, "missing", 7); v != 7 {
		t.Errorf("intParam(missing) = %d, want 7 (default)", v)
	}
}

func TestMetadataUsername(t *testing.T) {
	exchange := newTestExchange(t)
	exchange.In.GetMsg().Metadata.PutValue("username", "testuser")

	if v := metadataUsername(exchange); v != "testuser" {
		t.Errorf("metadataUsername = %q, want %q", v, "testuser")
	}
}

func TestMetadataValue(t *testing.T) {
	exchange := newTestExchange(t)
	exchange.In.GetMsg().Metadata.PutValue("custom-key", "custom-val")

	if v := metadataValue(exchange, "custom-key"); v != "custom-val" {
		t.Errorf("metadataValue = %q, want %q", v, "custom-val")
	}
	if v := metadataValue(exchange, "nonexistent"); v != "" {
		t.Errorf("metadataValue(nonexistent) = %q, want empty", v)
	}
}
