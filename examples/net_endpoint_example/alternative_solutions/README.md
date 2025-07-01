# NET端点数据包分割模式示例

这个目录包含了RuleGo NET端点支持的各种数据包分割模式的示例配置。NET端点现在支持更加灵活和通用的数据包分割方式，适用于各种网络协议和数据格式。

## 支持的分割模式

### 1. 按行分割 (line) - 默认模式
**适用场景**: 文本协议、基于行的协议（如HTTP、SMTP等）

**配置示例**:
```json
{
  "packetMode": "line"
}
```
- 默认模式，向后兼容
- 以 `\n` 或 `\r\n` 作为数据包分隔符
- 适用于大多数文本协议

### 2. 固定长度分割 (fixed)
**适用场景**: 固定长度的二进制协议、电报协议

**配置示例**:
```json
{
  "packetMode": "fixed",
  "packetSize": 16
}
```
- 每个数据包固定为指定字节数
- 适用于结构化的二进制协议
- 例如：设备ID(4字节) + 命令(4字节) + 数据(8字节)

### 3. 自定义分隔符分割 (delimiter)
**适用场景**: 使用特殊分隔符的协议、多字节分隔符

**配置示例**:
```json
{
  "packetMode": "delimiter",
  "delimiter": "0x0D0A"
}
```
- 支持字符串分隔符: `"delimiter": "END"`
- 支持十六进制分隔符: `"delimiter": "0x0D0A"`
- 支持多字节分隔符: `"delimiter": "0x1A2B3C"`

### 4. 长度前缀分割 (length_prefix)
**适用场景**: 变长数据包、消息队列协议、自定义协议

**配置示例**:
```json
{
  "packetMode": "length_prefix",
  "lengthPrefixSize": 2,
  "lengthPrefixBigEndian": true,
  "lengthIncludesPrefix": false
}
```
- `lengthPrefixSize`: 长度字段大小(1-4字节)
- `lengthPrefixBigEndian`: 字节序(true=大端序, false=小端序)
- `lengthIncludesPrefix`: 长度是否包含前缀本身

## 配置参数详解

### 通用参数
- **packetMode**: 数据包分割模式 (必填)
- **maxPacketSize**: 最大数据包大小，防止恶意数据包 (默认64KB)

### 固定长度模式 (fixed)
- **packetSize**: 固定数据包大小 (字节)

### 自定义分隔符模式 (delimiter)
- **delimiter**: 分隔符
  - 字符串格式: `"END"`, `"|"`
  - 十六进制格式: `"0x0A"`, `"0x0D0A"`, `"0x1A2B3C"`

### 长度前缀模式 (length_prefix)
- **lengthPrefixSize**: 长度字段大小 (1-4字节)
- **lengthPrefixBigEndian**: 字节序 (true/false)
- **lengthIncludesPrefix**: 长度是否包含前缀长度 (true/false)

## 使用建议

### 性能考虑
1. **固定长度** - 性能最好，CPU开销最小
2. **长度前缀** - 性能良好，支持变长数据
3. **自定义分隔符** - 性能中等，需要逐字节扫描
4. **按行分割** - 性能中等，针对文本优化

### 协议选择
1. **文本协议** → 使用 `line` 模式
2. **固定格式二进制** → 使用 `fixed` 模式
3. **变长二进制** → 使用 `length_prefix` 模式
4. **特殊分隔符** → 使用 `delimiter` 模式

## 安全考虑

### 防护措施
- **maxPacketSize**: 限制最大包大小，防止内存耗尽攻击
- **读取超时**: 防止慢速攻击
- **连接限制**: 在应用层实现连接数限制

### 建议配置
```json
{
  "readTimeout": 30,
  "maxPacketSize": 65536
}
```

## 错误处理

### 常见错误
1. **包大小超限**: 数据包超过 `maxPacketSize`
2. **格式错误**: 长度前缀格式错误
3. **连接超时**: 超过 `readTimeout` 时间

### 错误日志
```
UDP packet too large: 100000 > 65536 from 127.0.0.1:12345
failed to create packet splitter: packetSize must be greater than 0 for fixed mode
invalid packet length: 5 < prefix size 8
```

## 示例文件说明

- `fixed_length_example.json` - 固定长度协议示例
- `length_prefix_example.json` - 长度前缀协议示例  
- `custom_delimiter_example.json` - 自定义分隔符示例

每个示例文件都包含完整的端点配置和数据处理逻辑，可以直接用于测试和学习。 