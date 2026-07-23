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
	"reflect"
	"regexp"
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

// Register the node
func init() {
	Registry.Add(&DbClientNode{})
}

const (
	SELECT = "SELECT"
	INSERT = "INSERT"
	UPDATE = "UPDATE"
	DELETE = "DELETE"
	// EXEC is a unified execution type used for DDL and other statements
	EXEC = "EXEC"
	// Automatic detection
	AUTO = "AUTO"
)
const (
	rowsAffectedKey = "rowsAffected"
	lastInsertIdKey = "lastInsertId"
)

var (
	// Global SQL checker, default uses DefaultSqlValidator
	// Global SQL validator, defaults to DefaultSqlValidator
	globalSqlValidator SqlValidator = &DefaultSqlValidator{}
	// Protects the read/write lock of the global SQL checktester
	// Read-write lock to protect global SQL validator
	globalValidatorMutex sync.RWMutex
	// Precompiled placeholder matches regular expressions
	// Pre-compiled placeholder matching regex
	placeholderRegex = regexp.MustCompile(`\?`)
)

// SetGlobalSqlValidator sets up the global SQL validator
// SetGlobalSqlValidator sets the global SQL validator
func SetGlobalSqlValidator(validator SqlValidator) {
	globalValidatorMutex.Lock()
	defer globalValidatorMutex.Unlock()
	if validator != nil {
		globalSqlValidator = validator
	}
}

// GetGlobalSqlValidator obtains the global SQL checker
// GetGlobalSqlValidator gets the global SQL validator
func GetGlobalSqlValidator() SqlValidator {
	globalValidatorMutex.RLock()
	defer globalValidatorMutex.RUnlock()
	return globalSqlValidator
}

// DbClientNodeConfiguration
type DbClientNodeConfiguration struct {
	DriverName string        `json:"driverName" label:"Driver" desc:"Database driver, e.g. mysql, postgres, sqlite3" required:"true" ref:"shared"`
	Dsn        string        `json:"dsn" label:"DSN" desc:"Database connection string, e.g. user:password@tcp(host:port)/dbname" required:"true" ref:"primary"`
	PoolSize   int           `json:"poolSize" label:"Pool Size" desc:"Database connection pool size" ref:"shared"`
	OpType     string        `json:"opType" label:"Op Type" desc:"Operation type: SELECT, INSERT, UPDATE, DELETE" required:"true"`
	Sql        string        `json:"sql" label:"SQL" desc:"SQL statement, supports ${metadata.key} and ${msg.key} substitution" required:"true"`
	Params     []interface{} `json:"params" label:"Params" desc:"SQL parameter list, supports ${metadata.key} substitution"`
	GetOne     bool          `json:"getOne" label:"Get One" desc:"true=return only first record, false=return all records"`
}

// DbClientNode is a database client node that provides general database connections and SQL execution capabilities
// DbClientNode provides universal database connectivity and SQL execution capabilities
//
// Supported databases: MySQL, PostgreSQL (built-in), TDengine, SQL Server, Oracle, ClickHouse, SQLite, etc. (requires third-party drivers)
// Supports any driver implementing database/sql interface
// Variable replacement: ${metadata.key}, ${msg.key}
// Operation types: SELECT, INSERT, UPDATE, DELETE, EXEC (configurable or automatic detection)
// Connection Management: Uses connection pools and SharedNode patterns to share connections
type DbClientNode struct {
	base.SharedNode[*sql.DB]
	ruleConfig types.Config
	//Node configuration
	Config DbClientNodeConfiguration
	//Operation type: SELECT\UPDATE\INSERT\DELETE
	opType         string
	sqlTemplate    el.Template
	paramsTemplate []el.Template
	//Does SQL have variables?
	sqlHasVar bool
	//Whether the parameters have variables
	paramsHasVar bool
	// SQL checker, used for customizing SQL validation logic
	// SQL validator for custom SQL validation logic
	sqlValidator SqlValidator
}

// Type returns the component type
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

// SetSqlValidator sets up a custom SQL validator
// SetSqlValidator sets custom SQL validator
func (x *DbClientNode) SetSqlValidator(validator SqlValidator) {
	x.sqlValidator = validator
}

// Init initializes the component
func (x *DbClientNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	if x.Config.DriverName == "" {
		x.Config.DriverName = "mysql"
	}
	x.ruleConfig = ruleConfig
	// Initialize SQL verister: prioritize instance-level validators; if not available, use global validators
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
		//Check if it needs to be converted to a $1 style placeholder
		x.Config.Sql = str.ConvertDollarPlaceholder(x.Config.Sql, x.Config.DriverName)
		x.sqlTemplate, err = el.NewTemplate(x.Config.Sql)
		if err != nil {
			return err
		}
		if x.sqlTemplate.HasVar() {
			x.sqlHasVar = true
		} else {
			// It only detects automatically when OpType is not configured
			if x.opType == "" || x.opType == AUTO {
				x.opType = x.getOpType(x.Config.Sql)
			}
			if err = x.validateSQL(x.opType, x.Config.Sql); err != nil {
				return err
			}
		}
		//Check whether there are variables in the parameter
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
	//Initialize the client
	return x.SharedNode.InitWithClose(ruleConfig, x.Type(), x.Config.Dsn, ruleConfig.NodeClientInitNow, func() (*sql.DB, error) {
		return x.initClient()
	}, func(client *sql.DB) error {
		// Cleanup callback function
		return client.Close()
	})
}

// OnMsg handles messages, executes SQL operations, and processes results
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
		//Convert SQL variables
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
	//Convert parameter variables
	for _, item := range x.paramsTemplate {
		param, err := item.Execute(evn)
		if err != nil {
			ctx.TellFailure(msg, err)
			return
		}
		params = append(params, param)
	}

	// Expand the slicing parameters in the IN clause
	// Expand slice parameters in IN clause
	sqlStr, params = expandInClause(sqlStr, params, x.Config.DriverName)
	// PostgreSQL needs to convert placeholder formats
	// PostgreSQL requires placeholder format conversion
	if x.Config.DriverName == "postgres" {
		sqlStr = str.ConvertDollarPlaceholder(sqlStr, x.Config.DriverName)
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
		// For EXEC or undefined SQL statement types, use the exec method to handle them
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
			// For other types, set the number of affected rows
			msg.Metadata.PutValue(rowsAffectedKey, str.ToString(rowsAffected))
		}
		ctx.TellSuccess(msg)
	}
}

// query query data and returns a map or slice type
func (x *DbClientNode) query(client *sql.DB, sqlStr string, params []interface{}, getOne bool) (interface{}, error) {
	rows, err := client.Query(sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Retrieve column names and column types
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Create a fixed-size map and slice to store each row's data
	row := make(map[string]interface{}, len(columns))
	values := make([]interface{}, len(columns))

	// Traverse each column and initialize the value in the interface{} slice
	for i := range columns {
		var v interface{}
		values[i] = &v
		row[columns[i]] = &v
	}

	// Create an empty map slice to store the final result
	result := make([]map[string]interface{}, 0)

	// Traverse every row of data in the result set
	for rows.Next() {
		// Call rows.Scan method, storing results in pointer slices
		err = rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		// Copy the map from the current row deep into a new map to avoid future loops overwriting the data
		m := make(map[string]interface{}, len(row))
		for k, v := range row {
			var temp = v
			// If the value is of type []byte, convert it to string type
			if b1, ok := v.(*interface{}); ok {
				if b, ok := (*b1).([]byte); ok {
					temp = string(b)
				} else {
					temp = *b1
				}
			}
			m[k] = temp
		}
		// Append a new map to the resulting slice
		result = append(result, m)
	}

	// Check for errors
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if getOne {
		if len(result) > 0 {
			return result[0], nil // If there is only one record, return the map type
		} else {
			return nil, nil
		}
	} else {
		return result, nil // Otherwise, it returns the slice type
	}

}

// insert Insert data and return the increment ID
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

// execSQL executes SQL statements and returns the number of affected rows
// ignorRowsAffectedError: Whether to ignore the RowsAffected error (used in DDL statements)
func (x *DbClientNode) execSQL(client *sql.DB, sqlStr string, params []interface{}, ignoreRowsAffectedError bool) (int64, error) {
	result, err := client.Exec(sqlStr, params...)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		if ignoreRowsAffectedError {
			// Some DDL statements may not support RowsAffected, in which case they return 0 instead of an error
			return 0, nil
		}
		return 0, err
	}
	return rowsAffected, nil
}

// Destroy releases component resources
func (x *DbClientNode) Destroy() {
	_ = x.SharedNode.Close()
}

// initClient initializes the client
func (x *DbClientNode) initClient() (*sql.DB, error) {
	client, err := sql.Open(x.Config.DriverName, x.Config.Dsn)
	if err == nil {
		client.SetMaxOpenConns(x.Config.PoolSize)
		client.SetMaxIdleConns(x.Config.PoolSize / 2)
		err = client.Ping()
	}
	return client, err
}

// getOpType to get the type of operation in the SQL statement
// Supports recognizing ETL expressions starting with WITH AS and various DDL statements
// If OpType is configured, the configured type is used first
func (x *DbClientNode) getOpType(sql string) string {
	// If OpType is configured, the configured type is used first
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

// checkOpType to check whether the configured SQL operation type is supported
func (x *DbClientNode) checkOpType(opType string) error {
	switch opType {
	case SELECT, INSERT, UPDATE, DELETE, EXEC, AUTO:
		return nil
	default:
		return errors.New("unsupported opTypet: " + opType)
	}
}

// SqlValidator SQL checker interface, used for customizing SQL statement validation logic
// SqlValidator interface for custom SQL statement validation logic
type SqlValidator interface {
	// ValidateSQL checks SQL statements
	// ValidateSQL validates SQL statement
	// opType: Operation type (SELECT, INSERT, UPDATE, DELETE, EXEC)
	// sql: SQL statement
	// Returns an error message; if the checkcheck passes, returns nil
	ValidateSQL(config types.Config, opType, sql string) error
}

// DefaultSqlValidator is implemented as the default SQL validator
// DefaultSqlValidator default SQL validator implementation
type DefaultSqlValidator struct{}

// ValidateSQL is the default SQL validation implementation
// ValidateSQL default SQL validation implementation
func (v *DefaultSqlValidator) ValidateSQL(config types.Config, opType, sql string) error {
	return nil
}

// validateSQL uses configured SQL checkers to validate operation types and SQL statements
// validateSQL validates operation type and SQL statement using configured SQL validator
func (x *DbClientNode) validateSQL(opType, sql string) error {
	if x.sqlValidator != nil {
		return x.sqlValidator.ValidateSQL(x.RuleConfig, opType, sql)
	}
	return nil
}

// expandInClause expands slice/array parameters in SQL IN clauses.
// Example: "SELECT * FROM table WHERE id IN (?)" with params []int{1,2,3}
// becomes "SELECT * FROM table WHERE id IN (?, ?, ?)" with params 1, 2, 3.
func expandInClause(sqlStr string, params []interface{}, _ string) (string, []interface{}) {
	if len(params) == 0 {
		return sqlStr, params
	}

	placeholderMatches := placeholderRegex.FindAllStringIndex(sqlStr, -1)
	if len(placeholderMatches) == 0 {
		return sqlStr, params
	}

	// First pass: pre-calculate final parameter count
	totalParams := 0
	hasSlice := false
	for i, param := range params {
		if i >= len(placeholderMatches) {
			break
		}
		if sliceLen := getSliceLen(param); sliceLen >= 0 {
			hasSlice = true
			if sliceLen > 0 {
				totalParams += sliceLen
			}
		} else {
			totalParams++
		}
	}

	if !hasSlice {
		return sqlStr, params
	}

	// Second pass: perform expansion
	var builder strings.Builder
	builder.Grow(len(sqlStr) + totalParams*3)

	newParams := make([]interface{}, 0, totalParams)
	lastEnd := 0

	for i, param := range params {
		if i >= len(placeholderMatches) {
			break
		}

		pos := placeholderMatches[i]
		builder.WriteString(sqlStr[lastEnd:pos[0]])
		lastEnd = pos[1]

		sliceLen := getSliceLen(param)
		if sliceLen < 0 {
			builder.WriteByte('?')
			newParams = append(newParams, param)
		} else if sliceLen == 0 {
			builder.WriteString("NULL")
		} else {
			expandSliceToBuilder(&builder, param, sliceLen, &newParams)
		}
	}

	builder.WriteString(sqlStr[lastEnd:])

	return builder.String(), newParams
}

// getSliceLen returns the slice length, or -1 if not a slice/array.
func getSliceLen(param interface{}) int {
	if param == nil {
		return -1
	}

	switch v := param.(type) {
	case []int:
		return len(v)
	case []int64:
		return len(v)
	case []int32:
		return len(v)
	case []string:
		return len(v)
	case []float64:
		return len(v)
	case []float32:
		return len(v)
	case []bool:
		return len(v)
	case []interface{}:
		return len(v)
	}

	v := reflect.ValueOf(param)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return v.Len()
	case reflect.Interface:
		elem := v.Elem()
		if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array {
			return elem.Len()
		}
	}
	return -1
}

// expandSliceToBuilder expands a slice into placeholders and appends elements to params.
func expandSliceToBuilder(builder *strings.Builder, param interface{}, sliceLen int, params *[]interface{}) {
	v := reflect.ValueOf(param)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	for i := 0; i < sliceLen; i++ {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteByte('?')
		*params = append(*params, v.Index(i).Interface())
	}
}

// Desc returns the component description
func (x *DbClientNode) Desc() string {
	return "Database client for SQL databases (MySQL, PostgreSQL, etc.). opType auto-detected or manual. params support ${metadata.key} and ${msg.key}. IN clause expands slices. Routes to Success/Failure"
}
