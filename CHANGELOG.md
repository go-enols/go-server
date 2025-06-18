# Changelog

本文档记录了 go-server 项目的所有重要更改。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
并且本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [v1.4.3] - 2024-12-19

### 🔧 修复
- 版本更新到 v1.4.3
- 同步所有 SDK 版本号
- 前端新增支持Marked库解析文档

## [v1.4.2] - 2024-12-19

### 🔧 修复
- 版本更新到 v1.4.2
- 同步所有 SDK 版本号

## [v2.0.1] - 2024-12-19

### 🚀 新增
- 发布到 PyPI 和 NPM 的自动化工作流
- 完整的 CI/CD 管道配置
- 代码质量检查和安全扫描
- 多平台二进制文件构建和发布

### 📦 包管理
- Python SDK 发布到 PyPI: `pip install go-server-sdk==2.0.1`
- Node.js SDK 发布到 NPM: `npm install go-server-sdk@2.0.1`
- 更新了 README.md 中的安装说明

### 🔧 改进
- 统一了 SDK 命名规范（scheduler/worker）
- 优化了项目结构和文档
- 添加了版本管理和发布流程

### 🐛 修复
- 修复了 SDK 导入路径问题
- 统一了各语言 SDK 的接口设计

## [v1.3.0] - 2024-12-18

### 🚀 新增
- 多语言 SDK 支持（Python、Node.js、Go）
- Web UI 调试界面
- 方法文档化功能
- 负载均衡和任务分发

### 🔧 改进
- WebSocket 通信优化
- 自动重连机制
- 心跳检测功能
- 并发安全性改进

### 📚 文档
- 完善的 README.md
- 多语言示例代码
- API 文档说明

## [v1.2.0] - 2024-12-17

### 🚀 新增
- 基础的分布式任务调度功能
- Worker 节点管理
- 任务执行和结果返回

### 🔧 改进
- 系统架构优化
- 错误处理机制

## [v1.1.0] - 2024-12-16

### 🚀 新增
- 初始版本发布
- 基础的调度器功能
- Worker 连接管理

## 版本说明

### 语义化版本格式
- **主版本号**：不兼容的 API 修改
- **次版本号**：向下兼容的功能性新增
- **修订号**：向下兼容的问题修正

### 更新类型说明
- 🚀 **新增** - 新功能
- 🔧 **改进** - 功能改进和优化
- 🐛 **修复** - Bug 修复
- 📚 **文档** - 文档更新
- 📦 **包管理** - 依赖和包管理相关
- 🔒 **安全** - 安全相关修复
- ⚠️ **废弃** - 即将移除的功能
- 🗑️ **移除** - 已移除的功能

---

## 贡献指南

在提交更改时，请确保：

1. 更新此 CHANGELOG.md 文件
2. 遵循语义化版本规范
3. 在发布说明中包含重要更改
4. 测试所有受影响的功能

## 链接

- [项目主页](https://github.com/go-enols/go-server)
- [问题反馈](https://github.com/go-enols/go-server/issues)
- [PyPI 包](https://pypi.org/project/go-server-sdk/)
- [NPM 包](https://www.npmjs.com/package/go-server-sdk)