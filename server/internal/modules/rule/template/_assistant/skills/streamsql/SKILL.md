---
name: streamsql
description: 当需要创建实时数据聚合、窗口统计、流式计算、设备数据分组聚合等规则链时使用，或当使用 x/streamAggregator 和 x/streamTransform 组件时使用。包含完整的 SQL 语法、窗口类型和配置示例
---

# StreamSQL 流式计算组件

RuleGo 通过 `x/streamAggregator` 和 `x/streamTransform` 两个组件提供流式 SQL 处理能力。

## 组件选择

| 需求 | 组件 | 说明 |
|------|------|------|
| 聚合统计（AVG/COUNT/SUM/MAX/MIN） | `x/streamAggregator` | 必须包含 GROUP BY + 窗口函数 |
| 过滤、字段选择、计算转换 | `x/streamTransform` | 不能包含聚合函数 |
| 窗口聚合 + 实时统计 | `x/streamAggregator` | 时间窗口/计数窗口触发聚合 |

**用错组件会初始化失败**：aggregator 的 SQL 不能缺少聚合函数，transform 的 SQL 不能包含聚合函数。

## x/streamAggregator 流聚合器

### 数据流

```
输入数据 → 添加到聚合流 → 原始数据通过 Success 链继续传递
聚合触发（窗口到期） → 聚合结果通过 window_event 链传递
```

### 配置

```json
{
  "type": "x/streamAggregator",
  "name": "流聚合器",
  "configuration": {
    "sql": "SELECT deviceId, AVG(temperature) as avg_temp, COUNT(*) as count FROM stream GROUP BY deviceId, TumblingWindow('30s')"
  }
}
```

### 连接类型

| 连接 | 说明 |
|------|------|
| Success | 原始数据成功加入聚合流，继续传递原始消息 |
| Failure | 处理失败（非JSON数据、SQL错误等） |
| **window_event** | 窗口触发后的聚合结果（这是关键！） |

### 聚合结果消息

聚合结果通过 `window_event` 链输出，消息格式：

- `msgType`: `window_event`
- `metadata`: 包含 `queryType=aggregation`、`resultType=window_triggered`
- `msg.data`: 聚合结果 JSON 数组，如 `[{"deviceId":"sensor001","avg_temp":25.5,"count":10}]`

## x/streamTransform 流转换器

### 配置

```json
{
  "type": "x/streamTransform",
  "name": "数据转换",
  "configuration": {
    "sql": "SELECT deviceId, temperature * 1.8 + 32 as fahrenheit FROM stream WHERE temperature > 20"
  }
}
```

### 连接类型

| 连接 | 说明 |
|------|------|
| Success | 转换成功且符合 WHERE 条件 |
| Failure | 转换失败或不符合 WHERE 条件 |

转换后的数据直接替换原始消息的 data，通过 Success 链传递。不符合 WHERE 条件的数据走 Failure 链。

### 元数据标记

Success 时 metadata 设置 `match=true`，Failure 时设置 `match=false`，可用于下游节点判断。

## SQL 语法参考

### 完整语法结构

```sql
SELECT [DISTINCT] select_list
FROM stream
[WHERE condition]
[GROUP BY grouping_element [, ...]]
[HAVING condition]
[ORDER BY ordering_list]
[LIMIT count]
[WITH (option = value [, ...])]
```

### SELECT 字段

```sql
-- 直接引用字段
SELECT deviceId, temperature FROM stream

-- 选择所有字段
SELECT * FROM stream

-- 计算字段
SELECT temperature * 1.8 + 32 as fahrenheit FROM stream

-- 别名（AS 可省略）
SELECT temperature temp, humidity hum FROM stream

-- 去重
SELECT DISTINCT deviceType FROM stream

-- 聚合函数
SELECT AVG(temperature) as avg_temp, COUNT(*) as count FROM stream

-- 多字段聚合
SELECT deviceId, AVG(temperature) as avg_temp, SUM(humidity) as total_humidity
FROM stream GROUP BY deviceId, TumblingWindow('1m')
```

### 嵌套字段访问

支持点号、数组索引、Map 键访问：

```sql
-- 点号语法
SELECT device.info.name, device.location.building FROM stream

-- 深层嵌套
SELECT sensor.data.temperature.value FROM stream

-- 数组索引（支持负数索引）
SELECT items[0].name, readings[-1] FROM stream

-- Map 键访问
SELECT config['host'], metadata.building_info['architect'] FROM stream

-- 混合访问
SELECT floors[0].rooms[2]['name'] FROM stream
```

嵌套字段可在 WHERE、GROUP BY、HAVING 中使用：

```sql
SELECT device.type, AVG(sensor.temperature)
FROM stream
WHERE device.info.status = 'active'
GROUP BY device.type, TumblingWindow('1m')
```

### CASE 条件表达式

```sql
-- 搜索 CASE
SELECT deviceId,
       CASE
           WHEN temperature > 30 THEN '高温'
           WHEN temperature > 20 THEN '正常'
           ELSE '低温'
       END as temp_level
FROM stream

-- 简单 CASE
SELECT deviceId,
       CASE status
           WHEN 'active' THEN 1
           WHEN 'inactive' THEN 0
           ELSE -1
       END as status_code
FROM stream

-- CASE 在聚合函数内部（条件计数）
SELECT deviceId,
       SUM(CASE WHEN temperature > 30 THEN 1 ELSE 0 END) as hot_count
FROM stream
GROUP BY deviceId, TumblingWindow('1m')
```

### WHERE 条件（可选）

```sql
-- 比较操作
WHERE temperature > 25
WHERE deviceId != 'test001'

-- 逻辑组合
WHERE temperature > 20 AND humidity < 80
WHERE (temperature > 30 OR humidity > 90) AND deviceId LIKE 'sensor%'

-- 范围
WHERE temperature BETWEEN 20 AND 30
WHERE deviceType IN ('temperature', 'humidity')

-- NULL 检查
WHERE temperature IS NOT NULL
WHERE error_msg IS NULL

-- 模式匹配
WHERE deviceId LIKE 'sensor%'       -- 以sensor开头
WHERE location LIKE '%room%'        -- 包含room
WHERE deviceId LIKE 'dev___'        -- dev后跟3个字符

-- 嵌套字段条件
WHERE device.info.status = 'active'
WHERE sensor.data.temperature > 25

-- NOT
WHERE deviceId NOT IN ('test001', 'test002')
WHERE NOT (temperature < 0)
```

### HAVING 子句

过滤聚合结果（WHERE 过滤原始数据，HAVING 过滤聚合结果）：

```sql
SELECT deviceId, AVG(temperature) as avg_temp, COUNT(*) as count
FROM stream
WHERE temperature > 0
GROUP BY deviceId, TumblingWindow('30s')
HAVING AVG(temperature) > 25 AND COUNT(*) >= 5
```

### 窗口函数（streamAggregator 必须包含）

#### TumblingWindow 滚动窗口

固定大小、无重叠，适合周期性统计。

```sql
SELECT AVG(temperature) as avg_temp FROM stream GROUP BY TumblingWindow('30s')
SELECT COUNT(*) as event_count FROM stream GROUP BY TumblingWindow('5m')
```

#### SlidingWindow 滑动窗口

固定大小、有重叠，适合平滑趋势分析。

```sql
-- 窗口大小5分钟，每1分钟滑动一次
SELECT AVG(temperature) as avg_temp FROM stream GROUP BY SlidingWindow('5m', '1m')
```

#### CountingWindow 计数窗口

基于数据条数触发。

```sql
SELECT AVG(temperature) as avg_temp FROM stream GROUP BY CountingWindow(100)
```

#### SessionWindow 会话窗口

基于数据活跃度，超时关闭。

```sql
SELECT user_id, COUNT(*) as actions FROM stream GROUP BY user_id, SessionWindow('5m')
```

### GROUP BY 分组

```sql
-- 按设备分组 + 时间窗口
SELECT deviceId, AVG(temperature) as avg_temp FROM stream GROUP BY deviceId, TumblingWindow('1m')

-- 多字段分组
SELECT deviceId, location, AVG(temperature) FROM stream GROUP BY deviceId, location, TumblingWindow('1m')

-- 纯窗口（不分组）
SELECT AVG(temperature) as avg_temp FROM stream GROUP BY TumblingWindow('1m')
```

### 窗口元数据函数

在聚合查询中可获取窗口时间信息：

```sql
SELECT deviceId,
       AVG(temperature) as avg_temp,
       window_start() as start_time,
       window_end() as end_time
FROM stream
GROUP BY deviceId, TumblingWindow('5s')
```

### 事件时间配置（WITH 子句）

默认使用处理时间（数据到达时间）。指定事件时间字段：

```sql
SELECT AVG(temperature) as avg_temp
FROM stream
GROUP BY TumblingWindow('5m')
WITH (TIMESTAMP='event_time')

-- 毫秒时间戳（默认）
WITH (TIMESTAMP='ts', TIMEUNIT='ms')

-- 秒级时间戳
WITH (TIMESTAMP='ts', TIMEUNIT='ss')

-- 容忍乱序 + 允许延迟 + 空闲超时
WITH (TIMESTAMP='event_time', MAXOUTOFORDERNESS='5s', ALLOWEDLATENESS='2s', IDLETIMEOUT='5s')
```

### 聚合函数列表

| 函数 | 说明 | 示例 |
|------|------|------|
| COUNT(*) | 计数 | `COUNT(*) as count` |
| SUM(field) | 求和 | `SUM(amount) as total` |
| AVG(field) | 平均值 | `AVG(temperature) as avg_temp` |
| MAX(field) | 最大值 | `MAX(temperature) as max_temp` |
| MIN(field) | 最小值 | `MIN(temperature) as min_temp` |
| STDDEV(field) | 总体标准差 | `STDDEV(temperature) as std` |
| STDDEVS(field) | 样本标准差 | `STDDEVS(temperature) as s_std` |
| VAR(field) | 总体方差 | `VAR(temperature) as variance` |
| VARS(field) | 样本方差 | `VARS(temperature) as s_variance` |
| MEDIAN(field) | 中位数 | `MEDIAN(temperature) as med` |
| PERCENTILE(field, p) | 百分位数 | `PERCENTILE(temperature, 0.95) as p95` |
| COLLECT(field) | 收集为数组 | `COLLECT(temperature) as temps` |
| FIRST_VALUE(field) | 首个值 | `FIRST_VALUE(status) as first` |
| LAST_VALUE(field) | 最后值 | `LAST_VALUE(status) as last` |
| MERGE_AGG(field) | 合并聚合 | `MERGE_AGG(status) as all_status` |
| DEDUPLICATE(field, bool) | 去重 | `DEDUPLICATE(temperature, true) as unique` |

### 常用内置函数

**数学**: `ABS(x)`, `ROUND(x, d)`, `FLOOR(x)`, `CEIL(x)`, `SQRT(x)`, `POWER(x, y)`

**字符串**: `CONCAT(s1, s2)`, `UPPER(s)`, `LOWER(s)`, `LENGTH(s)`, `SUBSTRING(s, start, len)`, `TRIM(s)`

**类型转换**: `CAST(expr AS STRING)`

### 时间单位

| 单位 | 值 | 说明 |
|------|-----|------|
| 纳秒 | `ns` | |
| 毫秒 | `ms` | 默认 |
| 秒 | `ss` | |
| 分钟 | `mi` | |
| 小时 | `hh` | |
| 天 | `dd` | |

### 不支持的特性

- JOIN、UNION、子查询
- INSERT/UPDATE/DELETE
- CREATE TABLE

## 规则链示例

### 示例1：设备温度实时聚合

每30秒按设备分组计算平均温度，超过阈值告警。

```json
{
  "ruleChain": {
    "id": "temp_aggregation",
    "name": "温度聚合监控",
    "root": true
  },
  "metadata": {
    "nodes": [
      {
        "id": "node_1",
        "type": "x/streamAggregator",
        "name": "温度聚合",
        "configuration": {
          "sql": "SELECT deviceId, AVG(temperature) as avg_temp, MAX(temperature) as max_temp, COUNT(*) as count FROM stream GROUP BY deviceId, TumblingWindow('30s')"
        },
        "additionalInfo": {"layoutX": 400, "layoutY": 300}
      },
      {
        "id": "node_2",
        "type": "jsTransform",
        "name": "告警判断",
        "configuration": {
          "jsScript": "var results = JSON.parse(msg.data || msg);\nif (Array.isArray(results)) {\n  results.forEach(function(r) {\n    r.alert = r.max_temp > 35;\n  });\n  msg = results;\n}\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType};"
        },
        "additionalInfo": {"layoutX": 600, "layoutY": 300}
      },
      {
        "id": "node_3",
        "type": "log",
        "name": "记录日志",
        "configuration": {
          "jsScript": "return '聚合结果: ' + JSON.stringify(msg);"
        },
        "additionalInfo": {"layoutX": 800, "layoutY": 300}
      }
    ],
    "connections": [
      {"fromId": "node_1", "toId": "node_2", "type": "window_event"},
      {"fromId": "node_2", "toId": "node_3", "type": "Success"}
    ]
  }
}
```

### 示例2：数据过滤转换

用 streamTransform 过滤无效数据并转换格式。

```json
{
  "ruleChain": {
    "id": "data_filter",
    "name": "数据过滤",
    "root": false
  },
  "metadata": {
    "nodes": [
      {
        "id": "node_1",
        "type": "x/streamTransform",
        "name": "字段选择",
        "configuration": {
          "sql": "SELECT deviceId, temperature, humidity, temperature * 1.8 + 32 as fahrenheit FROM stream WHERE temperature IS NOT NULL AND temperature > -50 AND temperature < 100"
        },
        "additionalInfo": {"layoutX": 400, "layoutY": 300}
      }
    ],
    "connections": []
  }
}
```

### 示例3：聚合 + 原始数据双路处理

```json
{
  "ruleChain": {
    "id": "dual_output",
    "name": "双路处理",
    "root": true
  },
  "metadata": {
    "nodes": [
      {
        "id": "node_1",
        "type": "x/streamAggregator",
        "name": "流聚合器",
        "configuration": {
          "sql": "SELECT deviceId, AVG(value) as avg_value FROM stream GROUP BY deviceId, TumblingWindow('1m')"
        },
        "additionalInfo": {"layoutX": 400, "layoutY": 300}
      },
      {
        "id": "node_2",
        "type": "restApiCall",
        "name": "发送聚合结果",
        "configuration": {
          "restEndpointUrlPattern": "http://api.example.com/analytics",
          "requestMethod": "POST"
        },
        "additionalInfo": {"layoutX": 600, "layoutY": 200}
      },
      {
        "id": "node_3",
        "type": "log",
        "name": "记录原始数据",
        "configuration": {
          "jsScript": "return '原始数据: ' + JSON.stringify(msg);"
        },
        "additionalInfo": {"layoutX": 600, "layoutY": 400}
      }
    ],
    "connections": [
      {"fromId": "node_1", "toId": "node_2", "type": "window_event"},
      {"fromId": "node_1", "toId": "node_3", "type": "Success"}
    ]
  }
}
```

### 示例4：嵌套字段聚合

```json
{
  "ruleChain": {
    "id": "nested_agg",
    "name": "嵌套字段聚合",
    "root": false
  },
  "metadata": {
    "nodes": [
      {
        "id": "node_1",
        "type": "x/streamAggregator",
        "name": "设备数据聚合",
        "configuration": {
          "sql": "SELECT device.info.type as device_type, AVG(sensor.data.temperature) as avg_temp, COUNT(*) as count FROM stream WHERE device.info.status = 'active' GROUP BY device.info.type, TumblingWindow('1m')"
        },
        "additionalInfo": {"layoutX": 400, "layoutY": 300}
      }
    ],
    "connections": []
  }
}
```

## 注意事项

- 输入数据必须是 JSON 类型，非 JSON 数据走 Failure 链
- streamAggregator 支持单条和数组输入，数组会被逐条添加到聚合流
- streamTransform 支持单条和数组输入，数组逐条转换后合并输出
- 聚合计算是异步的，不会阻塞原始数据流转
- 窗口触发时机由 StreamSQL 引擎自动决定
- FROM 子句中的名称是逻辑概念（通常用 `stream`），也可以自定义
- 时间窗口参数格式：`'30s'`(秒)、`'5m'`(分钟)、`'1h'`(小时)、`'1d'`(天)
- **SQL 字段直接使用字段名（如 `temperature`），不需要 `msg.` 前缀**（与 jsFilter/jsTransform 的 `msg.temperature` 不同！）
- 字段名区分大小写，与 JSON 数据中的 key 精确匹配
