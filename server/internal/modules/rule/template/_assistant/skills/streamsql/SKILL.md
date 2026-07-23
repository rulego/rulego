---
name: streamsql
description: Used when creating rule chains such as real-time data aggregation/window statistics, stream filtering transformation, variation detection (CDC), lifecycle accumulation, metadata enrichment (stream-table JOIN), CEP pattern recognition (MATCH_RECOGNIZE), or when using x/streamAggregator and x/streamTransform components. Includes SQL syntax, windows, parsing functions, JOIN, CEP, and configuration examples
---

# StreamSQL Stream computing component

RuleGo Streaming SQL processing power is provided through two components: `x/streamAggregator` and `x/streamTransform`.

## Component Selection

| Requirement                                | Component                   | Note                                  |
| --------------------------------- | -------------------- | ----------------------------------- |
| Filtering, field selection, calculation conversion, change detection, cumulative              | `x/streamTransform`  | Synchronized processing of each entry cannot include aggregation functions /GROUP BY             |
| Aggregate statistics (AVG/COUNT/SUM/MAX/MIN) + window   | `x/streamAggregator` | Must include GROUP BY + window function                 |
| CEP Pattern recognition (event sequence matching MATCH\_RECOGNIZE) | `x/streamAggregator` | See below [CEP](#cep-模式识别match_recognize) |
| Change detection/lookback/accumulation within a window | `x/streamAggregator` | Analytic functions evaluate window output |

**Using the wrong component will cause initialization failure**: aggregator SQL must be aggregated or CEP (MATCH\_RECOGNIZE), transform cannot contain aggregation functions /MATCH\_RECOGNIZE. Analysis functions (`lag` / `changed_col` / `acc_*`, etc.) are not aggregate functions; both sides can be used—without a window, enter transform; with a window, enter aggregator.

## x/streamAggregator Flow aggregator

Data stream: Enter → to join the aggregate stream (original data goes `Success` continues), → window triggers → aggregate results go `stream_event`.

```json
{
  "type": "x/streamAggregator",
  "configuration": {
    "sql": "SELECT deviceId, AVG(temperature) as avg_temp, COUNT(*) as count FROM stream GROUP BY deviceId, TumblingWindow('30s')"
  }
}
```

Connection types: `Success` (raw data), `Failure` (non-JSON/SQL error), `stream_event` (**aggregated results, critical**).

Result message: `msgType=stream_event`, `metadata.queryType` = `aggregation` (aggregate) or `cep` (MATCH\_RECOGNIZE), `resultType` = `window_triggered` / `pattern_matched`, `msg.data` is the result JSON array, for example `[{"deviceId":"d1","avg_temp":25.5,"count":10}]`. Can simultaneously process dual outputs from `Success` (primitive) and `stream_event` (result) separately.

## x/streamTransform Stream Converter

```json
{
  "type": "x/streamTransform",
  "configuration": {
    "sql": "SELECT deviceId, temperature * 1.8 + 32 as fahrenheit FROM stream WHERE temperature > 20"
  }
}
```

Connection type: `Success` (Conversion successful and meets WHERE, result replacement `msg.data`, `match=true`), `False` (WHERE misses / `changed_cols` unchanged, **carries original data**, `match=false`), `Failure` (incorrect).

After converting array inputs one by one, merging outputs, metadata additionally includes `originalCount` / `transformedCount` / `failedCount`.

## Analysis Function (Cross-Event State)

Evaluate each event individually, retain states across events, and follow a transform synchronization path; Within the window, output is evaluated, and status is retained across windows.

| Function                                                     | Purpose                            |
| ------------------------------------------------------ | ----------------------------- |
| `lag(f [,offset [,default [,ignoreNull]]])`            | Values of the first N rows (CDC unwind)               |
| `latest(f [,default])`                                 | Latest non-null value                         |
| `had_changed(ignoreNull, f.)`                        | Is there a change from the previous comparison (initially considered a change)              |
| `changed_col(ignoreNull, f)`                           | Variable column value (unchanged returns nil)               |
| `changed_cols(prefix, ignoreNull, f.)`               | Multiple columns of variation values, output `prefix+列名`, only SELECT |
| `acc_sum` / `acc_max` / `acc_min` / `acc_count` / `acc_avg(f)` | Lifecycle accumulation (does not reset with window)                |

```sql
-- CDC: The current crosses from the low to 300A
SELECT current, deviceId FROM stream
WHERE current > 300 AND lag(current) OVER (PARTITION BY deviceId) < 300
-- Only send the change field
SELECT changed_cols("c_", true, temperature, humidity) FROM stream
-- Output only after the window average changes (aggregator)
SELECT changed_cols("t", true, avg(temperature)) FROM stream GROUP BY CountingWindow(2)
```

- `OVER (PARTITION BY. WHEN.)`: Zoning / Condition updates; `ORDER BY` / `ROWS` frame is not supported.
- `acc_count(v, startExpr, resetExpr)`: Condition accumulation (start point / reset point).
- Conditional expressions use comparison operators `>` / `<` / `==`; In the analysis function parameter, `=` does not perform equality checks; string equality is determined by `==`.
- `changed_cols` / `changed_col` Returns nil → **False** chain of transform when the only output is unchanged (expected event compression, just discard/ignore).

## CEP Pattern Recognition (MATCH\_RECOGNIZE)

`x/streamAggregator` Supports `MATCH_RECOGNIZE`, syntax alignment **Flink/SQL standard** line pattern recognition (for standard syntax, see [Flink MATCH\_RECOGNIZE documentation](https://nightlies.apache.org/flink/flink-docs-stable/docs/dev/table/sql/queries/match_recognize/)). After matching successfully, the result is `stream_event` (with Ju contract export, `metadata.queryType=cep`, `resultType=pattern_matched`).

```sql
SELECT * FROM stream
MATCH_RECOGNIZE (
  PARTITION BY deviceId                         -- Optional: Partition key
  ORDER BY ts                                   -- Required: Sort field
  MEASURES MATCH_NUMBER() AS mn, A.v AS peak    -- Output columns; may contain COUNT(*)/SUM(A.v)/AVG(v)/MAX(v) aggregations and FIRST(v)/LAST(v)/CLASSIFIER()
  ONE ROW PER MATCH                             -- Default; Or ALL ROWS PER MATCH
  AFTER MATCH SKIP PAST LAST ROW                -- Optional: PAST LAST ROW(default)/TO NEXT ROW/TO FIRST x/TO LAST x
  PATTERN (A{3} B?)                             -- Concatenation AB, alternation A|B, quantifiers {n}/{n,m}/{n,}/*/+/? , grouping (), and PERMUTE(A,B)
  WITHIN '1h'                                   -- Optional: Time window (default 1h)
  DEFINE A AS v > 50, B AS v < 10               -- Symbol condition; Symbols not appearing in DEFINE are always true
)
```

**Supported**: `PARTITION BY`, `ORDER BY` (required), `MEASURES` (including aggregation + `MATCH_NUMBER()`), `ONE` / `ALL ROWS PER MATCH`, `AFTER MATCH SKIP` all strategies, `PATTERN` (concatenation / alternation / quantifier / grouping / `PERMUTE`)、 `SUBSET` 、 `WITHIN` 、 `DEFINE`.

**Not supported / Note**:

- `{-. -}` Exclude (absence) is not supported and will cause errors.
- Single-stream only (`FROM stream`); Does not support multi-stream MATCH\_RECOGNIZE.
- The quantifier defaults to greed; Laziness (`*?` / `+?`) is also supported, but do not mix greed and laziness within the same mode.
- ⚠️ MATCH syntax errors in the \_RECOGNIZE clause itself are swallowed by the outer Parse fault tolerance, causing the query **to be silently downgraded to non-CEP** (no errors). Grammatical errors in expressions in `DEFINE`/`MEASURES` will be exposed in Execute. After writing the suggestion, I suggested running it for verification or using `IsCEPQuery()` to confirm that I had taken the CEP path.

## SQL Grammar

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

- **SELECT**: Field / `*` / Compute field / Alias (AS can be omitted) / `DISTINCT` / Aggregate function.
- **Nested fields**: dot number, `[index]` (supports negative numbers), `['key']`, such as `device.info.name`, `items[0].name`, `config['host']`. Can be used for WHERE/GROUP BY/HAVING.
- **CASE**: `CASE WHEN cond THEN v. ELSE d END`, can perform conditional counting `SUM(CASE WHEN. THEN 1 ELSE 0 END)` inside the aggregate.
- **WHERE**: Compare / `AND OR NOT` / `BETWEEN` / `IN` / `IS [NOT] NULL` / `LIKE` (`%` / `_`), all can be nested fields.
- **HAVING**: Filter and aggregate results. **Reference SELECT alias** (`HAVING avg_temp > 25`), cannot restate the aggregation function.

### Window Function (aggregator must include one)

```sql
GROUP BY TumblingWindow('30s')                                        -- Rolling: fixed size, no overlap
GROUP BY SlidingWindow('5m', '1m')                                    -- Sliding: size + sliding interval
GROUP BY CountingWindow(100)                                          -- Count: Triggered by the number of bars
GROUP BY user_id, SessionWindow('5m')                                 -- Session: Closes after timeout
GROUP BY deviceId, GLOBAL WINDOW TRIGGER WHEN COUNT(*) >= 3           -- Global: Clears after the predicate is triggered
```

Window metadata functions: `window_start()`, `window_end()`.

### Event Time (WITH Clause)

Default processing time. Specify event time fields and tolerance for out-of-order/delay:

```sql
WITH (TIMESTAMP='event_time', TIMEUNIT='ms', MAXOUTOFORDERNESS='5s', ALLOWEDLATENESS='2s', IDLETIMEOUT='5s')
```

`TIMEUNIT`: `ns` / `ms` (default) / `ss` / `mi` / `hh` / `dd`.

### Aggregation and built-in functions

Aggregate: `COUNT(*)` `SUM` `AVG` `MAX` `MIN` `STDDEV` `STDDEVS` `VAR` `VARS` `MEDIAN` `PERCENTILE(f,p)` `COLLECT` `FIRST_VALUE` `LAST_VALUE` `MERGE_AGG` `DEDUPLICATE(f,bool)`.

Built-in: Mathematics `ABS/ROUND/FLOOR/CEIL/SQRT/POWER`, String `CONCAT/UPPER/LOWER/LENGTH/SUBSTRING/TRIM`, `CAST(expr AS STRING)`.

## Metadata Tables and Stream-Table JOIN

Node configuration `tables` register metadata tables, SQL `JOIN` enrich popular (transform enrich line by line, aggregator aggregate after enrichment). **streamsql 1.0.0+**.

```json
"tables": [
  {"name": "meta", "source": "file", "path": "/etc/rulego/device_meta.json", "format": "json", "refresh": "30s"}
]
```

| Field                  | Note                                                               |
| ------------------- | ---------------------------------------------------------------- |
| `name`              | Appearance; **must appear in the JOIN of the SQL** (`JOIN meta m` → `name:"meta"`), otherwise the initialization fails |
| `source`            | `file` / `http`  (UI);  The backend also supports `inline` (inline`rows`, no refresh)                  |
| `path`              | File path (file) or GET URL (http)                                        |
| `format`            | `json` / `csv` (default json)                                            |
| `refresh`           | Refresh intervals; Empty = file/http default 1 hour; inline Do not refresh                            |
| `headers` / `timeout` | Only http                                                           |

JOIN Only supports equivalent ON (use `AND` for composite keys), and the side column returns by aliased namespace (`m.location`). Refresh fails to retain the old snapshot.

```sql
SELECT deviceId, m.location FROM stream s LEFT JOIN meta m ON s.deviceId = m.deviceId WHERE temperature > 30
```

## Example of a rule chain

Average temperature calculated by device group every 30 seconds, threshold warning is triggered:

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

## Precautions (Frequent pitfalls)

- **input must be JSON type** (`msg. DataType == JSON`). Non-JSON **silent Failure, no logs, no output throughout the chain** — very easy to fall into traps.
  - jsTransform When creating data to feed stream nodes, the return **must include** **`dataType:'JSON'`** (otherwise, it inherits upstream DataType; if upstream is TEXT, it will mute and fail):
    ```js
    return {'msg': data, 'metadata': metadata, 'msgType': msgType, 'dataType': 'JSON'};
    ```
- **SQL field directly uses the field name** (`temperature`), **do not** **`msg.`** **the prefix** (or jsFilter/jsTransform). `msg.temperature` different).
- Field names are case-sensitive and match JSON key exactly.
- Names after FROM are logical concepts (usually `stream`).
- Time window parameters: `'30s'` / `'5m'` / `'1h'` / `'1d'`.
- Not supported: stream-stream JOIN (WITHIN interval), UNION, subqueries, INSERT/UPDATE/DELETE, CREATE TABLE.
