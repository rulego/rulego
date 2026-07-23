//go:build !go1.20

package str

import (
	"reflect"
	"runtime"
	"unsafe"
)

// Package-level variables, which determine the conversion strategy at init
var (
	useUnsafeConversion bool                // Whether to use unsafe conversions
	conversionStrategy  string              // Description of the conversion strategy
	stringFromBytesFunc func([]byte) string // String conversion function pointer
	bytesFromStringFunc func(string) []byte // Byte conversion function pointer
)

// init determines the optimal conversion strategy during packet initialization
func init() {
	// Check if the platform is suitable for using Unsafe conversion
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

// Implemented in Go versions 1.18-1.19, using the strategy determined at initialization
func unsafeStringFromBytes_impl(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// Use the transformation function determined at initialization
	return stringFromBytesFunc(b)
}

func unsafeBytesFromString_impl(s string) []byte {
	if len(s) == 0 {
		return nil
	}

	// Use the transformation function determined at initialization
	return bytesFromStringFunc(s)
}

// === Specific conversion implementation function ===

// Unsafe transformation implementation
func unsafeStringFromBytesImpl(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func unsafeBytesFromStringImpl(s string) []byte {
	// Safely construct slice headers using the reflect package
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

// Secure conversion implementation
func safeStringFromBytesImpl(b []byte) string {
	return string(b)
}

func safeBytesFromStringImpl(s string) []byte {
	return []byte(s)
}

// === Platform detection function (called only once when init) ===

// Check if the platform is secure (suitable for using Unsafe conversion)
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

// Implementation information (dynamically returns the actual policy used)
func getImplementationInfo() string {
	return conversionStrategy
}

const implementationInfo = "Classic unsafe (Go 1.18+)"
