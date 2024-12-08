# server

[English](README.md)| 中文

`RuleGo-Server`一个独立运行的开箱即用规则引擎服务，该工程也是一个开发RuleGo应用的脚手架。你可以基于该工程进行二次开发，也可以直接下载可执行[二进制文件](https://github.com/rulego/rulego/releases)。

可视化编辑器：[RuleGo-Editor](https://editor.rulego.cc/) ，配置该工程HTTP API，可以对规则链管理和调试。

- 体验地址1：[http://8.134.32.225:9090/editor/](http://8.134.32.225:9090/editor/)
- 体验地址2：[http://8.134.32.225:9090/ui/](http://8.134.32.225:9090/ui/)

## HTTP API

[API 文档](https://apifox.com/apidoc/shared-d17a63fe-2201-4e37-89fb-f2e8c1cbaf40/234016936e0)

* 获取所有组件列表
    - GET /api/v1/components

* 执行规则链并得到执行结果API
    - POST /api/v1/rules/:chainId/execute/:msgType
    - chainId：处理数据的规则链ID
    - msgType：消息类型
    - body：消息体
  
* 往规则链上报数据API，不关注执行结果
  - POST /api/v1/rules/:chainId/notify/:msgType
  - chainId：处理数据的规则链ID
  - msgType：消息类型
  - body：消息体
  
* 查询规则链
    - GET /api/v1/rules/{chainId}
    - chainId：规则链ID

* 保存或更新规则链
    - POST /api/v1/rule/{chainId}
    - chainId：规则链ID
    - body：更新规则链DSL内容
  
* 保存规则链Configuration
    - POST /api/v1/rules/:chainId/config/:varType
    - chainId：规则链ID
    - varType: vars/secrets 变量/秘钥
    - body：配置内容

* 获取节点调试日志API
    - Get /api/v1/logs/debug?&chainId={chainId}&nodeId={nodeId}
    - chainId：规则链ID
    - nodeId：节点ID

  当节点debugMode打开后，会记录调试日志。目前该接口日志存放在内存，每个节点保存最新的40条，如果需要获取历史数据，请实现接口存储到数据库。

## server编译

为了节省编译后文件大小，默认不引入扩展组件[rulego-components](https://github.com/rulego/rulego-components) ，默认编译：

```shell
cd cmd/server
go build .
```

如果需要引入扩展组件[rulego-components](https://github.com/rulego/rulego-components) ，使用`with_extend`tag进行编译：

```shell
cd cmd/server
go build -tags with_extend .
```
其他扩展组件库tags：
- 注册扩展组件[rulego-components](https://github.com/rulego/rulego-components) ，使用`with_extend`tag进行编译：
- 注册AI扩展组件[rulego-components-ai](https://github.com/rulego/rulego-components-ai) ，使用`with_ai`tag进行编译
- 注册CI/CD扩展组件[rulego-components-ci](https://github.com/rulego/rulego-components-ci) ，使用`with_ci`tag进行编译
- 注册IoT扩展组件[rulego-components-iot](https://github.com/rulego/rulego-components-iot) ，使用`with_iot`tag进行编译

如果需要同时引入多个扩展组件库，可以使用`go build -tags "with_extend,with_ai,with_ci,with_iot" .` tag进行编译。

## server启动

```shell
./server -c="./config.conf"
```

或者后台启动

```shell
nohup ./server -c="./config.conf" >> console.log &
```
## RuleGo-Editor
RuleGo-Editor 是 RuleGo-Server 的UI界面，可以对规则链进行可视化管理，调试，部署等。

使用步骤：
- 解压下载好的`editor.zip`到当前目录，打开浏览器访问`http://localhost:9090/` ，即可访问RuleGo-Editor。
- 可以通过`config.conf`的 resource_mapping 配置修改rulego-editor目录。
- 可以通过`editor/config/config.js`的 baseUrl 配置修改rulego-editor后端api地址。

> RuleGo-Editor仅用于学习，商用请向我们购买授权。Email：rulego@outlook.com
## 配置文件参数
```ini
# 数据目录
data_dir = ./data
# cmd组件命令白名单
cmd_white_list = cp,scp,mvn,npm,yarn,git,make,cmake,docker,kubectl,helm,ansible,puppet,pytest,python,python3,pip,go,java,dotnet,gcc,g++,ctest
# 是否加载lua第三方库
load_lua_libs = true
# http server
server = :9090
# 默认用户
default_username = admin
# 是否把节点执行日志打印到日志文件
debug = true
# 最大节点日志大小，默认40
max_node_log_size =40
# 资源映射，支持通配符，多个映射用逗号分隔，格式：/url/*filepath=/path/to/file
resource_mapping = /editor/*filepath=./editor,/images/*filepath=./editor/images
# 节点池文件，规则链json格式，示例：./node_pool.json
node_pool_file=./node_pool.json

# 全局自定义配置，组件可以通过${global.xxx}方式取值
[global]
# 例子
sqlDriver = mysql
sqlDsn = root:root@tcp(127.0.0.1:3306)/test

```