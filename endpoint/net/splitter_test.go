package net

import (
	"bufio"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/rulego/rulego/test/assert"
)

// readAllPackets reads packets until the reader is exhausted and returns them
// as strings. The final entry is the trailing partial packet (if any) that was
// returned together with io.EOF.
func readAllPackets(t *testing.T, splitter PacketSplitter, reader io.Reader) []string {
	t.Helper()
	bufReader := bufio.NewReader(reader)
	var packets []string
	for {
		data, err := splitter.ReadPacket(bufReader)
		if len(data) > 0 {
			packets = append(packets, string(data))
		}
		if err != nil {
			assert.Equal(t, io.EOF, err)
			return packets
		}
	}
}

func TestDelimiterSplitterReadPacket(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		input     string
		want      []string
	}{
		{
			name:      "simple",
			delimiter: "\r\n",
			input:     "abc\r\ndef\r\n",
			want:      []string{"abc\r\n", "def\r\n"},
		},
		{
			// A mismatch after a partial match must re-test the current byte:
			// the second '\r' starts a new match of "\r\n".
			name:      "repeated first byte before delimiter",
			delimiter: "\r\n",
			input:     "abc\r\r\ndef\r\n",
			want:      []string{"abc\r\r\n", "def\r\n"},
		},
		{
			name:      "repeated first byte of multi byte delimiter",
			delimiter: "ab",
			input:     "xaabyz",
			want:      []string{"xaab", "yz"},
		},
		{
			name:      "self overlapping delimiter",
			delimiter: "aab",
			input:     "aaabxy",
			want:      []string{"aaab", "xy"},
		},
		{
			name:      "delimiter with repeated prefix",
			delimiter: "abab",
			input:     "abaabababxyz",
			want:      []string{"abaabab", "abxyz"},
		},
		{
			name:      "delimiter made of one repeated byte",
			delimiter: "aa",
			input:     "aaa",
			want:      []string{"aa", "a"},
		},
		{
			name:      "single byte delimiter",
			delimiter: ";",
			input:     "a;b;;c",
			want:      []string{"a;", "b;", ";", "c"},
		},
		{
			name:      "partial delimiter at end of stream is returned with EOF",
			delimiter: "\r\n",
			input:     "abc\r\ndef\r",
			want:      []string{"abc\r\n", "def\r"},
		},
		{
			name:      "no delimiter",
			delimiter: "\r\n",
			input:     "abcdef",
			want:      []string{"abcdef"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splitter := &DelimiterSplitter{Delimiter: []byte(tt.delimiter)}
			assert.Equal(t, tt.want, readAllPackets(t, splitter, strings.NewReader(tt.input)))

			// The result must not depend on how the underlying reader chunks data.
			splitter = &DelimiterSplitter{Delimiter: []byte(tt.delimiter)}
			assert.Equal(t, tt.want, readAllPackets(t, splitter,
				iotest.OneByteReader(strings.NewReader(tt.input))))
		})
	}
}

func TestDelimiterSplitterHexDelimiter(t *testing.T) {
	splitter, err := CreatePacketSplitter(Config{
		PacketMode: PacketModeDelimiter.String(),
		Delimiter:  "0x0D0A",
	})
	assert.Nil(t, err)
	got := readAllPackets(t, splitter, strings.NewReader("a\r\r\nb\r\n"))
	assert.Equal(t, []string{"a\r\r\n", "b\r\n"}, got)
}

func TestDelimiterFallback(t *testing.T) {
	tests := []struct {
		delimiter string
		want      []int
	}{
		{"", []int{}},
		{"a", []int{0}},
		{"ab", []int{0, 0}},
		{"aa", []int{0, 1}},
		{"abab", []int{0, 0, 1, 2}},
		{"aabaaab", []int{0, 1, 0, 1, 2, 2, 3}},
		{"\r\n", []int{0, 0}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, delimiterFallback([]byte(tt.delimiter)))
	}
}
