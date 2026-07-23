package str

import (
	"runtime"
	"testing"
)

func TestUnsafeStringFromBytes(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"empty", []byte{}, ""},
		{"simple", []byte("hello"), "hello"},
		{"with spaces", []byte("hello world"), "hello world"},
		{"unicode", []byte("你好世界"), "你好世界"},
		{"special chars", []byte("!@#$%^&*()"), "!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UnsafeStringFromBytes(tt.input)
			if got != tt.want {
				t.Errorf("UnsafeStringFromBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnsafeBytesFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []byte
	}{
		{"empty", "", nil},
		{"simple", "hello", []byte("hello")},
		{"with spaces", "hello world", []byte("hello world")},
		{"unicode", "你好世界", []byte("你好世界")},
		{"special chars", "!@#$%^&*()", []byte("!@#$%^&*()")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UnsafeBytesFromString(tt.input)
			if string(got) != string(tt.want) {
				t.Errorf("UnsafeBytesFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	testCases := []string{
		"",
		"hello",
		"hello world",
		"你好世界",
		"!@#$%^&*()",
		"This is a longer string to test the conversion",
	}

	for _, original := range testCases {
		t.Run("roundtrip_"+original, func(t *testing.T) {
			// string -> []byte -> string
			bytes := UnsafeBytesFromString(original)
			backToString := UnsafeStringFromBytes(bytes)
			if backToString != original {
				t.Errorf("Round trip failed: original=%q, got=%q", original, backToString)
			}

			// []byte -> string -> []byte
			originalBytes := []byte(original)
			str := UnsafeStringFromBytes(originalBytes)
			backToBytes := UnsafeBytesFromString(str)
			if string(backToBytes) != string(originalBytes) {
				t.Errorf("Round trip failed: originalBytes=%v, got=%v", originalBytes, backToBytes)
			}
		})
	}
}

func TestSafeConversions(t *testing.T) {
	testStr := "hello world"
	testBytes := []byte("hello world")

	// Test safe conversions
	safeStr := SafeStringFromBytes(testBytes)
	if safeStr != testStr {
		t.Errorf("SafeStringFromBytes() = %v, want %v", safeStr, testStr)
	}

	safeBytes := SafeBytesFromString(testStr)
	if string(safeBytes) != string(testBytes) {
		t.Errorf("SafeBytesFromString() = %v, want %v", safeBytes, testBytes)
	}
}

func TestZeroCopyProperty(t *testing.T) {
	original := "test string for zero copy"

	// Test string -> []byte with zero copy
	converted := UnsafeBytesFromString(original)
	if len(converted) > 0 {
		// Verify that the data content is the same
		if string(converted) != original {
			t.Errorf("Zero copy conversion failed: got %q, want %q", string(converted), original)
		}

		// Note: You cannot directly compare memory addresses because string and []byte have different memory layouts
		// But we can verify whether the transformation maintains data consistency
	}

	// Testing[] byte -> string zero copy
	originalBytes := []byte("test bytes for zero copy")
	convertedStr := UnsafeStringFromBytes(originalBytes)
	if convertedStr != string(originalBytes) {
		t.Errorf("Zero copy conversion failed: got %q, want %q", convertedStr, string(originalBytes))
	}
}

func TestGetConverterInfo(t *testing.T) {
	info := GetConverterInfo()
	validImplementations := []string{
		"Go 1.20+ official unsafe",
		"Classic unsafe (Go 1.18+)",
		"Safe fallback",
	}

	found := false
	for _, valid := range validImplementations {
		if info == valid {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("GetConverterInfo() returned unexpected implementation: %q", info)
	}

	t.Logf("Using implementation: %s", info)
	t.Logf("Go version: %s", runtime.Version())
	t.Logf("Platform: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func TestBuildTagSelection(t *testing.T) {
	t.Logf("Go version: %s", runtime.Version())
	t.Logf("Selected implementation: %s", GetConverterInfo())

	// Basic functional testing ensures that the implementation of build label selection works properly
	testStr := "build tag test"
	testBytes := []byte(testStr)

	// Round-trip conversion testing
	convertedBytes := UnsafeBytesFromString(testStr)
	backToString := UnsafeStringFromBytes(convertedBytes)
	if backToString != testStr {
		t.Errorf("Round trip string->bytes->string failed: got %q, want %q", backToString, testStr)
	}

	convertedStr := UnsafeStringFromBytes(testBytes)
	backToBytes := UnsafeBytesFromString(convertedStr)
	if string(backToBytes) != string(testBytes) {
		t.Errorf("Round trip bytes->string->bytes failed: got %v, want %v", backToBytes, testBytes)
	}
}

// === Performance Benchmark ===

func BenchmarkUnsafeStringFromBytes(b *testing.B) {
	data := []byte("This is a test string for benchmarking unsafe conversion performance")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = UnsafeStringFromBytes(data)
	}
}

func BenchmarkSafeStringFromBytes(b *testing.B) {
	data := []byte("This is a test string for benchmarking safe conversion performance")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SafeStringFromBytes(data)
	}
}

func BenchmarkUnsafeBytesFromString(b *testing.B) {
	data := "This is a test string for benchmarking unsafe conversion performance"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = UnsafeBytesFromString(data)
	}
}

func BenchmarkSafeBytesFromString(b *testing.B) {
	data := "This is a test string for benchmarking safe conversion performance"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SafeBytesFromString(data)
	}
}

func BenchmarkRoundTrip_Unsafe(b *testing.B) {
	data := "This is a test string for benchmarking round trip conversion"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes := UnsafeBytesFromString(data)
		_ = UnsafeStringFromBytes(bytes)
	}
}

func BenchmarkRoundTrip_Safe(b *testing.B) {
	data := "This is a test string for benchmarking round trip conversion"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes := SafeBytesFromString(data)
		_ = SafeStringFromBytes(bytes)
	}
}

// Compare the performance of strings of different sizes
func BenchmarkConversionSizes(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096}

	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte('a' + (i % 26))
		}

		b.Run("unsafe_"+string(rune(size)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = UnsafeStringFromBytes(data)
			}
		})

		b.Run("safe_"+string(rune(size)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = SafeStringFromBytes(data)
			}
		})
	}
}
