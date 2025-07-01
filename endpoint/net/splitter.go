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

// PacketMode 数据包分割模式
type PacketMode string

const (
	// PacketModeLine 按行分割（默认模式，以\n或\r\n分割）
	PacketModeLine PacketMode = "line"
	// PacketModeFixed 固定长度分割
	PacketModeFixed PacketMode = "fixed"
	// PacketModeDelimiter 自定义分隔符分割
	PacketModeDelimiter PacketMode = "delimiter"

	// 长度前缀模式（4种组合）
	// PacketModeLengthPrefixLE 长度前缀，小端序，长度不包含前缀
	PacketModeLengthPrefixLE PacketMode = "length_prefix_le"
	// PacketModeLengthPrefixBE 长度前缀，大端序，长度不包含前缀
	PacketModeLengthPrefixBE PacketMode = "length_prefix_be"
	// PacketModeLengthPrefixLEInc 长度前缀，小端序，长度包含前缀
	PacketModeLengthPrefixLEInc PacketMode = "length_prefix_le_inc"
	// PacketModeLengthPrefixBEInc 长度前缀，大端序，长度包含前缀
	PacketModeLengthPrefixBEInc PacketMode = "length_prefix_be_inc"
)

// String 返回模式的字符串表示
func (p PacketMode) String() string {
	return string(p)
}

// IsValid 检查模式是否有效
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

// IsLengthPrefixMode 检查是否为长度前缀模式
func (p PacketMode) IsLengthPrefixMode() bool {
	switch p {
	case PacketModeLengthPrefixLE, PacketModeLengthPrefixBE,
		PacketModeLengthPrefixLEInc, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false
	}
}

// IsBigEndian 是否为大端序
func (p PacketMode) IsBigEndian() bool {
	switch p {
	case PacketModeLengthPrefixBE, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false // 默认小端序
	}
}

// IncludesPrefix 长度是否包含前缀本身
func (p PacketMode) IncludesPrefix() bool {
	switch p {
	case PacketModeLengthPrefixLEInc, PacketModeLengthPrefixBEInc:
		return true
	default:
		return false // 默认不包含
	}
}

// PacketSplitter 数据包分割器接口
type PacketSplitter interface {
	// ReadPacket 从连接中读取一个完整的数据包
	ReadPacket(reader *bufio.Reader) ([]byte, error)
}

// LineSplitter 按行分割的数据包分割器
type LineSplitter struct{}

func (s *LineSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	data, err := reader.ReadBytes('\n')
	if err != nil {
		return data, err
	}
	// 去掉换行符分隔符
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
		// 如果是\r\n，也去掉\r
		if len(data) > 0 && data[len(data)-1] == '\r' {
			data = data[:len(data)-1]
		}
	}
	return data, nil
}

// FixedLengthSplitter 固定长度数据包分割器
type FixedLengthSplitter struct {
	PacketSize int
}

func (s *FixedLengthSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	data := make([]byte, s.PacketSize)
	_, err := io.ReadFull(reader, data)
	return data, err
}

// DelimiterSplitter 自定义分隔符数据包分割器
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

		// 检查是否匹配分隔符
		if b == s.Delimiter[delimiterIndex] {
			delimiterIndex++
			if delimiterIndex == len(s.Delimiter) {
				// 找到完整分隔符，返回包含分隔符的完整数据
				return buffer, nil
			}
		} else {
			delimiterIndex = 0
		}
	}
}

// LengthPrefixSplitter 长度前缀数据包分割器
type LengthPrefixSplitter struct {
	PrefixSize     int  // 长度前缀的字节数（1-4字节）
	BigEndian      bool // 是否使用大端序
	IncludesPrefix bool // 长度是否包含前缀本身
	MaxPacketSize  int  // 最大数据包大小
}

func (s *LengthPrefixSplitter) ReadPacket(reader *bufio.Reader) ([]byte, error) {
	// 读取长度前缀
	prefixBytes := make([]byte, s.PrefixSize)
	_, err := io.ReadFull(reader, prefixBytes)
	if err != nil {
		return nil, err
	}

	// 解析长度值
	var length uint32
	if s.BigEndian {
		switch s.PrefixSize {
		case 1:
			length = uint32(prefixBytes[0])
		case 2:
			length = uint32(binary.BigEndian.Uint16(prefixBytes))
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
		case 4:
			length = binary.LittleEndian.Uint32(prefixBytes)
		default:
			return nil, fmt.Errorf("unsupported prefix size: %d", s.PrefixSize)
		}
	}

	// 检查数据包大小限制
	if int(length) > s.MaxPacketSize {
		return nil, fmt.Errorf("packet too large: %d > %d", length, s.MaxPacketSize)
	}

	// 根据IncludesPrefix确定数据长度
	var dataLength uint32
	if s.IncludesPrefix {
		if length < uint32(s.PrefixSize) {
			return nil, fmt.Errorf("invalid packet length: %d < prefix size %d", length, s.PrefixSize)
		}
		dataLength = length - uint32(s.PrefixSize)
	} else {
		dataLength = length
	}

	// 读取数据部分
	data := make([]byte, dataLength)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}

	// 返回包含长度前缀的完整数据包
	result := make([]byte, 0, len(prefixBytes)+len(data))
	result = append(result, prefixBytes...)
	result = append(result, data...)
	return result, nil
}

// CreatePacketSplitter 根据配置创建数据包分割器
func CreatePacketSplitter(config Config) (PacketSplitter, error) {
	// 默认为line模式
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

		// 解析分隔符（支持十六进制格式）
		var delimiter []byte
		if strings.HasPrefix(config.Delimiter, HexPrefix) || strings.HasPrefix(config.Delimiter, HexPrefixUp) {
			// 十六进制格式: 0x0A0D
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
			// 直接使用字符串作为分隔符
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
