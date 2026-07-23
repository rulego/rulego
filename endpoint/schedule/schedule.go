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

// Package schedule provides a scheduled task endpoint implementation for the RuleGo framework.
// It enables creating scheduled tasks that execute rule chains or components at specified times
// using cron expressions, providing time-based automation capabilities.
//
// Package schedule provides timed task endpoints for the RuleGo framework.
// It supports creating scheduled tasks and using cron expressions to execute rule chains or components at specified times,
// Provides time-based automation features.
//
// # Key Features
//
// • Cron Expression Support: Full cron syntax with second-level precision
// • Rule Chain Integration: Direct integration with RuleGo rule chains
// • Multiple Schedules: Support for multiple concurrent scheduled tasks
// • Dynamic Management: Runtime addition and removal of scheduled tasks
// • Built-in Expressions: Predefined expressions for common scheduling patterns
//
// # Architecture
//
// The Schedule endpoint follows a time-driven execution model:
// Schedule endpoints follow a time-driven execution model:
//
// 1. Cron Engine: Manages time-based task scheduling. Cron Engine: Manages time-based task scheduling
// 2. Task Execution: Triggers rule chains at scheduled times
// 3. Message Generation: Creates RuleMsg for scheduled executions
// 4. Rule Processing: Executes business logic through rule chains
//
// Cron Expression Format / Cron Expression Format:
//
// The router's 'from' field supports the following cron expression format:
// The router's 'from' field supports the following cron expression formats:
//
// Field name   | Mandatory? | Allowed values  | Allowed special characters
// Field name | Is it necessary? | Allowed values | Special characters are allowed
// ----------   | ---------- | --------------  | --------------------------
// Seconds      | Yes        | 0-59            | * / , -
// Seconds | is | 0-59 | * /, -
// Minutes      | Yes        | 0-59            | * / , -
// Minutes | is | 0-59 | * /, -
// Hours        | Yes        | 0-23            | * / , -
// Hour | is | 0-23 | * /, -
// Day of month | Yes        | 1-31            | * / , - ?
// The sun in the middle of the month | is | 1-31 | * /, -?
// Month        | Yes        | 1-12 or JAN-DEC | * / , -
// Month | is | 1-12 or JAN-DEC | * /, -
// Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?
// Midweek Day | is | 0-6 or SUN-SAT | * /, -?
//
// # Built-in Special Expressions
//
// Entry                  | Description                                | Equivalent To
// Entry | Description | Equivalent to
// -----                  | -----------                                | -------------
// @yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 0 1 1 *
// @yearly (or @annually) | Runs once a year, at midnight on January 1st 0 0 0 1 1 *
// @monthly               | Run once a month, midnight, first of month | 0 0 0 1 * *
// @monthly | Runs once a month, at midnight on the 1st of each month 0 0 0 1 * *
// @weekly                | Run once a week, midnight between Sat/Sun  | 0 0 0 * * 0
// @weekly | Runs once a week, midnight between Saturday and Sunday 0 0 0 * * 0
// @daily (or @midnight)  | Run once a day, midnight                   | 0 0 0 * * *
// @daily (or @midnight) | Runs once daily, at midnight | 0 0 0 * * *
// @hourly                | Run once an hour, beginning of hour        | 0 0 * * * *
// @hourly | Runs once per hour, starting on the hour | 0 0 * * * *
//
// # Initialization Methods
//
// The Schedule endpoint supports two initialization approaches:
// The Schedule endpoint supports two initialization methods:
//
// 1. Registry-based Initialization
//
//	import "github.com/rulego/rulego/endpoint"
//
//	// Create endpoint through registry
//	Create endpoints through the registry
//	endpoint, err := endpoint.Registry.New(schedule.Type, ruleConfig, types.Configuration{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add scheduled tasks
//	Add scheduled tasks
//	router1 := endpoint.NewRouter().From("0 * * * * *").To("chain:minuteTask")
//	endpoint.AddRouter(router1)
//
//	router2 := endpoint.NewRouter().From("0 30 2 * * *").To("chain:dailyBackup")
//	endpoint.AddRouter(router2)
//
//	endpoint.Start()
//
// 2. Dynamic DSL Initialization
//
//	dslConfig := `{
//	  "id": "schedule-endpoint",
//	  "type": "endpoint/schedule",
//	  "name": "Task Scheduler",
//	  "configuration": {},
//	  "routers": [
//	    {
//	      "id": "minute-task",
//	      "from": {
//	        "path": "0 * * * * *"
//	      },
//	      "to": {
//	        "path": "chain:minuteTask"
//	      }
//	    },
//	    {
//	      "id": "daily-backup",
//	      "from": {
//	        "path": "0 30 2 * * *"
//	      },
//	      "to": {
//	        "path": "chain:dailyBackup"
//	      }
//	    },
//	    {
//	      "id": "monitoring",
//	      "from": {
//	        "path": "*/15 * * * * *"
//	      },
//	      "to": {
//	        "path": "chain:monitoring"
//	      }
//	    }
//	  ]
//	}`
//
//	// Create endpoint from DSL
//	Create endpoints from DSL
//	endpoint, err := endpoint.NewFromDsl([]byte(dslConfig))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	endpoint.Start()
//
// Cron Expression Examples / Cron Expression Examples:
//
//	// Every minute
//	Every minute
//	router := endpoint.NewRouter().From("0 * * * * *").To("chain:minuteTask")
//
//	// Every day at 2:30 AM
//	Every day at 2:30 a.m
//	router := endpoint.NewRouter().From("0 30 2 * * *").To("chain:dailyBackup")
//
//	// Every 15 seconds
//	Every 15 seconds
//	router := endpoint.NewRouter().From("*/15 * * * * *").To("chain:monitoring")
//
//	// Business hours (9 AM to 5 PM, weekdays)
//	Working hours (weekdays 9:00 AM to 5:00 PM)
//	router := endpoint.NewRouter().From("0 0 9-17 * * 1-5").To("chain:businessHours")
package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"sync"

	"github.com/gofrs/uuid/v5"
	"github.com/robfig/cron/v3"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/runtime"
	"github.com/rulego/rulego/utils/str"
)

// Type defines the component type identifier for the Schedule endpoint.
// This identifier is used for component registration and DSL configuration.
// Type defines the component type identifier for the Schedule endpoint.
// This identifier is used for component registration and DSL configuration.
const Type = types.EndpointTypePrefix + "schedule"

// Endpoint is an alias for Schedule to provide consistent naming with other endpoints.
// This allows users to reference the component using the standard Endpoint name.
// Endpoint is an alias for Schedule, providing a naming consistency with other endpoints.
// This allows users to reference components using standard Endpoint names.
type Endpoint = Schedule

// RequestMessage represents a scheduled task execution request in the RuleGo processing pipeline.
// Unlike other endpoints that receive external messages, the Schedule endpoint generates
// internal messages when scheduled tasks are triggered.
//
// RequestMessage means RuleGo handles scheduled tasks in the pipeline to execute requests.
// Unlike other endpoints that receive external messages, the Schedule endpoint generates internal messages when scheduled tasks are triggered.
//
// Key Features
// • Time-Triggered Generation: Messages are generated based on cron schedules
// • Minimal Payload: Contains minimal data as the trigger is time-based
// • Metadata Integration: Seamlessly integrates with RuleGo's metadata system
// • JSON Data Type: Uses JSON format for consistent processing
//
// Message Content
// The message body is typically empty as the trigger event is the schedule itself.
// Additional context can be provided through metadata or rule chain configuration.
// The message body is usually empty because the trigger event is the dispatch itself.
// Additional context can be provided through metadata or rule chain configuration.
type RequestMessage struct {
	//HTTP-style headers map storing schedule-specific information
	headers textproto.MIMEHeader
	//Message body data, usually empty
	body []byte
	//Converted rule message, cached to avoid re-conversion
	msg *types.RuleMsg
	//Error information during processing
	err error
	//Message type
	msgType types.DataType
	//Metadata for passing additional context information
	metadata map[string]string
}

func (r *RequestMessage) Body() []byte {
	return r.body
}

func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

// From does not provide source access
func (r *RequestMessage) From() string {
	return ""
}

// GetParam does not provide acquisition parameters
func (r *RequestMessage) GetParam(key string) string {
	return ""
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		dataType := types.JSON
		if r.msgType != "" {
			dataType = r.msgType
		}
		metadata := types.NewMetadata()
		// If there is incoming metadata, set it in the message
		if r.metadata != nil {
			for k, v := range r.metadata {
				metadata.PutValue(k, v)
			}
		}
		ruleMsg := types.NewMsg(0, r.From(), dataType, metadata, string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode does not provide a status code
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *RequestMessage) SetError(err error) {
	r.err = err
}

func (r *RequestMessage) GetError() error {
	return r.err
}

// ResponseMessage
type ResponseMessage struct {
	headers textproto.MIMEHeader
	body    []byte
	msg     *types.RuleMsg
	err     error
	mu      sync.RWMutex
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

// From does not provide source access
func (r *ResponseMessage) From() string {
	return ""
}

// GetParam does not provide acquisition parameters
func (r *ResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

// SetStatusCode does not provide a status code
func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.body = body
}

func (r *ResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

// Schedule represents a scheduled task endpoint implementation for the RuleGo framework.
// It provides time-based automation capabilities by executing rule chains or components
// at specified intervals using cron expressions.
//
// Schedule represents the implementation of scheduled task endpoints in the RuleGo framework.
// It provides time-based automation by using cron expressions to execute rule chains or components at specified intervals.
//
// # Architecture
//
// The Schedule endpoint uses a cron-based scheduling system:
// Schedule endpoints use a cron-based scheduling system:
//
// 1. Cron Engine: Manages all scheduled tasks and timing. Cron Engine: Manages all scheduled tasks and timing
// 2. Task Registry: Stores and manages active scheduled tasks
// 3. Execution Engine: Triggers rule chain execution at scheduled times
// 4. Lifecycle Management: Handles task creation, modification, and cleanup
//
// # Key Features
//
// • Precise Scheduling: Second-level precision with full cron syntax
// • Multiple Tasks: Support for unlimited concurrent scheduled tasks
// • Dynamic Management: Runtime task addition and removal
// • Unique Identification: Each endpoint instance has a unique identifier
// • Automatic Cleanup: Proper resource cleanup on endpoint destruction
//
// # Task Management
//
// Tasks are managed through the following lifecycle:
// Tasks are managed through the following lifecycles:
//
// 1. Creation: AddRouter() creates and registers a new scheduled task
// 2. Execution: Tasks execute automatically based on cron schedule
// 3. Removal: RemoveRouter() stops and removes scheduled tasks
// 4. Cleanup: Destroy() cleans up all resources and stops the cron engine
//
// Cron Engine Configuration / Cron Engine Configuration:
//
// The cron engine is configured with second-level precision:
// CRON engine configured for second-level accuracy:
//
// • WithSeconds(): Enables second-level scheduling precision
// • Thread-safe: Multiple goroutines can safely interact with tasks
// • Efficient: Optimized for minimal resource overhead
//
// # Error Handling
//
// • Invalid cron expressions are detected during task registration
// • Task execution errors are isolated and don't affect other tasks
// • Comprehensive logging for debugging and monitoring
//
// # Performance Considerations
//
// • Lightweight cron engine with minimal memory footprint
// • Efficient task scheduling algorithms
// • Non-blocking task execution
// • Automatic garbage collection of completed tasks
type Schedule struct {
	// id is a unique identifier for this Schedule endpoint instance
	// id is the unique identifier of this Schedule endpoint instance
	id string

	// BaseEndpoint provides common endpoint functionality
	// BaseEndpoint provides universal endpoint functionality
	impl.BaseEndpoint

	// RuleConfig provides access to the rule engine configuration
	// RuleConfig provides access to the rule engine configuration
	RuleConfig types.Config

	// cron is the underlying cron engine instance that manages all scheduled tasks
	// cron is the underlying cron engine instance that manages all scheduled tasks
	cron *cron.Cron
}

// New creates a new Schedule Endpoint instance
func New(ruleConfig types.Config) *Schedule {
	uuId, _ := uuid.NewV4()
	return &Schedule{RuleConfig: ruleConfig, cron: cron.New(cron.WithSeconds()), id: uuId.String()}
}

// Type returns the component type
func (schedule *Schedule) Type() string {
	return Type
}

// Category returns the component category
func (schedule *Schedule) Category() string {
	return "endpoint"
}

// Def returns the component definition. Schedule requires a 6-field cron expression in from.path.
func (schedule *Schedule) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "Scheduled task endpoint that triggers rule chains at specified times. from.path must be a full 6-field cron expression (Seconds Minutes Hours Day-of-month Month Day-of-week), e.g. `*/5 * * * * *` runs every 5 seconds. A bare `*` is NOT a valid cron expression.",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Cron",
					Desc:     "6-field cron expression, e.g. */5 * * * * *",
					Required: true,
				},
			},
			Params: &types.ComponentFormField{
				Name:     "params",
				Type:     "array",
				Desc:     "message body + data type emitted on each scheduled tick (no external input), e.g. [\"{}\",\"JSON\"]; do not leave null",
				Required: true,
			},
		},
	}
}

func (schedule *Schedule) New() types.Node {
	uuId, _ := uuid.NewV4()
	return &Schedule{cron: cron.New(cron.WithSeconds()), id: uuId.String()}
}

// Init initializes the component
func (schedule *Schedule) Init(ruleConfig types.Config, configuration types.Configuration) error {
	schedule.RuleConfig = ruleConfig
	return nil
}

// Destroy releases resources
func (schedule *Schedule) Destroy() {
	_ = schedule.Close()
}

func (schedule *Schedule) Close() error {
	if schedule.cron != nil {
		schedule.cron.Stop()
		schedule.cron = nil
	}
	schedule.BaseEndpoint.Destroy()
	return nil
}

func (schedule *Schedule) Id() string {
	return schedule.id
}

func (schedule *Schedule) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	}
	if router.GetFrom() == nil {
		return "", errors.New("from can not nil")
	}
	if len(params) > 0 {
		router.SetParams(params...)
	}
	if schedule.cron == nil {
		schedule.cron = cron.New(cron.WithSeconds())
	}
	//Get the cron expression
	from := router.GetFrom().ToString()
	//Add tasks
	id, err := schedule.cron.AddFunc(from, func() {
		schedule.handler(router)
	})
	idStr := strconv.Itoa(int(id))
	router.SetId(idStr)
	//Returns the task ID, used to clear the task
	return idStr, err
}

func (schedule *Schedule) RemoveRouter(routeId string, params ...interface{}) error {
	entryID, err := strconv.Atoi(routeId)
	if err != nil {
		return fmt.Errorf("%s it is an illegal routing id", routeId)
	}
	if schedule.cron != nil {
		schedule.cron.Remove(cron.EntryID(entryID))
	}
	return nil
}

func (schedule *Schedule) Start() error {
	if schedule.cron == nil {
		return errors.New("cron has not been initialized yet")
	}
	schedule.cron.Start()
	return nil
}

func (schedule *Schedule) Printf(format string, v ...interface{}) {
	if schedule.RuleConfig.Logger != nil {
		schedule.RuleConfig.Logger.Printf(format, v...)
	}
}

// Handling scheduled tasks
func (schedule *Schedule) handler(router endpoint.Router) {
	defer func() {
		//Capture anomalies
		if e := recover(); e != nil {
			schedule.Printf("schedule endpoint handler err :\n%v", runtime.Stack())
		}
	}()
	var body []byte
	var msgType = types.JSON
	var metadata map[string]string
	params := router.GetParams()
	if len(params) > 0 {
		if params[0] != nil {
			body = []byte(str.ToString(params[0]))
		}
	}
	if len(params) > 1 {
		if params[1] != nil {
			switch v := params[1].(type) {
			case types.DataType:
				msgType = v
			case string:
				msgType = types.DataType(v)
			default:
				msgType = types.DataType(str.ToString(params[1]))
			}
		}
	}
	// Handle the third parameter as metadata
	if len(params) > 2 && params[2] != nil {
		switch v := params[2].(type) {
		case map[string]string:
			metadata = v
		case map[string]interface{}:
			metadata = make(map[string]string)
			for key, val := range v {
				metadata[key] = str.ToString(val)
			}
		case string:
			if v != "" {
				// Try parsing it as JSON
				var m map[string]string
				if err := json.Unmarshal([]byte(v), &m); err == nil {
					metadata = m
				}
			}
		}
	}
	exchange := &endpoint.Exchange{
		In:  &RequestMessage{body: body, msgType: msgType, metadata: metadata},
		Out: &ResponseMessage{}}

	schedule.DoProcess(context.Background(), router, exchange)
}
