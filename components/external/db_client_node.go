/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package external

import (
	"database/sql"
	"errors"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// 注册节点
func init() {
	Registry.Add(&DbClientNode{})
}

const (
	SELECT = "SELECT"
	INSERT = "INSERT"
	UPDATE = "UPDATE"
	DELETE = "DELETE"
	// EXEC 统一的执行类型，用于DDL和其他语句
	EXEC = "EXEC"
	// 自动检测
	AUTO = "AUTO"
)
const (
	rowsAffectedKey = "rowsAffected"
	lastInsertIdKey = "lastInsertId"
)

var (
	// 全局 SQL 校验器，默认使用 DefaultSqlValidator
	// Global SQL validator, defaults to DefaultSqlValidator
	globalSqlValidator SqlValidator = &DefaultSqlValidator{}
	// 保护全局 SQL 校验器的读写锁
	// Read-write lock to protect global SQL validator
	globalValidatorMutex sync.RWMutex
)

// SetGlobalSqlValidator 设置全局 SQL 校验器
// SetGlobalSqlValidator sets the global SQL validator
func SetGlobalSqlValidator(validator SqlValidator) {
	globalValidatorMutex.Lock()
	defer globalValidatorMutex.Unlock()
	if validator != nil {
		globalSqlValidator = validator
	}
}

// GetGlobalSqlValidator 获取全局 SQL 校验器
// GetGlobalSqlValidator gets the global SQL validator
func GetGlobalSqlValidator() SqlValidator {
	globalValidatorMutex.RLock()
	defer globalValidatorMutex.RUnlock()
	return globalSqlValidator
}

// DbClientNodeConfiguration 节点配置
type DbClientNodeConfiguration struct {
	// DriverName 数据库驱动名称，mysql或postgres
	DriverName string `json:"driverName"`
	// Dsn 数据库连接配置，参考sql.Open参数
	Dsn string `json:"dsn"`
	// PoolSize 连接池大小
	PoolSize int `json:"poolSize"`
	// OpType 操作类型配置，可选值：SELECT、INSERT、UPDATE、DELETE、EXEC
	// 如果不配置，则自动根据SQL语句的第一个单词判断
	OpType string `json:"opType"`
	// Sql SQL语句，v0.23.0之后不再支持运行时变量进行替换
	Sql string `json:"sql"`
	// Params SQL语句参数列表，可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Params []interface{} `json:"params"`
	// GetOne 是否只返回一条记录，true:返回结构不是数组结构，false：返回数据是数组结构
	GetOne bool `json:"getOne"`
}

// DbClientNode 数据库客户端节点，提供通用数据库连接和SQL执行能力
// DbClientNode provides universal database connectivity and SQL execution capabilities
//
// 支持的数据库：MySQL、PostgreSQL（内置），TDengine、SQL Server、Oracle、ClickHouse、SQLite等（需引入第三方驱动）
// 支持任何实现database/sql接口的驱动 - Supports any driver implementing database/sql interface
// 变量替换：${metadata.key}、${msg.key}
// 操作类型：SELECT、INSERT、UPDATE、DELETE、EXEC（可配置或自动检测）
// 连接管理：使用连接池和SharedNode模式共享连接
type DbClientNode struct {
	base.SharedNode[*sql.DB]
	ruleConfig types.Config
	//节点配置
	Config DbClientNodeConfiguration
	//操作类型 SELECT\UPDATE\INSERT\DELETE
	opType         string
	sqlTemplate    el.Template
	paramsTemplate []el.Template
	//sql是否有变量
	sqlHasVar bool
	//参数是否有变量
	paramsHasVar bool
	// SQL校验器，用于自定义SQL校验逻辑
	// SQL validator for custom SQL validation logic
	sqlValidator SqlValidator
}

// Type 返回组件类型
func (x *DbClientNode) Type() string {
	return "dbClient"
}

func (x *DbClientNode) New() types.Node {
	return &DbClientNode{Config: DbClientNodeConfiguration{
		Sql:        "select * from test",
		DriverName: "mysql",
		Dsn:        "root:root@tcp(127.0.0.1:3306)/test",
	}}
}

// SetSqlValidator 设置自定义SQL校验器
// SetSqlValidator sets custom SQL validator
func (x *DbClientNode) SetSqlValidator(validator SqlValidator) {
	x.sqlValidator = validator
}

// Init 初始化组件
func (x *DbClientNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	if x.Config.DriverName == "" {
		x.Config.DriverName = "mysql"
	}
	x.ruleConfig = ruleConfig
	// 初始化SQL校验器：优先使用实例级别的校验器，如果没有则使用全局校验器
	// Initialize SQL validator: prioritize instance-level validator, fallback to global validator
	if x.sqlValidator == nil {
		x.sqlValidator = GetGlobalSqlValidator()
	}

	x.opType = strings.TrimSpace(strings.ToUpper(x.Config.OpType))
	if x.opType != "" {
		if err = x.checkOpType(x.opType); err != nil {
			return err
		}
	}
	if !base.NodeUtils.IsInitNetResource(ruleConfig, configuration) {
		if x.Config.Sql == "" {
			return errors.New("sql can not empty")
		}
		//检查是否需要转换成$1风格占位符
		x.Config.Sql = str.ConvertDollarPlaceholder(x.Config.Sql, x.Config.DriverName)
		x.sqlTemplate, err = el.NewTemplate(x.Config.Sql)
		if err != nil {
			return err
		}
		if x.sqlTemplate.HasVar() {
			x.sqlHasVar = true
		} else {
			// 只有在没有配置OpType时才自动检测
			if x.opType == "" || x.opType == AUTO {
				x.opType = x.getOpType(x.Config.Sql)
			}
			if err = x.validateSQL(x.opType, x.Config.Sql); err != nil {
				return err
			}
		}
		//检查是参数否有变量
		for _, item := range x.Config.Params {
			if temp, err := el.NewTemplate(item); err != nil {
				return err
			} else {
				x.paramsTemplate = append(x.paramsTemplate, temp)
				if temp.HasVar() {
					x.paramsHasVar = true
				}
			}
		}
	}
	//初始化客户端
	return x.SharedNode.InitWithClose(ruleConfig, x.Type(), x.Config.Dsn, ruleConfig.NodeClientInitNow, func() (*sql.DB, error) {
		return x.initClient()
	}, func(client *sql.DB) error {
		// 清理回调函数
		return client.Close()
	})
}

// OnMsg 处理消息，执行SQL操作并处理结果
// OnMsg processes messages by executing SQL operations and handling results.
func (x *DbClientNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var data interface{}
	var err error
	var rowsAffected int64
	var lastInsertId int64
	var evn map[string]interface{}
	if x.sqlHasVar || x.paramsHasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	var sqlStr = x.Config.Sql
	if x.sqlHasVar {
		//转换sql变量
		sqlStr = x.sqlTemplate.ExecuteAsString(evn)
		sqlStr = str.ConvertDollarPlaceholder(sqlStr, x.Config.DriverName)
	}
	opType := x.opType
	if opType == "" || x.opType == AUTO {
		opType = x.getOpType(sqlStr)
		if err := x.validateSQL(x.opType, sqlStr); err != nil {
			ctx.TellFailure(msg, err)
			return
		}
	}
	var params []interface{}
	//转换参数变量
	for _, item := range x.paramsTemplate {
		param, err := item.Execute(evn)
		if err != nil {
			ctx.TellFailure(msg, err)
			return
		}
		params = append(params, param)
	}
	client, err := x.SharedNode.GetSafely()
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	switch opType {
	case SELECT:
		data, err = x.query(client, sqlStr, params, x.Config.GetOne)
	case UPDATE, DELETE:
		rowsAffected, err = x.execSQL(client, sqlStr, params, false)
	case INSERT:
		rowsAffected, lastInsertId, err = x.insert(client, sqlStr, params)
	default:
		// 对于EXEC或者未明确定义的SQL语句类型，使用exec方法进行处理
		rowsAffected, err = x.execSQL(client, sqlStr, params, true)
	}

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		switch opType {
		case SELECT:
			msg.SetData(str.ToString(data))
		case UPDATE, DELETE, EXEC:
			msg.Metadata.PutValue(rowsAffectedKey, str.ToString(rowsAffected))
		case INSERT:
			msg.Metadata.PutValue(rowsAffectedKey, str.ToString(rowsAffected))
			msg.Metadata.PutValue(lastInsertIdKey, str.ToString(lastInsertId))
		default:
			// 对于其他类型，设置影响行数
			msg.Metadata.PutValue(rowsAffectedKey, str.ToString(rowsAffected))
		}
		ctx.TellSuccess(msg)
	}
}

// query 查询数据并返回map或slice类型
func (x *DbClientNode) query(client *sql.DB, sqlStr string, params []interface{}, getOne bool) (interface{}, error) {
	rows, err := client.Query(sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// 获取列名和列类型
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// 创建一个固定大小的 map 和切片，用于存储每一行的数据
	row := make(map[string]interface{}, len(columns))
	values := make([]interface{}, len(columns))

	// 遍历每一列，初始化 interface{} 切片中的值
	for i := range columns {
		var v interface{}
		values[i] = &v
		row[columns[i]] = &v
	}

	// 创建一个空的 map 切片，用于存储最终结果
	result := make([]map[string]interface{}, 0)

	// 遍历结果集中的每一行数据
	for rows.Next() {
		// 调用 rows.Scan 方法，将结果存储在指针切片中
		err = rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		// 将当前行的 map 深拷贝到一个新的 map 中，避免后续循环覆盖数据
		m := make(map[string]interface{}, len(row))
		for k, v := range row {
			var temp = v
			// 如果值是 []byte 类型，转换成 string 类型
			if b1, ok := v.(*interface{}); ok {
				if b, ok := (*b1).([]byte); ok {
					temp = string(b)
				} else {
					temp = *b1
				}
			}
			m[k] = temp
		}
		// 将新的 map 追加到结果切片中
		result = append(result, m)
	}

	// 检查是否有错误发生
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if getOne {
		if len(result) > 0 {
			return result[0], nil // 如果只有一条记录，返回map类型
		} else {
			return nil, nil
		}
	} else {
		return result, nil // 否则返回slice类型
	}

}

// insert 插入数据并返回自增ID
func (x *DbClientNode) insert(client *sql.DB, sqlStr string, params []interface{}) (int64, int64, error) {
	result, err := client.Exec(sqlStr, params...)
	if err != nil {
		return 0, 0, err
	} else {
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, 0, err
		}

		lastInsertId, _ := result.LastInsertId()
		return rowsAffected, lastInsertId, nil
	}
}

// execSQL 执行SQL语句并返回影响行数
// ignorRowsAffectedError: 是否忽略RowsAffected错误（用于DDL语句）
func (x *DbClientNode) execSQL(client *sql.DB, sqlStr string, params []interface{}, ignoreRowsAffectedError bool) (int64, error) {
	result, err := client.Exec(sqlStr, params...)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		if ignoreRowsAffectedError {
			// 某些DDL语句可能不支持RowsAffected，这种情况下返回0而不是错误
			return 0, nil
		}
		return 0, err
	}
	return rowsAffected, nil
}

// Destroy 销毁组件
func (x *DbClientNode) Destroy() {
	_ = x.SharedNode.Close()
}

// initClient 初始化客户端
func (x *DbClientNode) initClient() (*sql.DB, error) {
	client, err := sql.Open(x.Config.DriverName, x.Config.Dsn)
	if err == nil {
		client.SetMaxOpenConns(x.Config.PoolSize)
		client.SetMaxIdleConns(x.Config.PoolSize / 2)
		err = client.Ping()
	}
	return client, err
}

// getOpType 获取SQL语句的操作类型
// 支持识别 WITH AS 开头的 ETL 表达式和各种 DDL 语句
// 如果配置了OpType，则优先使用配置的类型
func (x *DbClientNode) getOpType(sql string) string {
	// 如果配置了OpType，则优先使用配置的类型
	if x.Config.OpType != "" {
		return x.Config.OpType
	}

	if sql == "" {
		return ""
	}
	words := strings.Fields(sql)
	if len(words) == 0 {
		return ""
	}

	return strings.ToUpper(words[0])

}

// checkOpType 检查配置的SQL操作类型是否支持
func (x *DbClientNode) checkOpType(opType string) error {
	switch opType {
	case SELECT, INSERT, UPDATE, DELETE, EXEC, AUTO:
		return nil
	default:
		return errors.New("unsupported opTypet: " + opType)
	}
}

// SqlValidator SQL校验器接口，用于自定义SQL语句校验逻辑
// SqlValidator interface for custom SQL statement validation logic
type SqlValidator interface {
	// ValidateSQL 校验SQL语句
	// ValidateSQL validates SQL statement
	// opType: 操作类型 (SELECT, INSERT, UPDATE, DELETE, EXEC)
	// sql: SQL语句
	// 返回错误信息，如果校验通过则返回nil
	ValidateSQL(config types.Config, opType, sql string) error
}

// DefaultSqlValidator 默认SQL校验器实现
// DefaultSqlValidator default SQL validator implementation
type DefaultSqlValidator struct{}

// ValidateSQL 默认的SQL校验实现
// ValidateSQL default SQL validation implementation
func (v *DefaultSqlValidator) ValidateSQL(config types.Config, opType, sql string) error {
	return nil
}

// validateSQL 使用配置的 SQL 校验器验证操作类型和 SQL 语句
// validateSQL validates operation type and SQL statement using configured SQL validator
func (x *DbClientNode) validateSQL(opType, sql string) error {
	if x.sqlValidator != nil {
		return x.sqlValidator.ValidateSQL(x.RuleConfig, opType, sql)
	}
	return nil
}
