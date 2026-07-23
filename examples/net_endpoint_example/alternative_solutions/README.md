# NET Example of endpoint packet segmentation pattern

This directory contains sample configurations for various packet splitting modes supported by RuleGo NET endpoints. NET endpoints now support more flexible and universal packet segmentation methods, suitable for various network protocols and data formats.

## Supported split modes

### 1. Split by line (line) - Default mode
**Applicable Scenarios**: Text protocols, line-based protocols (such as HTTP, SMTP, etc.)

**Configuration Example**:
```json
{
  "packetMode": "line"
}
```
- Default mode, backward compatible
- Use `\n` or `\r\n` as packet separators
- Suitable for most text protocols

### 2. Fixed length division (fixed)
**Applicable Scenarios**: Fixed-length binary protocols, telegraph protocols

**Configuration Example**:
```json
{
  "packetMode": "fixed",
  "packetSize": 16
}
```
- Each packet is fixed to a specified number of bytes
- Suitable for structured binary protocols
- For example: Device ID(4 bytes) + Command (4 bytes) + Data (8 bytes)

### 3. Custom delimiter split (delimiter)
**Applicable Scenarios**: Protocols using special separators, multi-byte separators

**Configuration Example**:
```json
{
  "packetMode": "delimiter",
  "delimiter": "0x0D0A"
}
```
- Supports string delimiter: `"delimiter": "END"`
- Supports hexadecimal delimiter: `"delimiter": "0x0D0A"`
- Supports multi-byte delimiters: `"delimiter": "0x1A2B3C"`

### 4. Length prefix segmentation (length_prefix)
**Applicable Scenarios**: Variable Length Data Packets, Message Queue Protocols, Custom Protocols

**Configuration Example**:
```json
{
  "packetMode": "length_prefix",
  "lengthPrefixSize": 2,
  "lengthPrefixBigEndian": true,
  "lengthIncludesPrefix": false
}
```
- `lengthPrefixSize`: Length field size (1-4 bytes)
- `lengthPrefixBigEndian`: Byte order (true= big-endian, false= little-endian)
- `lengthIncludesPrefix`: Whether the length contains the prefix itself

## Detailed Explanation of Configuration Parameters

### General Parameters
- **packetMode**: Packet splitting mode (required)
- **maxPacketSize**: Maximum packet size to prevent malicious packets (default 64KB)

### Fixed Length Mode (fixed)
- **packetSize**: Fixed packet size (bytes)

### Custom Separator Patterns (delimiter)
- **delimiter**: Separator
  - String format: `"END"`, `"|"`
  - Hexadecimal format: `"0x0A"`, `"0x0D0A"`, `"0x1A2B3C"`

### Length Prefix Mode (length_prefix)
- **lengthPrefixSize**: Length field size (1-4 bytes)
- **lengthPrefixBigEndian**: Byte order (true/false)
- **lengthIncludesPrefix**: Does the length include the length of the prefix (true/false)

## Usage Recommendations

### Performance considerations
1. **Fixed length** - Best performance, CPU Minimal overhead
2. **Length Prefix** - Good performance, supports variable data length
3. **Custom delimiter** - Medium performance, requires byte-byte scanning
4. **Split by line** - Moderate performance, optimized for text

### Protocol Selection
1. **Text Protocol** → Use `line` Mode
2. **Fixed Binary Format** → Use `fixed` Mode
3. **Variable Binary** → Use `length_prefix` Mode
4. **Special Separator** → Use `delimiter` Mode

## Safety considerations

### Protective Measures
- **maxPacketSize**: Limits maximum packet size to prevent memory exhaustion attacks
- **Read timeout**: Prevents slow attacks
- **Connection Limit**: Implement a connection limit at the application layer

### Recommended configuration
```json
{
  "readTimeout": 30,
  "maxPacketSize": 65536
}
```

## Error Handling

### Common Errors
1. **Packet size limit**: Data packets exceed `maxPacketSize`
2. **Formatting errors**: Incorrect format of length prefixes
3. **Connection timeout**: Exceeding `readTimeout` time

### Error log
```
UDP packet too large: 100000 > 65536 from 127.0.0.1:12345
failed to create packet splitter: packetSize must be greater than 0 for fixed mode
invalid packet length: 5 < prefix size 8
```

## Example document explanation

- `fixed_length_example.json` - Examples of fixed-length agreements
- `length_prefix_example.json` - Example of length prefix protocol  
- `custom_delimiter_example.json` - Example of custom delimiters

Each sample file contains complete endpoint configuration and data processing logic, which can be used directly for testing and learning. 
