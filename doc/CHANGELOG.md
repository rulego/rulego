# CHANGELOG

# [v0.36.0] 2026/06/01
### rulego-core
- feat(engine): Added Stream relationship types, supporting synchronous execution streams (#63)
- feat(endpoint): Added SSE streaming push support, ScopedMessage proxy, and NetClient/WsClient endpoint components
- feat(endpoint): rest endpoint Increases Restart events
- feat(node): Supports recursive variable replacement and rule chain environment variable injection
- feat(schedule): Supports custom message bodies, types, and metadata parameters
- feat(template): Adds built-in functions for include and fileExists
- feat(maps): Supports struct field access and backkey lookup
- feat(logger): Refactoring Logger interface, supports log level
- feat: Added MCP tool provider interface to support AI tool integration; Added GetUdf/GetUdfs methods and AiTool script types
- feat(dsl): Enhanced node reference extraction and field support
- feat(components): Adds while node components, supporting mode configuration
- feat(components): cacheGet node adds whenKeyNotFound configuration; cachePut Routing components to Failure chain when outputMode=2 and cache key does not exist (#104)
- feat: Improved component configuration form metadata (JSON labels, label, ref labels, RouterForm, and shared node support)
- feat: supports per-message debugMode and skip-tell-next controls; Precompute link level debugMode
- feat: Change the default field name tag to 'json'; Supports nested structure flattening in component form configuration
- feat: GetEnv Supports access to global vars variables
- feat(dbClient): Adds IN clause slice parameter expansion
- feat: NodePool.LoadFromRuleChain Skip loaded entries
- fix: Fixed JoinNode timeout failures, LCA algorithms causing deadlocks, premature callback triggers, and metadata not merged
- fix: Fixed duplicate writes on endpoint nodes
- fix: Fixed global variables not working in router fields of endpoint components (#93)
- fix: Fixed metadataToHeaders processors not working under fasthttp (#95)
- fix: Fixed support for the ${ } placeholder in inclusive/switch nodes
- fix: Ensure ruleChainPool is passed to rootRuleContext; Propagate parent ruleConfig to the dynamic component subrule engine
- fix: Fixed concurrent branch data contention, websocket concurrent write, and result slicing contention
- refactor(cache): Refactor Cache.Get signature, return (interface{}, error)
- refactor: Do not serialize function bodies
- perf: Optimized file operation functions
- chore: upgrade expr-lang/expr to v1.17.8
### rulego-components
- feat(nsq): Implements NSQD multi-node polling release and fault tolerance mechanism
- feat: Added python node components
- feat: Added file operation node components (read, write, delete, list)
- feat: fasthttp endpoint Increase restart events
- feat: Refined ref tags configured with SharedNode components, supporting shared connections
- opt: mongodb client components support ObjectId automatic conversion
- refactor(lua): Refactoring lua components to the transform/lua directory and adapting Cache.Get signature changes
- fix: Fixed ctx.Response data contention in streaming mode
- chore: Upgrade dependencies
### rulego-components-ai
- feat: transform into a full-function AI Agent framework, adding ReAct Agent cycles, Tool Agent, and Agent plants
- feat: Added 10 AOP aspects (logging, sessions, visualization, and more) to intercept the agent lifecycle
- feat: Added unified tool abstraction layer: bash, read, write, edit, browseruse, mcp, skill, and other built-in tools
- feat: Added MCP Bidirectional protocol support (client + server)
- feat: Added intent recognition modules and multi-dimensional session management systems
- feat: Added OpenAI stream processors, Embedding clients, and similarity calculations
- feat: Added dynamic model selection, model retrying, and multimodal vision support
- feat: Added Skill skill system to support AI tool calls orchestrated through rule chains
### rulego-components-iot
- feat: Added serial communication components
- feat(modbus): Adds runtime configuration persistence and hierarchical logging; Improved component form field label configuration
- fix(modbus): Fixed reconnection failures, Shock group issues, incorrect multi-register address stepping, and connection management issues in shared node pool mode
- chore: Upgrade dependencies
### rulego-server
- feat: server Upgraded from examples/server to architecture-level standalone modules, deployed independently as RuleGo automation workflow platforms
- feat: Added support for file operations and serial communication components
- feat(ci): Added 32-bit Linux build goals and server modules CI workflow
- fix: Keep the rule chain enabled after failed startup (#97 #98)
- fix: Fixed 32-bit system compilation failures and null pointer issues
- fix: Fixed the missing installed field in the component market listing API
- chore: Upgrade dependencies; Mark old examples/server as obsolete

# [v0.35.0] 2025/12/18
### rulego-core
- feat(components): join component supports passing errors to the next node
- feat(components): end component supports passing the previous node's error to the callback function
- feat(components): Adds break components
- feat(components): for node components support interrupts
- feat(components): Delay node components support obtaining time offsets through metadata
- feat(components): join/groupAction components support merging execution results into the same map
- feat(components): Function components support parameter configuration
- feat(components): Function component function registration supports adding display names and descriptions
- feat: Execute rule chains support instance cancellation
- feat: engine supports multi-node recovery 
- feat: Added Config.OnEndWithFailure configuration, whether to call the end callback function when an error occurs
- fix: Engine onEnd Callback may not trigger in some cases
- fix: Enforcement snapshot data contention issues
- perf: Optimized the engine's timeout context performance
- perf: Optimized the execution order of engine end-of-callbacks
- chore(ci): Compresses the compiled file
### rulego-components
- feat: opcua write components support types such as int, double, and arrays
- chore(ci): Upgrade dependencies

# [v0.34.0] 2025/11/03
### rulego-core
- feat(components): Delay component (delayNode), with delay time supporting milliseconds
- feat(components): Reference components (refNode) support referencing subchains
- feat(components): The database client component (dbClient) supports executing DDL and database dialects
- feat: Component form generation, supports generating icon fields
- feat: Component form generation, perfecting form configuration via tag
- fix: Fixed mqtt endpoint component initializing two clients
- fix: Fixed not supporting multi-layer nested cross-node value retrieval
- fix: Fixed subchain not supporting cross-node value retrieval
- fix(components): fieldFilter Logical error in component CheckAllKeys mode
- fix(components): Logical error in CheckAllKeys mode
- fix(components): join Components are blocked in certain cases
- chore(ci):actions workflow increases mysql
### rulego-components
- feat: nats endpoint components support QueueSubscribe mode

# [v0.33.0] 2025/09/03
### rulego-core
- feat: Improve the component configuration expression value system, supporting cross-node values, for example: ${node1.msg.xx}
- feat: Added end node components
- feat: Added cross-node value node components
- feat: Node component configuration form generation, skipping non-exportable and `json:-` fields
- perf: Improve the mqtt client reconnection mechanism
- perf: Optimized js engine timeout handling
- perf: Optimized expression engine performance for mixed string scenes
- perf: Use el.NewTemplate instead of str.NewTemplate
- perf: Improve net endpoint component data contention issues
- fix: js Node component, dataType field type conversion error
- fix: Reload engine chainCtx lost
- fix: Fixed read/write errors in some scenes of js scripts
- refactor: Refactoring ctx.TellFlow Entering the parameter
- refactor: Added common component categories and readjusted some component categories
- chore: expr Upgraded to 1.17.6

### rulego-components
- feat: Added pulsar publishing and subscription node components
- feat: Added pulsar publishing and subscription node components
- feat: Added streaming computation conversion node components
- feat: Added Stream Aggregation Node Components

### rulego-server
- fix: Improve the mqtt client reconnection mechanism
- fix: add defer resp.Body.Close() for GetComponentsFromMarketplace

### rulego-editor
- feat: By default, the [Input] node can be deleted
- feat: Added support for the latest node components
- feat: If a node is not configured, add it to the canvas for the first time without popping up the properties configuration form
- feat: Added new canvas nodes for displaying grouped components such as for and node groups
- feat: Added sql Editor form components
- fix: Left sidebar height adaptation
- chore: Upgrade the latest dependencies

# [v0.32.0] 2025/07/11

### rulego-core
- feat: endpoint/http restApiCall supports seamless switching to fasthttp implementation
- feat: endpoint Configure support for variable substitution
- feat: Rule engine overload adds error recovery mechanism
- feat: Rule engine adds graceful closing
- feat: Added a write-time replication (Copy-on-Write) mechanism for message passing
- feat: RuleMsg Increase zero-copy API
- feat: RuleMsg Message load uses []byte instead of string
- feat: Script components support handling byte array inputs
- feat(endpoint/http): Added read/write timeout configuration
- feat(endpoint/ws): Improved event registration
- feat(endpoint/net): Supports multiple package split configurations
- fix: Fixed race conditions between multiple component OnMsg and Destroy methods
- fix: Fixed concurrent faults in expression engine `vm.VM`
- fix: Enhanced ReloadChild and ReloadSelf method protection
- fix: Fixed endpoint Marshal DSL loop dependency issues
- fix: Fixed groupAction and groupFilter data contention
- fix(endpoint/mqtt): MaxReconnectInterval Supports seconds allocation methods
- refactor: Component configuration field names are prioritized from JSON tag
- refactor: Optimized exprFilter component initialization errors
- refactor: Improved restApiCall component proxy logic
- refactor: Rename the Config NetPool field to NodePool
- refactor: does not support direct access to msg.Data; use msg.GetData() and msg.SetData('') instead
- perf: Added intelligent passthrough mode to script components
- perf: Simplified implementation of shared node components
- perf: Use object pool optimization DefaultRuleContext
- perf: Optimize expressions to obtain variable performance
- perf: All components and test cases are tested in `-race` mode
- perf: Improving code comments
- perf: Added more examples and test cases

### rulego-components
- feat: Added fasthttp components
- feat: kafka Components add SASL and TLS configurations
- feat: Lua Scripts support handling byte streams
- feat: Lua Scripts support array transformation
- feat: Added integration testing and CI settings
- feat(ci): Added comprehensive GitHub Actions CI/CD pipeline and middleware testing
- feat(ci): Triggers CI for all pull requests
- fix: Fixed kafka component reconnection issue
- perf: Optimize metadata access with zero copy
- perf: Improved lifecycle management and testing

# [v0.31.0] 2025/05/20

### rulego-core
- feat: Add cacheSet/cacheGet/cacheDelete component nodes
- feat: Added cache module
- feat(restApiCall): Allows custom body and optimized variable values
- feat: node configuration supports mixing strings and variable values
- feat: Add AddNode API to the node pool
- feat: base endpoint Add HasRouter API
- feat: Add default HTTP endpoint to the node pool
- feat: endpoint Obtain the rule chain DSL
- feat(rest endpoint): rest endpoint Restart adds a timeout to close off
- feat: Unified registration methods for js and lua custom functions
- feat: supports binding all struct export functions to js and lua
- feat: Scripts can manipulate caches
- fix(switch): Fixed Switch node configuration not fully overriding the default cases parameters
- fix(restApiCall): restApiCall node failed request and could not retrieve request error messages in the metadata 
- fix(rest endpoint): rest endpoint Shared node hot update cannot restore routing
- fix(join): Join node has not collected error node information
- refactor: Optimize JS engine test cases
- refactor: Remove unnecessary code
- refactor: Hot update endpoint Routing recovery ignores errors
- chore: Optimized annotations

### rulego-server
- fix: Fixed HTTP Issue not found after server reboot `/editor`
- feat: Register mcp server endpoint
- feat: Shared system default http server

### rulego-components
- feat: Lua scripts support the same UDF registration methods as JS
- feat: Lua Scripts can call caching methods
- feat: Add mcp server endpoint

### rulego-editor
- feat: Added cache components
- feat: rest node adds body parameters for custom configuration
- feat: Add mcp server endpoint nodes
- fix: Fixed the issue where copying and deleting shortcut keys did not work in certain cases
- opt: Optimized integrated display

## [v0.30.0] 2025/04/03
- feat: Added dynamic components, supporting component definition via rule chains DSL
- feat: Component Registerer adds support for multiple tenants
- feat: Engine pool supports adding, modifying, and deleting callbacks from rule engine instances
- feat: Component adds CategoryGetter DescGetter optional interfaces
- feat: Required fields have been added to component forms
- feat(server): Increase module markets, module installation, module unloading, API
- feat(server): Adds MCP servers
- feat(server): Components, rule chains, rulego-server API support automatic registration as MCP tools
- feat(server):rulego-server Separated into independent warehouse maintenance: https://github.com/rulego/rulego-server
- feat(server):rulego-server UI of the new open-source version
- fix: Fixed the issue where only one type of shared node can be configured
- fix:OutBuiltins lock err
- fix:[dbClient] Errors caused by unsuccessful connections
- opt: Optimized component initialization error prompts
- opt:rest endpoint Component delays body acquisition
- chore:build.yaml Supports compiling to arm64
- chore: Upgrade github.com/expr-lang/expr to v1.17.2

## [v0.29.0] 2025/03/06
- feat(components): Added wukongIM node component @dimon
- feat(components): Added wukongIM input component @dimon
- feat(components): Added beanstalkd input component @dimon
- feat(components): Added beanstalkd node component @dimon
- feat(components): Added modbus read/write node components @dimon
- feat(components): Improve the node components of large models
- feat(components): Added the component for obtaining git log nodes
- feat: Added rule chain validation interceptors
- feat: Check whether the rule chain forms a loop; the sub-rule chain does not allow exploration of the input component
- feat:DSL NodeConnection Add Label fields
- opt: Delayed initialization of network client components
- opt:restApiCall node components pass response errors to the next node via err
- feat(server):rulego-server supports multi-tenant and permission validation
- feat(server):rulego-server supports apiKey access to api
- refactor:OnNodeBeforeInit and OnChainBeforeInitAspect support for obtaining Config
- refactor(components): Deprecating older large model components
- refactor(components): Optimized mqtt client connection failure error message

## [v0.28.0] 2025/01/09
- feat(components): Adds opcua endpoint components @dimon
- feat(components): Added opcua read node component @dimon
- feat(components): Added opcua write node component @dimon
- feat(components): Adds gRPC flow endpoint component @付晨阳
- feat(components): Adds Mysql CDC endpoint components
- feat(components): Adds OpenTelemetry components
- feat(components):endpoint/ws supports cross-origin configuration
- feat:for node adds asynchronous mode
- feat:js Engine injection RuleContext @Husky
- fix: Solve rule chains with multiple end points, which will export endpoint exceptions
- fix:str.ExecuteTemplate Issues with null parameters
- fix(server):save api Cannot save vars
- opt(components): Optimize parameters obtained from dbClient components
- opt: Optimized node form definitions
- opt:restApiCall Change the default timeout value to 2000ms
- opt(components): After receiving data from the redis endpoint component, XDel @Brian B. Williams

## [v0.27.0] 2024/12/08

- feat: Allows for endpoint router errors
- feat: Rule chain DSL add Disabled fields
- feat(endpoint/rest): Allows cross-origin settings
- feat(restApiCallNode): Allows configuration without proofreading verification certificates
- feat(flow): Subrule chains can be set to inheritance mode
- feat: If the rule chain is Disabled, the initialization engine is incorrect
- feat(groupActionNode): The node ID list allows string and array formats
- feat(builtin): Adds built-in functions for toHex and setJsonDataType
- feat(netNode): Supports not sending heartbeat packets
- fix(endpoint/rest): Type recognition error
- opt(netNode): Optimized the reconnection mechanism
- refactor:dsl additionInfo changed to map[string]interface{} type
- refactor: Removes log dependencies
- refactor(server): Reconstructs rulego-server api
- feat(server): Rules chain storage adds indexes
- feat(server): Automatically creates default users
- feat(server): Adds and disables the rule chain API
- feat(server): Allows searching for rule chains through Disabled fields
- feat(server): Adds default front-end access routes
- fix(server): Startup error exits
- ci(server): Reduces the size of the compilation package file
- ci(server): Provides the RuleGo-Editor Editor offline deployment package
### RuleGo-Editor[v0.27.0]
- feat(rulego-editor): Manage the rule chain list
- feat(rulego-editor): Displays the status and title of the rule chain
- feat(rulego-editor): Opens the rule chain
- feat(rulego-editor): Edit the rule chain
- feat(rulego-editor): Query rule chain integration URL
- feat(rulego-editor): Optimized import and export
- feat(rulego-editor): Component management
- feat(rulego-editor): Background API configuration persistence
- feat(rulego-editor): Rule chain deployment/offline operation
- feat(rulego-editor): Added tools for box selection, undo, redo, minimap, and fullscreen operations
- feat(rulego-editor): Sub-rule chain nodes allow selecting sub-rule chains by dropping down

## [v0.26.0] 2024/11/07

- feat: Add comment nodes
- feat: Add conditional branch nodes (switch node)
- feat: Added a rule engine indicator statistics module
- feat: Increase concurrency limits aspect
- feat:start aspect Provides error interrupt mechanisms
- feat: provides NewRuleGo Api
- feat:net components allow the use of node pooling
- fix:flow node Concurrent read/write issues
- fix:http endpoint Asynchronous execution will cause context canceled
- refactor:js Converter component ignores json conversion error
- refactor: Refactoring the built-in function registerer
- refactor: Change the default relationship of the route node to Default
- chore: Improved some annotations
- fix(server):config.conf allows configuration js to execute operation parameters
- feat(rulego-components): Adds MongoDB node components
- feat(rulego-components): Adds redis publishing node components

## [v0.25.0] 2024/10/07

- feat: Added parallel network node components
- feat: Added merged aggregated node components
- feat:for Node component adds an option to merge traversal results
- feat: Remove merge metadata from node groups and sub-rule chain nodes
- feat:ruleContext allows access to Out Message and error
- feat:websocket endpoint setBody Returns an error
- feat: Added js built-in function registerer
- fix:http endpoint Node pool cannot be used
- chore: Add contribution documents
- chore: Upgrade dependencies
- perf(server): Optimized the storage of runtime logs
- fix(server): Real-time execution logs require filtering other rule chain data
- fix(server): Real-time log response error, client need to be removed
- feat(rulego-components): Adds gRPC client node components
- feat(rulego-components): Add git push node components
- feat(rulego-components): Adds git create tag node components
- feat(rulego-components): Add git commit node components
- feat(rulego-editor): Added the latest version of node configuration
- feat(rulego-editor): Allows nodes to be replicated across rule chains

## [v0.24.0] 2024/09/09

- feat: Added a mechanism for reusing node connection resources
- feat: Network connection components support shared connection pools
- feat: Add nodes that reference nodes
- feat:exec node Allows data to be accessed via stderr
- feat:http endpoint Allows responses to html pages
- fix(server):post msg api No workDir
- feat(server): Added node reuse related api
- feat(server): Loads globally shared components
- feat(rulego-components): Adds rabbitmq endpoint and node components
- feat(rulego-components): Adds opengemini read and opengemini write components
- feat(rulego-components): Components support connection pools
- refactor(rulego-components): Change the brokers field of kafka component to server
- feat(rulego-editor): The rule chain ID uses nanoid by default
- feat(rulego-editor):endpoint supports multiple routes
- feat(rulego-editor): Added internationalization of connection types
- feat(rulego-editor): Added connection pool dropdown options
- feat(rulego-editor): Added the latest version of node configuration

## [v0.23.0] 2024/08/11
- feat(server): Dynamically retrieves the API of the built-in function list of functions nodes
- feat(server): log pagination
- feat(server):config.conf supports custom global configuration
- feat(rulego-components): Adds redis stream endpoint components
- feat(rulego-components):redis components support configuring passwords
- feat(rulego-components):redis components support operations such as HMSET, HGETALL, HDEL
- feat(rulego-components):redis components support dynamic parameters
- feat(rulego-components-ci): Adds gitClone components
- feat(rulego-components-ci): Adds server metric monitoring components, such as cpu, memory, disk, network, etc
- feat(builtin/processor): Adds metadataToHeaders built-in processor function
- feat(builtin/processor): Built-in responseToBody function supports all endpoint types
- feat:rest endpoint GET Request, message load is read from query parameters
- feat: Standardize the configuration variable value selection method across all components.
- fix(server): Cannot delete the rule chain
- fix(server):websocket Disconnection error
- fix:for node Modify out data
- fix:TellNode Node not found, no second callback triggered
- fix:dbClient node On certain go versions, the conversion int64 error
- fix:ToString Function adaptation map[interface{}]interface type {}
- refactor: Print endpoint Detailed error stack
- refactor:builtin/processor Distinguish between in and out types
- refactor: Optimize the rule chain parser

### RuleGo-Editor[v1.4]
- feat: Supports configuration of components rulego latest version
- feat: Supports endpoint component configuration
- feat: Supports dropdown forms
- fix: Fixed border text out-of-bounds issue
- fix: No prompt for failing to save the rule chain
- fix: Fixed the issue where the value could not be displayed
- fix: Custom components cannot display issues
- refactor:Input nodes allow movement
- refactor: Added help document links
- refactor: Upgrade element-plus
- refactor: Introduce element-plus zhCn lang

## [v0.22.0] 2024/07/08
- feat[rulego-editor]: Access terminal (endpoint) allows visual configuration. Experience link: [http://8.134.32.225:9090/ui/](http://8.134.32.225:9090/ui/)
- feat[rulego-components]: Adds redis endpoint components
- feat[rulego-components]: Added redis node components to allow configuration of db parameters
- feat[rulego-components]: Adds nats endpoint components
- feat[rulego-components]: Adds nats node components
- feat: Added for node components to control loop nodes
- feat: Added a component for executing local command nodes to control loop nodes
- feat: Added template node components
- feat: Added metadataTransform node components
- feat: Increases OnChainBeforeInitAspect and OnNodeBeforeInitAspect enhancement points
- feat: Added API related to rule engine interruption recovery
- feat: endpoint Allows specifying execution starting from a node in the rule chain
- fix: mqtt client Smooth closing
- refactor: endpoint type Add a prefix to the name
- refactor: iterator Deprecated node component tags

## [v0.21.0] 2024/06/06

- feat: rule chain DSL Allows dynamic configuration of access terminals (endpoint)
- feat: Access Terminal (endpoint) allows dynamic configuration and startup via DSL
- feat: endpoint Starts with no blocking
- feat: endpoint router Allows passing context
- Merge feat: endpoint component registration and rule component registration
- feat: Added nats node components
- If there is no match between nodes feat: msgTypeSwitch and jsSwitch, forwarding them to the default chain
- feat: Added nats endpoint components
- fix: Sub-rule chain context loss issues
- fix: examples/server Rule chain file parses fail and are not saved
- refactor: endpoint Module optimization and adjustment of directory structure
- refactor: engine Module optimization and adjustment of directory structure
- refactor: Optimized aspect initialization
- chore: examples/server build close CGO_ENABLED
- chore: examples/server Add nats components

## [v0.20.0] 2024/04/24
- feat: Allows different scripts to have the same function name
- feat: restApiCall nodes allow empty body
- feat: can obtain snapshots of rule chain execution
- feat: Allows adding onDebug callback functions in OnMsg context
- feat: endpoint Allows adding RuleContextOption
- feat: Rule chain DSL file can add vars variables
- feat: node configuration allows replacement with vars values via the rule chain
- feat: Rules Chain pools add reload and range methods
- feat: websocket endpoint allows rest endpoint to share a server
- feat: node debugMode allows unified override by the debugMode parameters of the rule chain
- feat: The sub-rule chain allows connections through Failure and other nodes
- feat: Load the rule chain and skip the faulty rule chain
- feat: Added initialization flags to the Rule Chain engine
- feat: js Relevant node runtime allows access to the rule chain vars via `vars.xx`
- feat: Refactoring examples/server provides scaffolding for rulego-based application development, frontend address: [example.rulego.cc](https://example.rulego.cc/)
- feat: Add rulego-components-ai modules to provide AI components
- feat: Add rulego-components-ci modules to provide CD/CI components
- feat: Add rulego-components-iot modules to provide iot components
- fix: mqtt client If nodes cannot connect mqtt broker allow delayed connections instead of errors
- fix: Fixed groupAction nodes, which may cause concurrent read/write issues
- fix: Rule chain has no nodes, so execution error issues
- opt: Optimized execution efficiency for large js files

## [v0.19.0] 2024/02/18

- feat: Added expression filter node components. [Document](https://rulego.cc/pages/c8fe75/)
- feat: Added expression conversion node components. [Document](https://rulego.cc/pages/3769cc/)
  Example expression:
  Function used: upper(msg.name)
  Judgment: (msg.temperature+10)> 50
  Ternary operations: upper(msg.name==nil? 'no':msg.name)
  Extract string: msg.name[:4]
  Replace the string: replace("Hello World", "World", "Universe") == "Hello Universe"

- feat: Add groupAction node components to group multiple nodes into a group, execute all nodes asynchronously, wait for all nodes to finish, then merge all node results and send them to the next node. [Document](https://rulego.cc/pages/bf06e2/)
- feat: Added iterator node components. Traverse the value of each specified field in msg or msg to the next section. [Document](https://rulego.cc/pages/5898a0/)
- fix: Fixed subrule result merging and concurrency issues.
- fix:onEnd Some reasons may repeatedly call the issue.
- fix:metadata Concurrent read/write issues may occur.
- fix:js Engine initialization adds concurrency protection.
- fix:jsTransform encounters NaN value and flows to TellFailure branch.

## [v0.18.0] 2023/12/27

- feat: Added AOP module, which allows adding extra actions to the execution of the rule chain or node without modifying the original logic of the rule chain or node, or directly replacing the original rule chain or node logic. Provides the following enhancements: Before Advice, After Advice, Around Advice, Start Advice, End Advice, Completed Advice, OnCreated Advice、OnReload Advice、OnDestroy Advice. [Document](https://rulego.cc/pages/a1ed6c/)
- feat:restApiCall node components, adding SSE(Server-Sent Events) streaming request mode, supporting integration with large model interfaces.
- feat: Increase CI automated testing processes.
- feat: Increased a large number of unit tests, achieving a 92% coverage rate.
- feat: Enhanced performance [test case](https://rulego.cc/pages/f60381/).
- feat:sendEmail node components to add ConnectTimeout configuration.
- feat:/examples/server Example project, adds -js -plugins -chain_id flags, supports launching and loading js native files, plugins, and specified mqtt subscription processing rule chains ID.
- fix:/examples/server Example project: Multi-layer paths in the Rule Chain folder cannot be parsed properly.
- fix:/examples/server Example project: Saving the rule chain, may encounter issues where the old rule chain file data cannot be properly overwritten.
- fix:metadata Concurrent read/write issues may occur.
- fix: The rule engine processes data synchronously, which may fail to correctly call the onCompleted callback function.
- fix:RuleChainPool nil Problem.
- fix:mqtt endpoint, cannot get the theme through header.
- refactor:onEnd The callback function allows relationType.
- refactor: Delete function Configuration. GetToString.
- opt: Partial components to enhance nil inspection.
- opt:dsl AdditionalInfo field adds omitempty json tag.
- opt:run go fmt。

## [v0.17.0] 2023/11/27

- feat: Added websocket endpoint Component [Documentation](https://rulego.cc/pages/e36f41/)
- feat: Added tcp/udp endpoint Component [Documentation](https://rulego.cc/pages/b7050c/)
- feat: Add kafka endpoint Components (Expand Component Library) [Documentation](https://rulego.cc/pages/07ad50/)
- feat: Added tcp/udp node component [documentation](https://rulego.cc/pages/c1af87/)
- feat:endpoint Components use a unified creation method [Documentation](https://rulego.cc/pages/5a3227/)
- feat: Added Filter Group Node Components [Documentation](https://rulego.cc/pages/b14e3b/)
- feat: Added sub-rule chain node components (atomic rule chain configuration is obsolete) [Document](https://rulego.cc/pages/e27cec/)
- feat: Allows subrules to link to other nodes
- feat:functions node component, supports dynamically specifying function names
- feat:delay node components to add override modes
- feat: Supports loading JavaScript script files
- feat:onEnd Callback function, supports obtaining ctx
- feat:examples/server Use independent go.mod
- feat:examples/server Supports introducing build tags of the extended component library
- feat:mqtt client Reconnection allowed is canceled
- fix:http endpoint If not application/json, body cannot be obtained
- fix:mqtt client node components, with no retry limit
- opt:Metadata Modify the implementation
- opt:rest node ReadTimeoutMs Change the default value to 0
- opt:mqtt client config MaxReconnectInterval changed to int
- opt:Node Interface OnMsg Cancel return value error
- opt:config.JsMaxExecutionTime->ScriptMaxExecutionTime
- opt:Endpoint.AddRouterWithParams->Endpoint.AddRouter
- opt:Endpoint.RemoveRouterWithParams->Endpoint.RemoveRouter
- opt:RuleMetadata.RuleChainConnections Marking is deprecated
- opt:config.OnEnd Marking is deprecated
- opt:RuleEngine.OnMsgWithEndFunc Marking is deprecated
- opt:RuleEngine.OnMsgWithOptions Marking is deprecated
- opt: Add doc overview

## [v0.16.0] 2023/10/30

- feat: Provides a rule chain visualization editor RuleGo-Editor [Online Use](https://editor.rulego.cc/)
- feat: Added ssh node component [Documentation](https://rulego.cc/pages/fa62c1/)
- feat: Added Latency Node Component [Document](https://rulego.cc/pages/5f5612/)
- feat: Added functions node component [Documentation](https://rulego.cc/pages/b7edde/)
- feat:dbClient node components support manual import of database drivers, such as TDengine
- feat: Added schedule endpoint Component [Documentation](https://rulego.cc/pages/4c4e4c/)
- feat:http endpoint Increase global options handler
- feat: Added example projects for rule engines running independently as middleware, and provided binary files [examples/server](https://github.com/rulego/rulego/tree/main/examples/server)
- feat:endpoint.AddRouterWithParams Return routerId
- feat: Visualize the json returned by the api, change the field initials to lowercase
- feat:onDebug Callback function, which can obtain the id of the rule chain
- feat: Perfecting ctx.TellSelf logic
- fix: Rule chain JSON file, change the node Id field to lowercase: id
- opt:upgraded github.com/dop251/goja v0.0.0-20230605162241-28ee0ee714f3 => v0.0.0-20231024180952-594410467bc6
- opt: Adjust the component package structure
- Change opt:dbClient node dbType to driverName
- opt: Perfecting documentation

## [v0.15.0] 2023/10/7

- feat: Added official document website: [rulego.cc](https://rulego.cc/)
- feat: Increase visualization related API. [Document](https://rulego.cc/pages/cf0193/)
- feat: Added global rule chain configuration Properties. [Document](https://rulego.cc/pages/config/#properties)
- feat: Adds global rule chain configuration and custom functions to js runtime, js scripts can call golang custom functions. [Document](https://rulego.cc/pages/config/#udf)
- feat: Added synchronous call to the rule chain: `OnMsgAndWait`.
- feat:http Endpoint Support for responding to the processing results of the rule chain to the frontend.
- feat:Endpoint module, routing adds Wait() semantics, indicating synchronized waiting for the execution result of the rule chain.
- feat: Added batch trigger rule engine instance pools for all rule chain message processing methods.
- feat:DefaultRuleContext Increase onAllNodeCompleted pullbacks.
- feat:DefaultRuleContext Added parentRuleCtx to support more flexible nested rule chains.
- fix: Fixed log component metadata parameter loss issue.
- fix:examples/server getDsl The response head is not `application/json`.
- opt: All components `config` changed to uppercase `Config` became public.
- opt: Optimize the call method of the subrule chain.
- opt:restApiCall Component ReadTimeoutMs parameter is set to 2000ms by default.
- opt: all test rule chains json files and add ruleId.
- opt: Optimize documents.

## [v0.14.0] 2023/9/6

### New Features

- [examples] Added extensive usage examples: [Details](https://gitee.com/rulego/rulego/tree/main/examples)
- [Standard Components] Add database client node components (dbClient), supporting both mysql and postgres databases. You can add, delete, or modify the database via configuration in the rule chain. Check: [Usage Example](https://gitee.com/rulego/rulego/tree/main/examples/db_client)
- [[Extension Component](https://gitee.com/rulego/rulego-components)] Adds redis Client Node Component (x/redisClient): [Usage Example](https://gitee.com/rulego/rulego-components/tree/main/examples/redis)
- [Rule Chain Engine] Added the ability to load all rule chains in the specified path folder
- [HTTP Endpoint Component] URL Query parameters are automatically stored in msg.Metadata
- [msg] msg.Metadata value Allowed to be null
- [Node Components] Node configuration, supports string mapping to time.Duration type
- The rule chain configuration file supports configuring rule chain id

### Fix

- Fixed an issue where mqttClient node components were not effective with random clientId

### Improvements

- [Endpoint](https://gitee.com/rulego/rulego/blob/main/endpoint/README_ZH.md) Interface abstraction, implements types.Node interfaces, which can be uniformly called by the Endpoint "type" at the upper level
- js Script-related nodes, handling msg supports array and map methods
- Change the configuration Addr to Server in the [HTTP Endpoint Component].

### Other information

- Feedback or suggestions are welcome on [Gitee](https://gitee.com/rulego/rulego) or [Github](https://github.com/rulego/rulego).
- Extension Component rulego-components: [Gitee](https://gitee.com/rulego/rulego-components) [Github](https://github.com/rulego/rulego-components)
- Welcome to join the community discussion QQ group: 720103251


## [v0.13.0] 2023/8/23

### New Features

- Added a data integration module (**Endpoint**), click in documentation and introduction: [Gitee](https://gitee.com/rulego/rulego/blob/main/endpoint/README_ZH.md) or [Github](https://github.com/rulego/rulego/blob/main/endpoint/README_ZH.md)
    - 提供统一的数据处理抽象，方便异构系统数据集成，目前支持HTTP和MQTT协议
    - 支持其他协议集成扩展，例如：kafka数据等
    - 支持统一的数据路由和数据响应
- Added a field filter component (**fieldFilter**)
- Added RuleEngine.OnMsgWithOptions method to support passing context and sharing data
- Component support ctx.GetContext().Value(shareKey) to obtain shared data


### Fix

- Fixed RuleEngine rootCtx security issues

### Improvements

- jsFilter, jsSwitch, jsTransform, log components: Under dataType=JSON data type, support js scripts to operate msg payload in msg.xx ways
- Rename mqttClient component tls relevant fields
- Optimize Metadata usage
- Optimized testcases
- Optimized README

### Other information

- Added RuleGo Extension Component Library project; contributions are welcome
    - 详情点击：[Gitee](https://gitee.com/rulego/rulego-components) 或者 [Github](https://github.com/rulego/rulego-components)

- Feedback or suggestions are welcome on [Gitee](https://gitee.com/rulego/rulego) or [Github](https://github.com/rulego/rulego).
