---
name: streamsql
description: 当需要创建实时数据聚合/窗口统计、流式过滤转换、变化检测（CDC）、生命周期累计、元数据富化（流-表 JOIN）、CEP 模式识别（MATCH_RECOGNIZE）等规则链时使用，或当使用 x/streamAggregator 和 x/streamTransform 组件时使用。包含 SQL 语法、窗口、分析函数、JOIN、CEP 和配置示例
---

# StreamSQL 流式计算组件

RuleGo 通过 `x/streamAggregator` 和 `x/streamTransform` 两个组件提供流式 SQL 处理能力。

## 组件选择

| 需求                                | 组件                   | 说明                                  |
| --------------------------------- | -------------------- | ----------------------------------- |
| 过滤、字段选择、计算转换、变化检测、累计              | `x/streamTransform`  | 逐条同步处理，不能含聚合函数/GROUP BY             |
| 聚合统计（AVG/COUNT/SUM/MAX/MIN）+ 窗口   | `x/streamAggregator` | 必须含 GROUP BY + 窗口函数                 |
| CEP 模式识别（事件序列匹配 MATCH\_RECOGNIZE） | `x/streamAggregator` | 见下文 [CEP](#cep-模式识别match_recognize) |
| 窗口内做变化检测/回溯/累计                    | `x/streamAggregator` | 分析函数对窗口输出求值                         |

**用错组件会初始化失败**：aggregator 的 SQL 必须是聚合或 CEP（MATCH\_RECOGNIZE），transform 不能含聚合函数/MATCH\_RECOGNIZE。分析函数（`lag`/`changed_col`/`acc_*` 等）不是聚合函数，两边都可用——不带窗口时进 transform，带窗口时进 aggregator。

## x/streamAggregator 流聚合器

数据流：输入 → 加入聚合流（原始数据走 `Success` 继续）→ 窗口触发 → 聚合结果走 `stream_event`。

```json
{
  "type": "x/streamAggregator",
  "configuration": {
    "sql": "SELECT deviceId, AVG(temperature) as avg_temp, COUNT(*) as count FROM stream GROUP BY deviceId, TumblingWindow('30s')"
  }
}
```

连接类型：`Success`（原始数据）、`Failure`（非 JSON/SQL 错误）、`stream_event`（**聚合结果，关键**）。

结果消息：`msgType=stream_event`，`metadata.queryType`=`aggregation`(聚合)或 `cep`(MATCH\_RECOGNIZE)、`resultType`=`window_triggered`/`pattern_matched`，`msg.data` 是结果 JSON 数组，如 `[{"deviceId":"d1","avg_temp":25.5,"count":10}]`。可同时从 `Success`（原始）和 `stream_event`（结果）双路输出分别处理。

## x/streamTransform 流转换器

```json
{
  "type": "x/streamTransform",
  "configuration": {
    "sql": "SELECT deviceId, temperature * 1.8 + 32 as fahrenheit FROM stream WHERE temperature > 20"
  }
}
```

连接类型：`Success`（转换成功且符合 WHERE，结果替换 `msg.data`，`match=true`）、`False`（WHERE 不命中 / `changed_cols` 无变化，**携带原始数据**，`match=false`）、`Failure`（错误）。

数组输入逐条转换后合并输出，metadata 额外含 `originalCount`/`transformedCount`/`failedCount`。

## 分析函数（跨事件状态）

逐条事件求值、状态跨事件保留，走 transform 的同步路径；窗口内则对窗口输出求值、状态跨窗口保留。

| 函数                                                     | 用途                            |
| ------------------------------------------------------ | ----------------------------- |
| `lag(f [,offset [,default [,ignoreNull]]])`            | 前 N 行的值（CDC 回溯）               |
| `latest(f [,default])`                                 | 最新非空值                         |
| `had_changed(ignoreNull, f...)`                        | 与上次比是否变化（首次视为变化）              |
| `changed_col(ignoreNull, f)`                           | 变化的列值（未变返回 nil）               |
| `changed_cols(prefix, ignoreNull, f...)`               | 多列变化值，输出 `prefix+列名`，仅 SELECT |
| `acc_sum`/`acc_max`/`acc_min`/`acc_count`/`acc_avg(f)` | 生命周期累计（不随窗口重置）                |

```sql
-- CDC：电流从低跨过 300A
SELECT current, deviceId FROM stream
WHERE current > 300 AND lag(current) OVER (PARTITION BY deviceId) < 300
-- 仅发送变化字段
SELECT changed_cols("c_", true, temperature, humidity) FROM stream
-- 窗口均值变化才输出（进 aggregator）
SELECT changed_cols("t", true, avg(temperature)) FROM stream GROUP BY CountingWindow(2)
```

- `OVER (PARTITION BY ... WHEN ...)`：分区 / 条件更新；不支持 `ORDER BY`/`ROWS` frame。
- `acc_count(v, startExpr, resetExpr)`：条件累计（开始点 / 重置点）。
- 条件表达式用比较运算符 `>`/`<`/`==`；分析函数参数里 `=` 不作相等判断，字符串相等用 `==`。
- `changed_cols`/`changed_col` 唯一输出且无变化时返回 nil → 走 transform 的 **False** 链（预期的事件压缩，接丢弃/忽略即可）。

## CEP 模式识别（MATCH\_RECOGNIZE）

`x/streamAggregator` 支持 `MATCH_RECOGNIZE`，语法对齐 **Flink / SQL 标准**的行模式识别（标准语法详见 [Flink MATCH\_RECOGNIZE 文档](https://nightlies.apache.org/flink/flink-docs-stable/docs/dev/table/sql/queries/match_recognize/)）。匹配命中后结果走 `stream_event`（与聚合同出口，`metadata.queryType=cep`、`resultType=pattern_matched`）。

```sql
SELECT * FROM stream
MATCH_RECOGNIZE (
  PARTITION BY deviceId                         -- 可选：分区键
  ORDER BY ts                                   -- 必填：排序字段
  MEASURES MATCH_NUMBER() AS mn, A.v AS peak    -- 输出列；可含聚合 COUNT(*)/SUM(A.v)/AVG(v)/MAX(v)，及 FIRST(v)/LAST(v)/CLASSIFIER()
  ONE ROW PER MATCH                             -- 默认；或 ALL ROWS PER MATCH
  AFTER MATCH SKIP PAST LAST ROW                -- 可选：PAST LAST ROW(默认)/TO NEXT ROW/TO FIRST x/TO LAST x
  PATTERN (A{3} B?)                             -- 拼接 AB、交替 A|B、量词 {n}/{n,m}/{n,}/*/+/?、分组 ()、PERMUTE(A,B)
  WITHIN '1h'                                   -- 可选：时间窗（默认 1h）
  DEFINE A AS v > 50, B AS v < 10               -- 符号条件；未在 DEFINE 出现的符号恒为真
)
```

**已支持**：`PARTITION BY`、`ORDER BY`（必填）、`MEASURES`（含聚合 + `MATCH_NUMBER()`）、`ONE`/`ALL ROWS PER MATCH`、`AFTER MATCH SKIP` 全部策略、`PATTERN`（拼接 / 交替 / 量词 / 分组 / `PERMUTE`）、`SUBSET`、`WITHIN`、`DEFINE`。

**不支持 / 注意**：

- `{- ... -}` 排除（absence）不支持，会报错。
- 仅单流（`FROM stream`）；不支持多流 MATCH\_RECOGNIZE。
- 量词默认贪婪；懒惰量词（`*?`/`+?`）亦支持，但同一模式内勿混用贪婪与懒惰。
- ⚠️ MATCH\_RECOGNIZE 子句本身的语法错误会被外层 Parse 容错吞掉，导致查询**静默降级为非 CEP**（不报错）。`DEFINE`/`MEASURES` 里的表达式语法错会在 Execute 期暴露。写完建议实跑验证或用 `IsCEPQuery()` 确认走了 CEP 路径。

## SQL 语法

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

- **SELECT**：字段 / `*` / 计算字段 / 别名（AS 可省）/ `DISTINCT` / 聚合函数。
- **嵌套字段**：点号、`[index]`（支持负数）、`['key']`，如 `device.info.name`、`items[0].name`、`config['host']`。可用于 WHERE/GROUP BY/HAVING。
- **CASE**：`CASE WHEN cond THEN v ... ELSE d END`，可在聚合内部做条件计数 `SUM(CASE WHEN ... THEN 1 ELSE 0 END)`。
- **WHERE**：比较 / `AND OR NOT` / `BETWEEN` / `IN` / `IS [NOT] NULL` / `LIKE`（`%`/`_`），均可用嵌套字段。
- **HAVING**：过滤聚合结果。**引用 SELECT 别名**（`HAVING avg_temp > 25`），不能复述聚合函数。

### 窗口函数（aggregator 必含一个）

```sql
GROUP BY TumblingWindow('30s')                                        -- 滚动：固定大小、无重叠
GROUP BY SlidingWindow('5m', '1m')                                    -- 滑动：大小 + 滑动间隔
GROUP BY CountingWindow(100)                                          -- 计数：按条数触发
GROUP BY user_id, SessionWindow('5m')                                 -- 会话：超时关闭
GROUP BY deviceId, GLOBAL WINDOW TRIGGER WHEN COUNT(*) >= 3           -- 全局：谓词触发后清空
```

窗口元数据函数：`window_start()`、`window_end()`。

### 事件时间（WITH 子句）

默认处理时间。指定事件时间字段与乱序/延迟容忍：

```sql
WITH (TIMESTAMP='event_time', TIMEUNIT='ms', MAXOUTOFORDERNESS='5s', ALLOWEDLATENESS='2s', IDLETIMEOUT='5s')
```

`TIMEUNIT`：`ns`/`ms`(默认)/`ss`/`mi`/`hh`/`dd`。

### 聚合与内置函数

聚合：`COUNT(*)` `SUM` `AVG` `MAX` `MIN` `STDDEV` `STDDEVS` `VAR` `VARS` `MEDIAN` `PERCENTILE(f,p)` `COLLECT` `FIRST_VALUE` `LAST_VALUE` `MERGE_AGG` `DEDUPLICATE(f,bool)`。

内置：数学 `ABS/ROUND/FLOOR/CEIL/SQRT/POWER`、字符串 `CONCAT/UPPER/LOWER/LENGTH/SUBSTRING/TRIM`、`CAST(expr AS STRING)`。

## 元数据表与流-表 JOIN

节点配置 `tables` 注册元数据表，SQL 里 `JOIN` 富化流行（transform 逐行富化、aggregator 富化后聚合）。**streamsql 1.0.0+**。

```json
"tables": [
  {"name": "meta", "source": "file", "path": "/etc/rulego/device_meta.json", "format": "json", "refresh": "30s"}
]
```

| 字段                  | 说明                                                               |
| ------------------- | ---------------------------------------------------------------- |
| `name`              | 表名；**必须出现在 SQL 的 JOIN 里**（`JOIN meta m` → `name:"meta"`），否则初始化失败 |
| `source`            | `file`/`http`（UI）；后端另支持 `inline`（行内 `rows`，不刷新）                  |
| `path`              | 文件路径（file）或 GET URL（http）                                        |
| `format`            | `json`/`csv`（默认 json）                                            |
| `refresh`           | 刷新间隔；空 = file/http 默认 1 小时；inline 不刷新                            |
| `headers`/`timeout` | 仅 http                                                           |

JOIN 仅支持等值 ON（复合键用 `AND`），表侧列按别名命名空间返回（`m.location`）。刷新失败保留旧快照。

```sql
SELECT deviceId, m.location FROM stream s LEFT JOIN meta m ON s.deviceId = m.deviceId WHERE temperature > 30
```

## 规则链示例

每 30 秒按设备分组计算平均温度，超阈值告警：

```json
{
  "ruleChain": {"id": "temp_aggregation", "name": "温度聚合监控", "root": true},
  "metadata": {
    "nodes": [
      {"id": "node_1", "type": "x/streamAggregator", "name": "温度聚合",
       "configuration": {"sql": "SELECT deviceId, AVG(temperature) as avg_temp, MAX(temperature) as max_temp, COUNT(*) as count FROM stream GROUP BY deviceId, TumblingWindow('30s')"}},
      {"id": "node_2", "type": "jsTransform", "name": "告警判断",
       "configuration": {"jsScript": "var r = JSON.parse(msg.data || msg); if (Array.isArray(r)) { r.forEach(function(x){ x.alert = x.max_temp > 35; }); msg = r; } return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
      {"id": "node_3", "type": "log", "name": "记录", "configuration": {"jsScript": "return JSON.stringify(msg);"}}
    ],
    "connections": [
      {"fromId": "node_1", "toId": "node_2", "type": "stream_event"},
      {"fromId": "node_2", "toId": "node_3", "type": "Success"}
    ]
  }
}
```

## 注意事项（高频踩坑）

- **输入必须是 JSON 类型**（`msg.DataType == JSON`）。非 JSON **静默走 Failure、无日志、整条链无输出**——极易踩坑。
  - jsTransform 造数据喂 stream 节点时，返回**必须带** **`dataType:'JSON'`**（否则继承上游 DataType，上游若是 TEXT 就静默失败）：
    ```js
    return {'msg': data, 'metadata': metadata, 'msgType': msgType, 'dataType': 'JSON'};
    ```
- **SQL 字段直接用字段名**（`temperature`），**不要** **`msg.`** **前缀**（与 jsFilter/jsTransform 的 `msg.temperature` 不同）。
- 字段名区分大小写，与 JSON key 精确匹配。
- FROM 后的名称是逻辑概念（通常用 `stream`）。
- 时间窗口参数：`'30s'`/`'5m'`/`'1h'`/`'1d'`。
- 不支持：流-流 JOIN（WITHIN 区间）、UNION、子查询、INSERT/UPDATE/DELETE、CREATE TABLE。

