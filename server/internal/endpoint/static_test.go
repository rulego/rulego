package endpoint

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestAcceptsGzip(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"gzip, deflate, br", true},
		{"deflate, gzip;q=1.0", true},
		{"deflate", false},
		{"gzip;q=0", false},
		{"gzip;q=0.0", false},
		{"", false},
		{"br", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.header != "" {
			r.Header.Set("Accept-Encoding", c.header)
		}
		if got := requestAcceptsGzip(r); got != c.want {
			t.Errorf("requestAcceptsGzip(%q) = %v, want %v", c.header, got, c.want)
		}
	}
}

func TestSetStaticCacheControl(t *testing.T) {
	cases := []struct {
		urlPath string
		want    string
	}{
		{"/editor/assets/index-abc123.js", "public, max-age=31536000, immutable"},
		{"/editor/assets/vendor-vue-xyz.js", "public, max-age=31536000, immutable"},
		{"/images/endpoint/endpoints.svg", "public, max-age=86400"},
		{"/editor/favicon.ico", "public, max-age=86400"},
		{"/editor/", "no-cache"},
		{"/editor/config/config.js", "no-cache"},
		{"/editor/index.html", "no-cache"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		setStaticCacheControl(w, c.urlPath)
		if got := w.Header().Get("Cache-Control"); got != c.want {
			t.Errorf("setStaticCacheControl(%q) = %q, want %q", c.urlPath, got, c.want)
		}
	}
}

func TestServeStaticFile_GzipAndSafety(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("index.html", "<html>editor</html>")
	bigJS := "console.log(1);" + strings.Repeat("// padding to exceed gzip threshold ", 40)
	writeFile("assets/app.js", bigJS)

	// gzip 生效：JS 文本 + Accept-Encoding: gzip → Content-Encoding: gzip 且可解压回原文
	r := httptest.NewRequest(http.MethodGet, "/editor/assets/app.js", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	serveStaticFile(w, r, dir, "/assets/app.js")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := w.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	body, err := gunzip(w.Body.Bytes())
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if string(body) != bigJS {
		t.Errorf("decompressed body mismatch")
	}

	// 不带 Accept-Encoding：原样返回
	r2 := httptest.NewRequest(http.MethodGet, "/editor/assets/app.js", nil)
	w2 := httptest.NewRecorder()
	serveStaticFile(w2, r2, dir, "/assets/app.js")
	if w2.Header().Get("Content-Encoding") != "" {
		t.Error("no Accept-Encoding should not compress")
	}
	if w2.Body.String() != bigJS {
		t.Errorf("raw body mismatch")
	}

	// 目录请求落 index.html，且 no-cache
	r3 := httptest.NewRequest(http.MethodGet, "/editor/", nil)
	w3 := httptest.NewRecorder()
	serveStaticFile(w3, r3, dir, "/")
	if w3.Code != http.StatusOK || w3.Body.String() != "<html>editor</html>" {
		t.Fatalf("dir request: code=%d body=%q", w3.Code, w3.Body.String())
	}
	if got := w3.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("index.html Cache-Control = %q, want no-cache", got)
	}

	// 路径穿越被拒
	r4 := httptest.NewRequest(http.MethodGet, "/editor/../../etc/passwd", nil)
	w4 := httptest.NewRecorder()
	serveStaticFile(w4, r4, dir, "/../../etc/passwd")
	if w4.Code != http.StatusNotFound {
		t.Errorf("traversal code = %d, want 404", w4.Code)
	}
}

func gunzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
