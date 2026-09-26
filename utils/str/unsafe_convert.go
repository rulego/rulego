package str

// UnsafeStringFromBytes 零拷贝转换[]byte到string
// 使用构建标签自动选择最优实现，支持Go 1.18+所有版本
//
// WARNING: 返回的字符串与底层[]byte共享内存
// 在使用字符串期间不要修改原始数据
func UnsafeStringFromBytes(b []byte) string {
	return unsafeStringFromBytes_impl(b)
}

// UnsafeBytesFromString 零拷贝转换string到[]byte
// 使用构建标签自动选择最优实现，支持Go 1.18+所有版本
//
// WARNING: 返回的[]byte与底层字符串共享内存
// 不要修改返回的[]byte
func UnsafeBytesFromString(s string) []byte {
	return unsafeBytesFromString_impl(s)
}

// SafeStringFromBytes 安全转换[]byte到string（带内存拷贝）
func SafeStringFromBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return string(b)
}

// SafeBytesFromString 安全转换string到[]byte（带内存拷贝）
func SafeBytesFromString(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return []byte(s)
}

// GetConverterInfo 返回当前使用的转换器信息
func GetConverterInfo() string {
	return implementationInfo
}
