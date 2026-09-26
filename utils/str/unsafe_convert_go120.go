//go:build go1.20

package str

import "unsafe"

// Go 1.20+版本的实现，使用官方unsafe函数
func unsafeStringFromBytes_impl(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// 使用Go 1.20+的官方unsafe函数
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func unsafeBytesFromString_impl(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	// 使用Go 1.20+的官方unsafe函数
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// 实现信息
const implementationInfo = "Go 1.20+ official unsafe"
