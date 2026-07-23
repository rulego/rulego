package str

// UnsafeStringFromBytes Zero copy conversion[] byte to string
// Automatically selects optimal implementation using build tags, supporting all Go 1.18+ versions
//
// WARNING: The returned string shares memory with the underlying []byte
// Do not modify the original data while using the string
func UnsafeStringFromBytes(b []byte) string {
	return unsafeStringFromBytes_impl(b)
}

// UnsafeBytesFromString Zero-copy converts string to []byte
// Automatically selects optimal implementation using build tags, supporting all Go 1.18+ versions
//
// WARNING: The returned []byte shares memory with the underlying string
// Do not modify the returned []byte
func UnsafeBytesFromString(s string) []byte {
	return unsafeBytesFromString_impl(s)
}

// SafeStringFromBytes Safe Conversion[] Byte to string (with memory copy)
func SafeStringFromBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return string(b)
}

// SafeBytesFromString Safely converts string to []byte (with memory copy)
func SafeBytesFromString(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return []byte(s)
}

// GetConverterInfo returns information about the converter currently in use
func GetConverterInfo() string {
	return implementationInfo
}
