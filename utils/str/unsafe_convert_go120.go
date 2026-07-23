//go:build go1.20

package str

import "unsafe"

// Go version 1.20+ implementation, using the official unsafe function
func unsafeStringFromBytes_impl(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Use the official unsafe function from Go 1.20+
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func unsafeBytesFromString_impl(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	// Use the official unsafe function from Go 1.20+
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// Implementation information
const implementationInfo = "Go 1.20+ official unsafe"
