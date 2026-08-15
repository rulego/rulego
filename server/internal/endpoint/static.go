package endpoint

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
)

// 静态资源服务：gzip 压缩 + Cache-Control + HEAD 支持。
// 压缩为在线 gzip + 进程内缓存（key=绝对路径，mtime 失效），上限 128MB，超限清空。
// 前置 nginx 已开启压缩时可配置 disable_gzip=true 关掉本层。

// gzipEnabled 响应压缩开关（静态资源 + JSON API 共用），启动时按配置设置
var gzipEnabled = true

// SetGzipEnabled 设置响应压缩开关
func SetGzipEnabled(enabled bool) {
	gzipEnabled = enabled
}

// registerStaticFiles 注册静态资源路由，mapping 格式：/url/=./dir,...（逗号分隔多组），
// url 兼容 /url/*filepath=./dir 写法
func (s *Server) registerStaticFiles(ep endpointApi.HttpEndpoint, resourceMapping string) {
	for _, item := range strings.Split(resourceMapping, ",") {
		files := strings.Split(item, "=")
		if len(files) != 2 {
			continue
		}
		urlPath := strings.TrimSpace(files[0])
		localDir := strings.TrimSpace(files[1])
		urlPath = strings.TrimSuffix(urlPath, "/*filepath")
		urlBase := strings.TrimRight(urlPath, "/")
		// 根路径 catch-all 与已注册的 / 重定向路由冲突（httprouter panic）
		if urlBase == "" {
			continue
		}
		pattern := urlBase + "/*filepath"
		handler := s.staticHandler(localDir)
		ep.GET(endpoint.NewRouter().From(s.basePath() + pattern).Process(handler).End())
		ep.HEAD(endpoint.NewRouter().From(s.basePath() + pattern).Process(handler).End())
	}
}

// staticHandler 直接操作底层 http.ResponseWriter，不走 exchange 的 SetBody
func (s *Server) staticHandler(localDir string) func(endpointApi.Router, *endpointApi.Exchange) bool {
	root := filepath.Clean(localDir)
	return func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		in, ok1 := exchange.In.(*rest.RequestMessage)
		out, ok2 := exchange.Out.(*rest.ResponseMessage)
		if !ok1 || !ok2 {
			return false
		}
		serveStaticFile(out.Response(), in.Request(), root, in.Params.ByName("filepath"))
		return false
	}
}

// serveStaticFile 核心下发逻辑：安全解析路径 → 目录落 index.html → 缓存头 → 按需 gzip。
func serveStaticFile(w http.ResponseWriter, r *http.Request, rootDir, urlFilepath string) {
	if urlFilepath == "" {
		urlFilepath = "/"
	}
	// Clean("/"+p) 保证以 / 开头且不含 ..，映射回本地路径后再确认仍在根目录内
	rel := path.Clean("/" + urlFilepath)
	full := filepath.Join(rootDir, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
	if full != rootDir && !strings.HasPrefix(full, rootDir+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		full = filepath.Join(full, "index.html")
		info, err = os.Stat(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// 全局拦截器给所有响应 Add 了 application/json，需清掉让 ServeContent 按扩展名判定
	w.Header().Del("Content-Type")
	setStaticCacheControl(w, r.URL.Path)

	name := filepath.Base(full)
	// Range 语义基于原始字节，与压缩表示不兼容，带 Range 的请求走原文件
	if gzipEnabled && info.Size() >= 1024 && r.Header.Get("Range") == "" &&
		isCompressibleExt(name) && requestAcceptsGzip(r) {
		if data, modTime, ok := staticGzipCache.get(full); ok {
			serveGzipContent(w, r, name, modTime, data)
			return
		}
		raw, err := os.ReadFile(full)
		if err == nil {
			if gz := gzipBytes(raw); len(gz) < len(raw) {
				staticGzipCache.put(full, info.ModTime(), gz)
				serveGzipContent(w, r, name, info.ModTime(), gz)
				return
			}
		}
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func serveGzipContent(w http.ResponseWriter, r *http.Request, name string, modTime time.Time, data []byte) {
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	http.ServeContent(w, r, name, modTime, bytes.NewReader(data))
}

// setStaticCacheControl 缓存策略：
//   - /assets/：内容哈希产物，永久缓存（immutable），改版换文件名
//   - 图片/字体：1 天
//   - 其余（index.html、config.js 等运行时可变文件）：no-cache，走条件请求
func setStaticCacheControl(w http.ResponseWriter, urlPath string) {
	p := strings.ToLower(urlPath)
	if strings.Contains(p, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	if strings.Contains(p, "/images/") || isStaticAssetExt(path.Ext(p)) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

// isCompressibleExt 文本类扩展名；图片/字体已是压缩格式
func isCompressibleExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".js", ".mjs", ".css", ".json", ".html", ".htm", ".svg", ".map", ".txt", ".xml", ".webmanifest":
		return true
	}
	return false
}

func isStaticAssetExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot":
		return true
	}
	return false
}

// requestAcceptsGzip 解析 Accept-Encoding，gzip;q=0 视为不接受
func requestAcceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		token := strings.TrimSpace(part)
		if token == "gzip" || strings.HasPrefix(token, "gzip;") {
			return !strings.Contains(token, "q=0")
		}
	}
	return false
}

// gzipBytes 以默认级别压缩
func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

// ===== gzip 结果进程内缓存 =====

type gzipCacheEntry struct {
	modTime time.Time
	data    []byte
}

type gzipFileCache struct {
	mu       sync.Mutex
	entries  map[string]gzipCacheEntry
	total    int
	maxBytes int
}

var staticGzipCache = &gzipFileCache{entries: make(map[string]gzipCacheEntry), maxBytes: 128 << 20}

func (c *gzipFileCache) get(full string) ([]byte, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[full]
	if !ok {
		return nil, time.Time{}, false
	}
	info, err := os.Stat(full)
	if err != nil || !info.ModTime().Equal(e.modTime) {
		c.total -= len(e.data)
		delete(c.entries, full)
		return nil, time.Time{}, false
	}
	return e.data, e.modTime, true
}

func (c *gzipFileCache) put(full string, modTime time.Time, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries[full]; ok {
		c.total -= len(old.data)
	}
	if c.total+len(data) > c.maxBytes {
		c.entries = make(map[string]gzipCacheEntry)
		c.total = 0
	}
	c.entries[full] = gzipCacheEntry{modTime: modTime, data: data}
	c.total += len(data)
}
