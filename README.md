# ArgusGate

ArgusGate 是一个使用 Go 构建的 OpenAI-compatible AI 推理网关。它位于 AI 应用与独立推理服务之间，计划统一处理请求校验、后端路由、普通与 SSE 响应转发、取消传播、健康检查和可观测性。

> 当前状态：**v0.1 / Phase 0（基础服务）开发中**。仓库已具备 Go 模块、命令入口、最小 JSON 配置加载、HTTP 服务、`GET /healthz`、Request ID、基础结构化请求日志和最小 CI，并已接通 SIGINT/SIGTERM 到限时 `Server.Shutdown` 的第一阶段关闭流程；尚未完成受控强制取消、关闭超时配置化、完整关闭生命周期集成测试和模型请求代理。

## 项目目标

ArgusGate 不运行模型。llama.cpp、vLLM 等 OpenAI-compatible 服务作为独立 inference backend 运行，客户端只需要访问一个统一入口。

```text
AI Application
      |
      | OpenAI-compatible API
      v
+--------------------------------------+
| ArgusGate                            |
|                                      |
| Observe -> Validate -> Route -> Gate |
+------------------+-------------------+
                   |
          +--------+--------+
          |                 |
          v                 v
      llama.cpp            vLLM
```

v0.1 的核心目标包括：

- 提供 `POST /v1/chat/completions`，透明转发普通 JSON 与 SSE 流式响应。
- 在客户端断开后取消上游请求，避免继续占用推理资源。
- 按 `model` 精确匹配后端，并在健康实例之间 round-robin。
- 自动隔离异常后端，并在恢复后重新加入路由。
- 提供 `/healthz`、`/readyz`、`/metrics` 以及结构化请求日志。
- 支持本地运行、自动测试、race test 和 Docker 运行。

## 当前进度

截至当前工作区状态，项目处于路线图的 Phase 0。

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| Go module 与命令入口 | 已完成最小骨架 | module 为 `github.com/kongerly/ArgusGate` |
| JSON 配置默认值、读取与基础校验 | 已完成最小版本 | 支持默认监听地址、严格 JSON 读取和空地址校验，并覆盖主要错误路径测试 |
| 示例配置 | 已完成最小版本 | 当前仅包含 `server.address` |
| HTTP Server 与 `/healthz` | 已完成最小版本 | 服务监听配置地址；`GET /healthz` 返回 200，其他 method 由路由拒绝 |
| Request ID 与结构化请求日志 | 已完成最小版本 | 响应包含 `X-Request-ID`，请求结束记录 method、path、status、request ID 和 duration |
| `app.Run` 生命周期边界 | 已完成骨架 | App 组装 HTTP Server，并处理监听与 `Shutdown` 的基础流程 |
| SIGINT/SIGTERM 与优雅关闭 | 部分完成 | 入口捕获信号，App 使用 5 秒超时调用 `Server.Shutdown`；配置化超时、受控强制取消、goroutine 汇合与集成测试待补 |
| CI | 已建立最小流程 | GitHub Actions 在 push 和 pull request 时检查 format、vet 和 test；当前格式门槛仍有待清理的文件 |
| 非流式代理 | 未开始 | Phase 1 |
| SSE 与取消传播 | 未开始 | Phase 2 |
| 多后端路由与健康检查 | 未开始 | Phase 3 |
| Prometheus 指标 | 未开始 | Phase 4 |
| Docker 与发布验证 | 未开始 | Phase 5 |

当前代码已在 Go 1.27.1 / Windows amd64 环境通过：

```text
go test ./...
go vet ./...
```

`gofmt -l .` 当前仍会报告部分 Go 文件，因此 CI 的格式门槛尚未通过。现有测试覆盖配置默认值、覆盖读取及主要失败路径，验证健康检查、method 限制、Request ID 和请求日志，并覆盖 `app.Run` 在 context 取消时退出以及监听地址冲突时返回错误。操作系统信号、在途请求优雅完成、超时后的强制取消和完整真实监听生命周期仍缺少集成测试。

## 快速开始（当前开发状态）

### 环境要求

- Go 1.27.1（以 [`go.mod`](./go.mod) 的 `go` directive 为准）
- Git

克隆仓库后，在项目根目录执行：

```bash
go version
go mod download
go run ./cmd/argusgate -config ./configs/argusgate.example.json
```

当前预期启动日志类似：

```text
time=... level=INFO msg="starting HTTP server" address=127.0.0.1:8080
```

程序会持续监听 `127.0.0.1:8080`。在另一个终端验证探活：

```bash
curl -i http://127.0.0.1:8080/healthz
```

响应状态为 `200 OK`、body 为 `OK`，并包含 `X-Request-ID`。按 Ctrl+C 会触发信号驱动的第一阶段关闭，服务最多等待 5 秒让在途 Handler 完成；如果超时，当前实现会返回错误并由进程退出。关闭超时目前仍为硬编码值，尚未实现通过 `Server.BaseContext` 和 `Server.Close` 受控取消在途请求、汇合服务 goroutine、分类记录关闭结果及对应集成测试。代理调用示例将在对应功能实现后补充。

不指定 `-config` 时使用内置默认配置：

```bash
go run ./cmd/argusgate
```

## 当前配置

配置使用 JSON。现阶段只支持监听地址：

```json
{
  "server": {
    "address": "127.0.0.1:8080"
  }
}
```

配置加载器已经实现文件读取、严格 JSON 字段检查、额外 JSON 值检查和空监听地址校验，并通过自动测试覆盖这些主要错误路径。v0.1 后续会随阶段逐步加入 HTTP 超时、请求体上限、健康检查、上游传输和 backend 配置；配置格式以设计文档和实际实现为准，不提前承诺尚未落地的字段。

## 开发与验证

从仓库根目录执行日常检查：

```bash
go fmt ./...
go vet ./...
go test ./...
```

共享并发状态实现后，还需要执行：

```bash
go test -race ./...
```

提交前请检查变更中不包含密钥、本地配置、日志、构建产物、模型文件或个人机器路径。真实 backend 的凭据只应通过环境变量引用，不能写入示例配置或版本库。

## v0.1 路线

| 阶段 | 主题 | 可验收结果 |
| --- | --- | --- |
| Phase 0 | 基础服务 | 服务启动、探活、结构化日志和优雅退出 |
| Phase 1 | 单后端非流式代理 | 可代理真实 Chat Completions 请求 |
| Phase 2 | SSE 与取消传播 | 流式及时 flush，断连后停止上游请求 |
| Phase 3 | 多后端与健康检查 | model 路由、故障隔离、恢复和 race-free 并发 |
| Phase 4 | 可观测性 | 请求生命周期日志和 Prometheus 指标 |
| Phase 5 | 发布质量 | Docker、可复现测试与压测、v0.1 发布检查 |

完整任务、验收门槛和 Knowledge Checkpoint 见 [`docs/v0.1_roadmap.md`](./docs/v0.1_roadmap.md)。

## v0.1 边界

v0.1 聚焦一个正确、可测试的推理代理，不包含：

- 用户系统、API key 管理、RBAC 或复杂鉴权。
- 自动重试、熔断、速率限制和动态服务发现。
- 完整 OpenAI API 覆盖或 `/v1/models` 聚合。
- 配置热加载、管理控制台和 OpenTelemetry tracing。
- 模型运行、调度或内容安全审核。

在没有认证能力的情况下，服务只应部署在本机或受信任网络，不应直接暴露到公网。

## 文档索引

- [`docs/design.md`](./docs/design.md)：v0.1 的定位、范围、接口、架构和行为基线。
- [`docs/v0.1_roadmap.md`](./docs/v0.1_roadmap.md)：分阶段任务、验收条件与发布清单。
- [`docs/engineering.md`](./docs/engineering.md)：代码、测试、CI、Git 和安全规范。
- [`AGENTS.md`](./AGENTS.md)：所有 AI Agent 必须遵守的授权边界和变更约束。

当文档之间出现冲突时，系统行为以 `design.md` 为准，阶段范围以 `v0.1_roadmap.md` 为准，工程执行方式以 `engineering.md` 为准。

## 贡献约定

- 只为当前阶段创建包和抽象，不预建未来目录。
- 行为变化与对应测试一起提交。
- 影响系统边界、接口、配置、错误语义或并发所有权的修改，应先更新设计文档或补充 ADR。
- Commit Message 使用 `<类型>: <中文说明>`，例如 `feat: 增加健康检查接口`。
