# go-server

这是一个基于 Go 语言实现的轻量级分布式任务调度系统。旨在通过将任务分发到多个 Worker 节点并行处理，从而解决单台计算机在处理计算密集型或 I/O 密集型任务时可能遇到的性能瓶颈问题。您可以将其视为一个基础的微服务架构雏形，用于构建可水平扩展的后台服务。

##核心理念

本项目的核心理念是将复杂的任务分解为更小的、可独立执行的工作单元，并通过一个中心化的调度器（Scheduler）将这些工作单元分发给一组工作节点（Worker）。客户端（Client）可以向调度器提交任务请求，并获取任务执行结果。

这种架构的优势在于：

*   **可伸缩性**：可以通过增加 Worker 节点的数量来提高系统的整体处理能力。
*   **灵活性**：不同的 Worker 可以执行不同类型的任务，方便横向扩展特定功能。
*   **容错性**：单个 Worker 节点的故障不会影响整个系统的运行（需要配合适当的重试和故障转移机制）。

## 项目结构

```
├── examples          // 示例代码
│   ├── client
│   ├── scheduler
│   └── worker
├── go.mod            // Go 模块文件
├── go.sum            // Go 模块校验和文件
├── scheduler.go      // 调度器核心逻辑实现
├── schedulersdk      // 客户端与调度器交互的 SDK
└── workersdk         // Worker 与调度器交互的 SDK

## 核心组件

1.  **调度器 (Scheduler)** (`scheduler.go`, `schedulersdk`):
    *   负责接收来自客户端的任务请求。
    *   管理可用的 Worker 节点列表。
    *   根据一定的策略（例如负载均衡、任务类型等）将任务分配给合适的 Worker。
    *   跟踪任务状态，并将结果返回给客户端。
    *   `schedulersdk` 提供了客户端与调度器通信的接口和辅助功能。

2.  **工作节点 (Worker)** (`workersdk`, `examples/worker`):
    *   向调度器注册自身，表明可以接收任务。
    *   从调度器接收分配的任务。
    *   执行任务的具体逻辑。
    *   将任务执行结果或状态上报给调度器。
    *   `workersdk` 提供了 Worker 与调度器通信的接口和辅助功能。
    *   `examples/worker/main.go` 是一个 Worker 节点的示例实现。

3.  **客户端 (Client)** (`examples/client`):
    *   通过 `schedulersdk` 与调度器进行通信。
    *   向调度器提交新的任务。
    *   查询任务状态或获取任务结果。
    *   `examples/client/main.go` 是一个客户端应用的示例实现。

## 交互流程

1.  一个或多个 Worker 节点启动，并通过 `workersdk` 向 Scheduler 注册。
2.  Client 应用通过 `schedulersdk` 连接到 Scheduler，并提交一个任务请求（例如，执行某个计算、处理某个数据等）。
3.  Scheduler 收到任务请求后，从已注册的 Worker 列表中选择一个合适的 Worker，并将任务信息发送给该 Worker。
4.  Worker 接收到任务后，执行预定义的任务逻辑。
5.  Worker 执行完毕后，将结果通过 `workersdk` 返回给 Scheduler。
6.  Scheduler 收到 Worker 的结果后，再通过 `schedulersdk` 将最终结果返回给 Client。
```

## TODO (目前工作量较大, 后续会逐步完善)
- [ ] 调度器目前只支持单个节点，也许可以考虑支持集群模式?

## 构建和运行

### 前提条件

- Go (请根据 `go.mod` 文件中的版本或更新版本安装)

### 构建

在项目根目录下运行：
```bash
go build
```

### 运行示例

根据 `examples` 目录下的具体示例运行，例如：

```bash
# 运行客户端示例
go run ./examples/client/main.go

# 运行调度器示例
go run ./examples/scheduler/main.go

# 运行 Worker 示例
go run ./examples/worker/main.go
```

## 贡献

欢迎提交 Pull Request 或 Issue。