# Contribution Guide

**Our mission is to build a new generation of rule engines suitable for various scenario requirements and to foster a vibrant component ecosystem, leading software development into a new paradigm of innovation.**

Welcome to the rulego community! We are an open and inclusive team dedicated to advancing the technology of rule engines. We believe that with the power of the community, we can together realize this vision.

## 1. Source Code Directory Structure

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

| Directory  | Description                                                                                              |
|------------|----------------------------------------------------------------------------------------------------------|
| api        | Defines interfaces for components, nodes, and rule engines                                               |
| builtin    | Built-in functions, slices, endpoint processors, etc.                                                    |
| components | Node components, such as action, filter, converter, external components, sub-rule chain components, etc. |
| doc        | Project documentation and update logs                                                                    |
| endpoint   | Input modules and components                                                                             |
| engine     | Core implementation of the rule engine                                                                   |
| examples   | Code examples and demonstrations                                                                         |
| node_pool  | Shared node pool                                                                                         |
| test       | Testing tools and scripts                                                                                |
| testdata   | Test datasets                                                                                            |
| utils      | Utility classes                                                                                          |
| rulego.go  | Engine initialization and execution entry point                                                          |

## 2. Code of Conduct

We are committed to providing an open and inclusive community environment. Please adhere to our [Code of Conduct] when participating in this project.

## 3. Submitting Issues/Handling Issue Tasks

- **Submit Issue**: Submit bug reports, feature requests, or suggestions in the [Github Issues](https://github.com/rulego/rulego/issues) .
- **Participate in Discussions**: Join the discussion of existing Issues and share your thoughts and feedback.
- **Claim an Issue**: If you are willing to work on an Issue, assign it to yourself by commenting `/assign @yourself`.

## 4. Contributing Source Code

### 4.1 Detailed Steps for Submitting Pull Requests

1. **Check Existing PRs**: Search for related PRs in [Github Pull Requests](https://github.com/rulego/rulego/pulls) to avoid duplicate work.
2. **Discuss Design**: Discussing your design before submitting a PR can help ensure that your work meets the project's requirements.
3. **Sign DCO**: Ensure each commit is signed with [DCO](https://developercertificate.org) using `git commit -s`.
4. **Fork the Repository**: Fork and clone the rulego/rulego repository on Github.
5. **Create a Branch**: `git checkout -b my-feature-branch main`.
6. **Write Code and Tests**: Add your code and corresponding test cases.
7. **Format Code**: Format your code using the `gofmt -s -w .` command.
8. **Commit the code**: Use `git add .` and `git commit -s -m "fix: add new feature"` to commit the changes.
    - `feat`: Abbreviation for feature, a new functionality or enhancement.
    - `fix`: Bug fix.
    - `docs`: Documentation changes.
    - `style`: Formatting changes. For example, adjusting indentation, spaces, removing extra blank lines, or adding missing semicolons. In short, changes that do not affect the meaning or functionality of the code.
    - `refactor`: Code refactoring. Modifications that are neither bug fixes nor new feature additions.
    - `perf`: Abbreviation for performance, improvements to code performance.
    - `test`: Changes to test files.
    - `chore`: Other minor changes. Typically one or two lines of changes, or a series of small changes that belong to this category.

   For more detailed information, please refer to [Conventional Commits](https://www.conventionalcommits.org/zh-hans/v1.0.0/).

9. **Push the code**: Before committing the code, please first perform a rebase operation to ensure that your branch is synchronized with the main branch of the upstream repository. 
   - `git fetch --all`
   - `git rebase upstream/main`
   - Push your branch to GitHub: `git push origin my-fix-branch`
10. **Create a PR**: Create a PR on Github and ensure you fill in a detailed PR description.

### 4.2 Compiling Source Code

#### 4.2.1 Supported Platforms
- All operating systems that support the Go language.

#### 4.2.2 Compilation Environment Information
- Go version: v1.20+ (without extended components: v1.18+)
- Git: [Download Git].

#### 4.2.3 GO Environment Variable Settings (Optional)
```bash
# Set GOPATH (customize the directory)
export GOPATH=$HOME/gocodez
export GOPROXY=https://goproxy.cn,direct
export GO111MODULE=on
export GONOSUMDB=*
export GOSUMDB=off
```

#### 4.2.4 Download Source Code and Compile
```bash
git clone git@github.com:<username>/rulego.git
cd rulego/examples/server/cmd/server
# Compile
go build .
# Or compile with extended components
go build -tags "with_extend,with_ai,with_ci,with_iot" .
```

For more details, refer to: [server](examples/server/README_ZH.md)

### 4.3 Start the Service
```bash
./server -c ./config.conf
```

### 4.4 Test the Rule Engine
Use the [RuleGo-Editor](https://editor.rulego.cc/) 、[RuleGo-Example](https://example.rulego.cc/) or [RuleGo-Server](http://8.134.32.225:9090/ui/)  UI for testing.

## 5. Participate in Other Community Contributions

### 5.1 Contribute Extended Components
- [rulego-components](https://github.com/rulego/rulego-components) : Other extended components.
- [rulego-components-ai](https://github.com/rulego/rulego-ai) : Components for AI scenarios.
- [rulego-components-ci](https://github.com/rulego/rulego-ci) : Components for CI/CD scenarios.
- [rulego-components-iot](https://github.com/rulego/rulego-iot) : Components for IoT scenarios.
- [rulego-components-etl](https://github.com/rulego/rulego-components-etl): ETL scenario components.
- [streamsql](https://github.com/rulego/streamsql): Subproject for enhancing edge computing aggregation capabilities.
- [rulego-marketplace](https://github.com/rulego/rulego-marketplace): Components marketplace.

### 5.2 Contribute Documentation
- Official documentation: [rulego-doc](https://github.com/rulego/rulego-doc) .

### 5.3 Contribute Test Cases

### 5.4 Contribute Tutorials
