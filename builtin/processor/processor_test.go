/*
 * Copyright 2024 The RuleGo Authors.
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

package processor

import (
	"errors"
	"net/textproto"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/test/assert"
)

// fakeMessage implements endpoint.Message for driving processors directly.
type fakeMessage struct {
	body    []byte
	headers textproto.MIMEHeader
	from    string
	params  map[string]string
	msg     *types.RuleMsg
	status  int
	err     error
}

func newFakeMessage(from string, body []byte) *fakeMessage {
	return &fakeMessage{
		body:    body,
		headers: textproto.MIMEHeader{},
		from:    from,
		params:  map[string]string{},
	}
}

func (m *fakeMessage) Body() []byte                  { return m.body }
func (m *fakeMessage) Headers() textproto.MIMEHeader { return m.headers }
func (m *fakeMessage) From() string                  { return m.from }
func (m *fakeMessage) GetParam(key string) string    { return m.params[key] }
func (m *fakeMessage) SetMsg(msg *types.RuleMsg)     { m.msg = msg }
func (m *fakeMessage) GetMsg() *types.RuleMsg        { return m.msg }
func (m *fakeMessage) SetStatusCode(code int)        { m.status = code }
func (m *fakeMessage) SetBody(body []byte)           { m.body = body }
func (m *fakeMessage) SetError(err error)            { m.err = err }
func (m *fakeMessage) GetError() error               { return m.err }

// modifierMessage additionally implements endpoint.HeaderModifier.
type modifierMessage struct {
	fakeMessage
	setHeaders map[string]string
}

func (m *modifierMessage) AddHeader(key, value string) {
	if m.setHeaders == nil {
		m.setHeaders = map[string]string{}
	}
	m.setHeaders[key] = value
}

func (m *modifierMessage) SetHeader(key, value string) {
	if m.setHeaders == nil {
		m.setHeaders = map[string]string{}
	}
	m.setHeaders[key] = value
}

func (m *modifierMessage) DelHeader(key string) {
	delete(m.setHeaders, key)
}

func (m *modifierMessage) GetMetadata() *types.Metadata {
	if m.msg != nil {
		return m.msg.Metadata
	}
	return nil
}

func runProcess(t *testing.T, name string, in, out endpoint.Message) bool {
	p, ok := InBuiltins.Get(name)
	if !ok {
		p, ok = OutBuiltins.Get(name)
	}
	assert.True(t, ok, "processor "+name+" should be registered")
	return p(nil, &endpoint.Exchange{In: in, Out: out})
}

func TestInBuiltinsSetDataType(t *testing.T) {
	cases := []struct {
		name        string
		dataType    types.DataType
		contentType string
	}{
		{"setJsonDataType", types.JSON, "application/json"},
		{"setTextDataType", types.TEXT, "text/plain"},
		{"setBinaryDataType", types.BINARY, "application/octet-stream"},
	}
	for _, c := range cases {
		in := newFakeMessage("/api/test", []byte("data"))
		in.msg = &types.RuleMsg{}
		out := newFakeMessage("", nil)
		assert.True(t, runProcess(t, c.name, in, out))
		assert.Equal(t, c.dataType, in.msg.DataType)
		assert.Equal(t, c.contentType, out.headers.Get(HeaderKeyContentType))
	}
}

func TestInBuiltinsHeadersToMetadata(t *testing.T) {
	in := newFakeMessage("/api/test", []byte("data"))
	in.headers.Set("X-Token", "abc")
	in.headers.Set("User-Agent", "test")
	meta := types.NewMetadata()
	meta.PutValue("existing", "kept")
	msg := types.NewMsg(0, "TEST", types.JSON, meta, "data")
	in.msg = &msg
	out := newFakeMessage("", nil)

	assert.True(t, runProcess(t, "headersToMetadata", in, out))
	assert.Equal(t, "abc", in.msg.Metadata.GetValue("X-Token"))
	assert.Equal(t, "test", in.msg.Metadata.GetValue("User-Agent"))
	assert.Equal(t, "kept", in.msg.Metadata.GetValue("existing"))
}

func TestInBuiltinsToHex(t *testing.T) {
	in := newFakeMessage("/device/01", []byte{0x01, 0xab, 0xff})
	in.msg = &types.RuleMsg{}
	out := newFakeMessage("", nil)

	assert.True(t, runProcess(t, "toHex", in, out))
	msg := in.msg
	assert.NotNil(t, msg)
	assert.Equal(t, types.TEXT, msg.DataType)
	assert.Equal(t, "01ABFF", msg.GetData())
	assert.Equal(t, "/device/01", msg.Metadata.GetValue(KeyTopic))
}

func TestOutBuiltinsResponseToBody(t *testing.T) {
	// error case: status code 400 and error text as body
	in := newFakeMessage("", nil)
	out := newFakeMessage("", []byte("old"))
	out.err = errors.New("bad request")
	assert.True(t, runProcess(t, "responseToBody", in, out))
	assert.Equal(t, 400, out.status)
	assert.Equal(t, "bad request", string(out.body))

	// JSON message without content type: type header set and data becomes body
	out = newFakeMessage("", []byte("old"))
	jsonMsg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"ok":true}`)
	out.msg = &jsonMsg
	assert.True(t, runProcess(t, "responseToBody", in, out))
	assert.Equal(t, HeaderValueApplicationJson, out.headers.Get(HeaderKeyContentType))
	assert.Equal(t, `{"ok":true}`, string(out.body))

	// non-JSON message: content type untouched
	out = newFakeMessage("", []byte("old"))
	out.headers.Set(HeaderKeyContentType, "text/csv")
	textMsg := types.NewMsg(0, "TEST", types.TEXT, types.NewMetadata(), "hello")
	out.msg = &textMsg
	assert.True(t, runProcess(t, "responseToBody", in, out))
	assert.Equal(t, "text/csv", out.headers.Get(HeaderKeyContentType))
	assert.Equal(t, "hello", string(out.body))

	// no error and no message: body untouched
	out = newFakeMessage("", []byte("old"))
	assert.True(t, runProcess(t, "responseToBody", in, out))
	assert.Equal(t, "old", string(out.body))
}

func TestOutBuiltinsMetadataToHeaders(t *testing.T) {
	// error case
	in := newFakeMessage("", nil)
	out := newFakeMessage("", []byte("old"))
	out.err = errors.New("denied")
	assert.True(t, runProcess(t, "metadataToHeaders", in, out))
	assert.Equal(t, 400, out.status)
	assert.Equal(t, "denied", string(out.body))

	// plain message: metadata goes to the generic header map
	meta := types.NewMetadata()
	meta.PutValue("X-Custom", "v1")
	out = newFakeMessage("", nil)
	metaMsg := types.NewMsg(0, "TEST", types.TEXT, meta, "data")
	out.msg = &metaMsg
	assert.True(t, runProcess(t, "metadataToHeaders", in, out))
	assert.Equal(t, "v1", out.headers.Get("X-Custom"))

	// HeaderModifier implementation: metadata goes through SetHeader
	mod := &modifierMessage{}
	modMsg := types.NewMsg(0, "TEST", types.TEXT, meta, "data")
	mod.msg = &modMsg
	exchange := &endpoint.Exchange{In: in, Out: mod}
	p, ok := OutBuiltins.Get("metadataToHeaders")
	assert.True(t, ok)
	assert.True(t, p(nil, exchange))
	assert.Equal(t, "v1", mod.setHeaders["X-Custom"])

	// no message and no error: nothing happens
	out = newFakeMessage("", []byte("old"))
	assert.True(t, runProcess(t, "metadataToHeaders", in, out))
	assert.Equal(t, "old", string(out.body))
}

func TestBuiltinsRegistry(t *testing.T) {
	var b builtins
	assert.Equal(t, 0, len(b.Names()))

	proc := func(router endpoint.Router, exchange *endpoint.Exchange) bool { return true }
	b.Register("p1", proc)
	got, ok := b.Get("p1")
	assert.True(t, ok)
	assert.True(t, got(nil, nil))

	b.RegisterAll(map[string]endpoint.Process{"p2": proc, "p3": proc})
	assert.Equal(t, 3, len(b.Names()))

	b.Register("p1", proc) // replace existing name
	assert.Equal(t, 3, len(b.Names()))

	b.Unregister("p2", "missing")
	_, ok = b.Get("p2")
	assert.False(t, ok)
	_, ok = b.Get("p1")
	assert.True(t, ok)

	inNames := InBuiltins.Names()
	for _, name := range []string{"headersToMetadata", "setJsonDataType", "setTextDataType", "setBinaryDataType", "toHex"} {
		found := false
		for _, n := range inNames {
			if n == name {
				found = true
				break
			}
		}
		assert.True(t, found, name+" should be in InBuiltins")
	}
	outNames := OutBuiltins.Names()
	for _, name := range []string{"responseToBody", "metadataToHeaders"} {
		found := false
		for _, n := range outNames {
			if n == name {
				found = true
				break
			}
		}
		assert.True(t, found, name+" should be in OutBuiltins")
	}
}
