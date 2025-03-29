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
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strings"
)

// 注册节点
func init() {
	Registry.Add(&DbClientNode{})
}

const (
	SELECT = "SELECT"
	INSERT = "INSERT"
	DELETE = "DELETE"
	UPDATE = "UPDATE"
)
const (
	rowsAffectedKey = "rowsAffected"
	lastInsertIdKey = "lastInsertId"
)

// DbClientNodeConfiguration 节点配置
type DbClientNodeConfiguration struct {
	// DriverName 数据库驱动名称，mysql或postgres
	DriverName string
	// Dsn 数据库连接配置，参考sql.Open参数
	Dsn string
	// PoolSize 连接池大小
	PoolSize int
	// Sql SQL语句，v0.23.0之后不再支持运行时变量进行替换
	Sql string
	// Params SQL语句参数列表，可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Params []interface{}
	// GetOne 是否只返回一条记录，true:返回结构不是数组结构，false：返回数据是数组结构
	GetOne bool
}

type DbClientNode struct {
	base.SharedNode[*sql.DB]
	//节点配置
	Config DbClientNodeConfiguration
	client *sql.DB
	//操作类型 SELECT\UPDATE\INSERT\DELETE
	opType string
	//sqlTemplate    str.Template
	//paramsTemplate []str.Template
	//sql是否有变量
	sqlHasVar bool
	//参数是否有变量
	paramsHasVar   bool
	paramsTemplate []el.Template
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

// Init 初始化组件
func (x *DbClientNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	if x.Config.DriverName == "" {
		x.Config.DriverName = "mysql"
	}

	if !base.NodeUtils.IsInitNetResource(ruleConfig, configuration) {
		if x.Config.Sql == "" {
			return errors.New("sql can not empty")
		}
		//检查是否需要转换成$1风格占位符
		x.Config.Sql = str.ConvertDollarPlaceholder(x.Config.Sql, x.Config.DriverName)
		if str.CheckHasVar(x.Config.Sql) {
			x.sqlHasVar = true
		}
		if !x.sqlHasVar {
			x.opType = x.getOpType(x.Config.Sql)
			if err = x.checkOpType(x.opType, x.Config.Sql); err != nil {
				return err
			}
		}
		//检查是参数否有变量
		for _, item := range x.Config.Params {

			if temp, err := el.NewTemplate(item); err != nil {
				return err
			} else {
				x.paramsTemplate = append(x.paramsTemplate, temp)
				if !temp.IsNotVar() {
					x.paramsHasVar = true
				}
			}
		}

	}
	//初始化客户端
	return x.SharedNode.Init(ruleConfig, x.Type(), x.Config.Dsn, ruleConfig.NodeClientInitNow, func() (*sql.DB, error) {
		return x.initClient()
	})
}

// OnMsg 处理消息
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
		sqlStr = str.ExecuteTemplate(x.Config.Sql, evn)
		sqlStr = str.ConvertDollarPlaceholder(sqlStr, x.Config.DriverName)
	}
	opType := x.opType
	if opType == "" {
		opType = x.getOpType(sqlStr)
		if err := x.checkOpType(opType, sqlStr); err != nil {
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
	client, err := x.SharedNode.Get()
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	switch opType {
	case SELECT:
		data, err = x.query(client, sqlStr, params, x.Config.GetOne)
	case UPDATE:
		rowsAffected, err = x.update(client, sqlStr, params)
	case INSERT:
		rowsAffected, lastInsertId, err = x.insert(client, sqlStr, params)
	case DELETE:
		rowsAffected, err = x.delete(client, sqlStr, params)
	default:
		err = fmt.Errorf("unsupported sql statement: %s", sqlStr)
	}

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		switch opType {
		case SELECT:
			msg.Data = str.ToString(data)
		case UPDATE, DELETE:
			msg.Metadata.PutValue(rowsAffectedKey, str.ToString(rowsAffected))
		case INSERT:
			msg.Metadata.PutValue(rowsAffectedKey, str.ToString(rowsAffected))
			msg.Metadata.PutValue(lastInsertIdKey, str.ToString(lastInsertId))
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

// update 修改数据并返回影响行数
func (x *DbClientNode) update(client *sql.DB, sqlStr string, params []interface{}) (int64, error) {
	result, err := client.Exec(sqlStr, params...)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
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

// delete 删除数据并返回影响行数
func (x *DbClientNode) delete(client *sql.DB, sqlStr string, params []interface{}) (int64, error) {
	result, err := client.Exec(sqlStr, params...)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}

// Destroy 销毁组件
func (x *DbClientNode) Destroy() {
	if x.client != nil {
		_ = x.client.Close()
	}
}

// initClient 初始化客户端
func (x *DbClientNode) initClient() (*sql.DB, error) {
	if x.client != nil {
		return x.client, nil
	} else {
		x.Locker.Lock()
		defer x.Locker.Unlock()
		if x.client != nil {
			return x.client, nil
		}
		var err error
		x.client, err = sql.Open(x.Config.DriverName, x.Config.Dsn)
		if err == nil {
			x.client.SetMaxOpenConns(x.Config.PoolSize)
			x.client.SetMaxIdleConns(x.Config.PoolSize / 2)
			err = x.client.Ping()
			return x.client, err
		} else {
			return nil, err
		}
	}
}

func (x *DbClientNode) getOpType(sql string) string {
	if sql == "" {
		return ""
	}
	words := strings.Fields(sql)
	return strings.ToUpper(words[0])
}

func (x *DbClientNode) checkOpType(opType string, sql string) error {
	//检查操作类型是否支持
	switch opType {
	case SELECT, UPDATE, INSERT, DELETE:
		return nil
	default:
		return fmt.Errorf("unsupported sql statement: %s", sql)
	}
}
