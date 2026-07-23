package net

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// PacketMode packet splitting mode
type PacketMode string

const (
	// PacketModeLine line splitting (default mode, split by \n or \r\n)
	PacketModeLine PacketMode = "line"
	// PacketModeFixed fixed-length split
	PacketModeFixed PacketMode = "fixed"
	// PacketModeDelimiter Custom delimiter splits
	PacketModeDelimiter PacketMode = "delimiter"

	// Length prefix mode (4 combinations)
	// PacketModeLengthPrefixLE length prefix, small-termination, length does not include prefixes
	PacketModeLengthPrefixLE PacketMode = "length_prefix_le"
	// PacketModeLengthPrefixBE length prefix, major terminology, length does not include prefixes
	PacketModeLengthPrefixBE PacketMode = "length_prefix_be"
	// PacketModeLengthPrefixLEInc length prefix, small-terminology, length includes prefix
	PacketModeLengthPrefixLEInc PacketMode = "length_prefix_le_inc"
	// PacketModeLengthPrefixBEInc length prefix, major terminology, length containing prefix
	PacketModeLengthPrefixBEInc PacketMode = "length_prefix_be_inc"
)

// String returns the pattern of string representation
func (p PacketMode) String() string {
	return string(p)
}

// IsValid checks whether the mode is valid
func (p PacketMode) IsValid() bool {
	switch p {
	case PacketModeLine, PacketModeFixed, PacketModeDelimiter,
		PacketModeLengthPrefixLE, PacketModeLengthPrefixBE,
		PacketModeLengthPrefixLEInc, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false
	}
}

// IsLengthPrefixMode checks whether it is a length prefix mode
func (p PacketMode) IsLengthPrefixMode() bool {
	switch p {
	case PacketModeLengthPrefixLE, PacketModeLengthPrefixBE,
		PacketModeLengthPrefixLEInc, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false
	}
}

// Is IsBigEndian a large endology sequence?
func (p PacketMode) IsBigEndian() bool {
	switch p {
	case PacketModeLengthPrefixBE, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false // Default is the small end-to-end order
	}
}

// IncludesPrefix: Does the length contain the prefix itself?
func (p PacketMode) IncludesPrefix() bool {
	switch p {
	case PacketModeLengthPrefixLEInc, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false // Not included by default
	}
}

// PacketSplitter interface
type PacketSplitter interface {
	// ReadPacket reads a complete data packet from a connection
	ReadPacket(reader *bufio.Reader) ([]byte, error)
}

// LineSplitter is a packet splitter by row
type LineSplitter struct{}

func (s *LineSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	data, err := reader.ReadBytes('\n')
	if err != nil {
		return data, err
	}
	// Remove line break separators
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
		// If it is \r\n, also remove \r
		if len(data) > 0 && data[len(data)-1] == '\r' {
			data = data[:len(data)-1]
		}
	}
	return data, nil
}

// FixedLengthSplitter
type FixedLengthSplitter struct {
	PacketSize int
}

func (s *FixedLengthSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	data := make([]byte, s.PacketSize)
	_, err := io.ReadFull(reader, data)
	return data, err
}

// DelimiterSplitter is a custom delimiter packet splitter
type DelimiterSplitter struct {
	Delimiter []byte
}

func (s *DelimiterSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	var buffer []byte
	delimiterIndex := 0

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return buffer, err
		}

		buffer = append(buffer, b)

		// Check if the separator matches
		if b == s.Delimiter[delimiterIndex] {
			delimiterIndex++
			if delimiterIndex == len(s.Delimiter) {
				// Find the complete delimiter and return the complete data containing the separator
				return buffer, nil
			}
		} else {
			delimiterIndex = 0
		}
	}
}

// LengthPrefixSplitter is a packet splitter with the length prefix
type LengthPrefixSplitter struct {
	PrefixSize     int  // Number of bytes of length prefixes (1-4 bytes)
	BigEndian      bool // Whether to use large-scale sequence
	IncludesPrefix bool // Does the length contain the prefix itself?
	MaxPacketSize  int  // Maximum packet size
}

func (s *LengthPrefixSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	// Read the length prefix
	prefixBytes := make([]byte, s.PrefixSize)
	_, err := io.ReadFull(reader, prefixBytes)
	if err != nil {
		return nil, err
	}

	// Parse length values
	var length uint32
	if s.BigEndian {
		switch s.PrefixSize {
		case 1:
			length = uint32(prefixBytes[0])
		case 2:
			length = uint32(binary.BigEndian.Uint16(prefixBytes))
		case 3:
			// Major Sequence 3 bytes: Add 0 before it to become 4 bytes
			length = uint32(prefixBytes[0])<<16 | uint32(prefixBytes[1])<<8 | uint32(prefixBytes[2])
		case 4:
			length = binary.BigEndian.Uint32(prefixBytes)
		default:
			return nil, fmt.Errorf("unsupported prefix size: %d", s.PrefixSize)
		}
	} else {
		switch s.PrefixSize {
		case 1:
			length = uint32(prefixBytes[0])
		case 2:
			length = uint32(binary.LittleEndian.Uint16(prefixBytes))
		case 3:
			// Small end-order 3-byte: Combine 3 bytes according to small-end-order order
			length = uint32(prefixBytes[0]) | uint32(prefixBytes[1])<<8 | uint32(prefixBytes[2])<<16
		case 4:
			length = binary.LittleEndian.Uint32(prefixBytes)
		default:
			return nil, fmt.Errorf("unsupported prefix size: %d", s.PrefixSize)
		}
	}

	// Check the packet size limits
	if int(length) > s.MaxPacketSize {
		return nil, fmt.Errorf("packet too large: %d > %d", length, s.MaxPacketSize)
	}

	// Determine the data length based on IncludesPrefix
	var dataLength uint32
	if s.IncludesPrefix {
		if length < uint32(s.PrefixSize) {
			return nil, fmt.Errorf("invalid packet length: %d < prefix size %d", length, s.PrefixSize)
		}
		dataLength = length - uint32(s.PrefixSize)
	} else {
		dataLength = length
	}

	// Data reading section
	data := make([]byte, dataLength)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}

	// Returns the complete packet containing the length prefix
	result := make([]byte, 0, len(prefixBytes)+len(data))
	result = append(result, prefixBytes...)
	result = append(result, data...)
	return result, nil
}

// CreatePacketSplitter creates a packet splitter based on the configuration
func CreatePacketSplitter(config Config) (PacketSplitter, error) {
	// The default is Line mode
	mode := strings.ToLower(config.PacketMode)
	if mode == "" {
		mode = PacketModeLine.String()
	}

	switch mode {
	case PacketModeLine.String():
		return &LineSplitter{}, nil

	case PacketModeFixed.String():
		if config.PacketSize <= 0 {
			return nil, errors.New("packetSize must be greater than 0 for fixed mode")
		}
		return &FixedLengthSplitter{
			PacketSize: config.PacketSize,
		}, nil

	case PacketModeDelimiter.String():
		if config.Delimiter == "" {
			return nil, errors.New("delimiter must be specified for delimiter mode")
		}

		// Parse separator (supports hexadecimal format)
		var delimiter []byte
		if strings.HasPrefix(config.Delimiter, HexPrefix) || strings.HasPrefix(config.Delimiter, HexPrefixUp) {
			// Hexadecimal format: 0x0A0D
			hexStr := config.Delimiter[2:]
			if len(hexStr)%2 != 0 {
				return nil, errors.New("invalid hex delimiter format")
			}
			delimiter = make([]byte, len(hexStr)/2)
			for i := 0; i < len(hexStr); i += 2 {
				b, err := hex.DecodeString(hexStr[i : i+2])
				if err != nil {
					return nil, fmt.Errorf("invalid hex delimiter: %v", err)
				}
				delimiter[i/2] = b[0]
			}
		} else {
			// Directly use strings as delimiters
			delimiter = []byte(config.Delimiter)
		}

		return &DelimiterSplitter{
			Delimiter: delimiter,
		}, nil

	case PacketModeLengthPrefixLE.String(), PacketModeLengthPrefixBE.String(),
		PacketModeLengthPrefixLEInc.String(), PacketModeLengthPrefixBEInc.String():
		if config.PacketSize <= 0 || config.PacketSize > 4 {
			return nil, errors.New("packetSize must be between 1 and 4 for length_prefix mode")
		}

		bigEndian := strings.Contains(mode, BigEndianSuffix)
		includesPrefix := strings.Contains(mode, IncludesPrefixSuffix)

		return &LengthPrefixSplitter{
			PrefixSize:     config.PacketSize,
			BigEndian:      bigEndian,
			IncludesPrefix: includesPrefix,
			MaxPacketSize:  config.MaxPacketSize,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported packet mode: %s", config.PacketMode)
	}
}
