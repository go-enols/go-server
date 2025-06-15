# 贡献指南

感谢您对 go-server 项目的关注！我们欢迎所有形式的贡献，包括但不限于：

- 🐛 Bug 报告
- 💡 功能建议
- 📝 文档改进
- 🔧 代码贡献
- 🧪 测试用例

## 开始之前

### 环境要求

- **Go**: 1.21 或更高版本
- **Python**: 3.8 或更高版本
- **Node.js**: 16 或更高版本
- **Git**: 最新版本

### 项目结构

```
go-server/
├── main.go              # 主程序入口
├── scheduler/           # 调度器核心
├── worker/             # Worker 管理
├── web/                # Web UI
├── python-sdk/         # Python SDK
├── node-sdk/           # Node.js SDK
├── go-sdk/             # Go SDK
└── .github/workflows/  # CI/CD 配置
```

## 开发流程

### 1. Fork 和 Clone

```bash
# Fork 项目到您的 GitHub 账户
# 然后 clone 到本地
git clone https://github.com/YOUR_USERNAME/go-server.git
cd go-server

# 添加上游仓库
git remote add upstream https://github.com/go-enols/go-server.git
```

### 2. 创建分支

```bash
# 从 develop 分支创建新的功能分支
git checkout develop
git pull upstream develop
git checkout -b feature/your-feature-name
```

### 3. 本地开发

#### Go 主程序开发

```bash
# 安装依赖
go mod tidy

# 运行程序
go run main.go

# 运行测试
go test ./...

# 代码格式化
go fmt ./...
```

#### Python SDK 开发

```bash
cd python-sdk

# 创建虚拟环境
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# 安装依赖
pip install -r requirements.txt
pip install -e .

# 运行测试
python -m pytest

# 代码格式化
black .
isort .
```

#### Node.js SDK 开发

```bash
cd node-sdk

# 安装依赖
npm install

# 运行测试
npm test

# 代码格式化
npm run format
```

### 4. 提交更改

#### 提交信息规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/v1.0.0/) 规范：

```
<类型>[可选的作用域]: <描述>

[可选的正文]

[可选的脚注]
```

**类型说明：**
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式化（不影响功能）
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

**示例：**
```bash
git commit -m "feat(scheduler): 添加任务优先级支持"
git commit -m "fix(python-sdk): 修复连接超时问题"
git commit -m "docs: 更新 README 安装说明"
```

### 5. 推送和创建 PR

```bash
# 推送到您的 fork
git push origin feature/your-feature-name

# 在 GitHub 上创建 Pull Request
# 目标分支：develop
```

## 代码规范

### Go 代码规范

- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 使用 `golangci-lint` 进行代码检查
- 函数和方法需要有注释
- 导出的类型和函数必须有文档注释

### Python 代码规范

- 遵循 [PEP 8](https://pep8.org/)
- 使用 `black` 格式化代码
- 使用 `isort` 排序导入
- 使用 `flake8` 进行代码检查
- 使用类型注解（Type Hints）

### Node.js 代码规范

- 遵循 [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)
- 使用 `prettier` 格式化代码
- 使用 `eslint` 进行代码检查
- 使用 TypeScript（推荐）

## 测试要求

### 单元测试

- 新功能必须包含相应的测试用例
- 测试覆盖率应保持在 80% 以上
- 测试应该是独立的，不依赖外部服务

### 集成测试

- 重要功能需要集成测试
- 测试应该覆盖主要的使用场景
- 确保 SDK 之间的兼容性

### 运行测试

```bash
# Go 测试
go test ./... -v -cover

# Python 测试
cd python-sdk && python -m pytest -v --cov

# Node.js 测试
cd node-sdk && npm test
```

## 文档要求

### 代码文档

- 所有公共 API 必须有文档注释
- 复杂的算法和逻辑需要详细注释
- 示例代码应该是可运行的

### 用户文档

- 新功能需要更新 README.md
- 重要更改需要更新 CHANGELOG.md
- 提供使用示例和最佳实践

## 发布流程

### 版本管理

我们使用 [语义化版本](https://semver.org/lang/zh-CN/) 规范：

- **主版本号**：不兼容的 API 修改
- **次版本号**：向下兼容的功能性新增
- **修订号**：向下兼容的问题修正

### 发布步骤

1. 更新版本号（package.json, setup.py, etc.）
2. 更新 CHANGELOG.md
3. 创建 Git 标签
4. 推送标签触发自动发布

```bash
# 创建标签
git tag -a v2.0.2 -m "Release version 2.0.2"
git push origin v2.0.2
```

## 问题报告

### Bug 报告

请使用 [Bug 报告模板](https://github.com/go-enols/go-server/issues/new?template=bug_report.md) 并包含：

- 详细的问题描述
- 重现步骤
- 期望行为
- 实际行为
- 环境信息（操作系统、版本等）
- 相关日志和错误信息

### 功能请求

请使用 [功能请求模板](https://github.com/go-enols/go-server/issues/new?template=feature_request.md) 并包含：

- 功能描述
- 使用场景
- 期望的 API 设计
- 可能的实现方案

## 社区准则

### 行为准则

- 尊重所有贡献者
- 保持友好和专业的交流
- 欢迎新手提问和学习
- 避免人身攻击和歧视性言论

### 沟通渠道

- **GitHub Issues**: 问题报告和功能请求
- **GitHub Discussions**: 一般讨论和问答
- **Pull Requests**: 代码审查和讨论

## 获得帮助

如果您在贡献过程中遇到任何问题，请：

1. 查看现有的 Issues 和 Discussions
2. 阅读项目文档和代码注释
3. 创建新的 Issue 或 Discussion
4. 在 PR 中 @维护者寻求帮助

## 致谢

感谢所有为 go-server 项目做出贡献的开发者！您的贡献让这个项目变得更好。

---

**记住：没有贡献是太小的，每一个改进都很重要！** 🚀