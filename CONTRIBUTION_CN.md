
# 贡献指南

**我们的使命是构建适用于各种场景需求的新一代规则引擎，并致力于培育充满活力的组件生态系统，引领软件开发进入全新的创新范式。**

欢迎加入rulego社区！我们是一个开放和包容的团队，致力于推动规则引擎技术的发展。我们相信，通过社区的力量，我们可以一起实现这一愿景。

## 1. 源码目录结构

```
rulego
├── api
├── builtin
├── components
├── doc
├── endpoint
├── engine
├── examples
├── node_pool
├── test
├── testdata
├── utils
├── rulego.go
```

| 目录         | 说明                   |
|------------|----------------------|
| api        | 定义组件、节点、规则引擎的接口      |
| builtin    | 内置函数、切片、endpoint处理器等 |
| components | 节点组件，如动作、过滤器、转换器等    |
| doc        | 项目文档和更新日志            |
| endpoint   | 输入端模块和组件             |
| engine     | 规则引擎核心实现             |
| examples   | 代码示例和演示              |
| node_pool  | 共享节点资源池              |
| test       | 测试工具和脚本              |
| testdata   | 测试数据集                |
| utils      | 辅助工具类                |
| rulego.go  | 引擎初始化和执行入口           |

## 2. 行为守则

我们致力于提供一个开放和包容的社区环境。请在参与本项目时遵守我们的[行为准则] 。

## 3. 提交Issue/处理Issue任务

- **提交Issue**：在[Github Issues](https://github.com/rulego/rulego/issues) 中提交Bug报告、功能请求或建议。
- **参与讨论**：加入现有Issue的讨论，分享你的想法和反馈。
- **领取Issue**：如果你愿意处理某个Issue，请在评论中使用`/assign @yourself`将其分配给自己。

## 4. 贡献源码

### 4.1 提交拉取请求详细步骤

1. **检查现有PR**：在[Github Pull Requests](https://github.com/rulego/rulego/pulls) 中搜索相关PR，避免重复工作。
2. **讨论设计**：在提交PR前，讨论你的设计可以帮助确保你的工作符合项目需求。
3. **签署DCO**：使用`git commit -s`确保每次提交都签署了[DCO](https://developercertificate.org) 。
4. **Fork仓库**：在Github上Fork并Clone rulego/rulego仓库。
5. **创建分支**：`git checkout -b my-feature-branch main`。
6. **编写代码和测试**：添加你的代码和相应的测试用例。
7. **格式化代码**：使用`gofmt -s -w .`命令格式化代码。
8. **提交和推送**：使用`git add .`和`git commit -s -m "fix: add new feature"`提交更改，然后推送到你的Fork仓库。
9. **创建PR**：在Github上创建PR，并确保填写详细的PR描述。

### 4.2 编译源码

#### 4.2.1 支持平台
- 所有支持Go语言的操作系统。

#### 4.2.2 编译环境信息
- Go版本：v1.20+ (不引入扩展组件：v1.18+)
- Git：[下载Git](https://git-scm.com/downloads)

#### 4.2.3 GO环境变量设置（可选）
```bash
# 设置GOPATH(可自定义目录)
export GOPATH=$HOME/gocodez
export GOPROXY=https://goproxy.cn,direct
export GO111MODULE=on
export GONOSUMDB=*
export GOSUMDB=off
```

#### 4.2.4 下载源码编译
```bash
git clone git@github.com:<username>/rulego.git
cd rulego/examples/server/cmd/server
# 编译
go build .
# 或者 加入扩展组件编译
go build -tags "with_extend,with_ai,with_ci,with_iot" .
```
详细参考：[server](examples/server/README_ZH.md)
### 4.3 启动服务
```bash
./server -c ./config.config
```

### 4.4 测试规则引擎
使用[RuleGo-Editor](https://editor.rulego.cc/) 、[RuleGo-Example](https://example.rulego.cc/) 或[RuleGo-Server](http://8.134.32.225:9090/ui/) 可视化界面进行测试。

## 5. 参与社区其他贡献

### 5.1 贡献扩展组件
- [rulego-components](https://github.com/rulego/rulego-components) ：其他扩展组件。
- [rulego-components-ai](https://github.com/rulego/rulego-components-ai) ：AI场景组件。
- [rulego-components-ci](https://github.com/rulego/rulego-components-ci) ：CI/CD场景组件。
- [rulego-components-iot](https://github.com/rulego/rulego-components-iot) ：IoT场景组件。
- [streamsql]() : 增强边缘计算聚合计算能力的子项目。

### 5.2 贡献文档
- 官网文档：[rulego-doc](https://github.com/rulego/rulego-doc)

### 5.3 贡献测试用例

### 5.4 贡献教程

