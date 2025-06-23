//go:build !go1.20

package str

import (
	"reflect"
	"runtime"
	"unsafe"
)

// 包级变量，在init时决定转换策略
var (
	useUnsafeConversion bool                // 是否使用unsafe转换
	conversionStrategy  string              // 转换策略描述
	stringFromBytesFunc func([]byte) string // 字符串转换函数指针
	bytesFromStringFunc func(string) []byte // 字节转换函数指针
)

// init在包初始化时决定最优的转换策略
func init() {
	// 检查平台是否适合使用unsafe转换
	if isSafePlatform() {
		useUnsafeConversion = true
		conversionStrategy = "Classic unsafe (Go 1.18+)"
		stringFromBytesFunc = unsafeStringFromBytesImpl
		bytesFromStringFunc = unsafeBytesFromStringImpl
	} else {
		useUnsafeConversion = false
		conversionStrategy = "Safe fallback"
		stringFromBytesFunc = safeStringFromBytesImpl
		bytesFromStringFunc = safeBytesFromStringImpl
	}
}

// Go 1.18-1.19版本的实现，使用初始化时决定的策略
func unsafeStringFromBytes_impl(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// 使用初始化时决定的转换函数
	return stringFromBytesFunc(b)
}

func unsafeBytesFromString_impl(s string) []byte {
	if len(s) == 0 {
		return nil
	}

	// 使用初始化时决定的转换函数
	return bytesFromStringFunc(s)
}

// === 具体的转换实现函数 ===

// unsafe转换实现
func unsafeStringFromBytesImpl(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func unsafeBytesFromStringImpl(s string) []byte {
	// 使用reflect包安全地构造slice header
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

// 安全转换实现
func safeStringFromBytesImpl(b []byte) string {
	return string(b)
}

func safeBytesFromStringImpl(s string) []byte {
	return []byte(s)
}

// === 平台检测函数（仅在init时调用一次）===

// 检查是否为安全的平台（适合使用unsafe转换）
func isSafePlatform() bool {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		switch runtime.GOARCH {
		case "amd64", "arm64":
			return true
		}
	}
	return false
}

// 实现信息（动态返回实际使用的策略）
func getImplementationInfo() string {
	return conversionStrategy
}

const implementationInfo = "Classic unsafe (Go 1.18+)"
