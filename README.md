# ArgusGate

ArgusGate 是一个正在使用 Go 构建的 OpenAI-compatible AI 推理网关。它位于 AI 应用与独立推理服务之间，计划统一处理请求校验、后端路由、普通与 SSE 响应转发、取消传播、健康检查和可观测性。

> 当前状态：**v0.1 / Phase 0（基础服务）已完成，Phase 1（非流式单后端代理）进行中，但尚未达到阶段验收闭环**。已接入 `POST /v1/chat/completions`，复用请求校验、Proxy 和响应转发组件完成非流式单 backend HTTP 调用链；D8 的超时与故障精细分类、D9 的独立 mock backend 和真实后端 Demo 仍待完成。

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

截至当前实现，项目已完成 Phase 0 和 Phase 1 的 D7 Handler 集成。自动化测试已验证客户端经网关到测试上游的非流式调用链，Phase 1 整体验收仍待 D8、D9 完成。

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| Go module 与命令入口 | 已完成最小骨架 | module 为 `github.com/kongerly/ArgusGate` |
| JSON 配置默认值、读取与基础校验 | 已完成 Phase 0；Phase 1 配置扩展已实现 | 严格 JSON 读取；校验监听地址、关闭超时，并要求恰好一个使用合法 HTTP(S) origin 的 backend |
| 示例配置 | 已更新 | 包含监听地址、关闭超时和一个本地 backend；Authorization 留空 |
| HTTP Server 与 `/healthz` | 已完成最小版本 | 服务监听配置地址；`GET /healthz` 返回 200，其他 method 由路由拒绝 |
| Request ID 与结构化请求日志 | 已完成最小版本 | 响应包含 `X-Request-ID`，请求结束记录 method、path、status、request ID 和 duration |
| `app.Run` 生命周期边界 | 已完成 Phase 0 | App 组装 HTTP Server，返回监听错误，并在 context 取消后执行限时 `Shutdown` |
| SIGINT/SIGTERM 与优雅关闭 | 已完成 Phase 0 | 入口将信号转换为 context 取消，App 按 `server.shutdown_timeout` 等待服务关闭 |
| CI | 已完成 Phase 0 | GitHub Actions 在 push 和 pull request 时检查 format、vet 和 test；当前本地检查全部通过 |
| 请求体读取与请求探针 | 已接入 Handler | 固定 4 MiB 上限，只解析 `model` 和 `stream`；拒绝空 model 和 `stream=true`，其他字段交给 backend |
| 最小上游请求 | 已接入 HTTP API | 使用共享 Client 与请求 context，原始 body 和 model 原样转发；复用 header 过滤、网关 Request ID 和 backend 凭据设置 |
| 非流式端到端代理 | D7 已实现 | 透传上游状态、允许的 header 和 body；输入错误返回统一 JSON，提交后复制失败只记录日志；阶段 Demo 和超时精细分类待完成 |
| SSE 与客户端断连处理 | 未开始 | Phase 2 |
| 多后端路由与健康检查 | 未开始 | Phase 3 |
| Prometheus 指标 | 未开始 | Phase 4 |
| Docker 与发布验证 | 未开始 | Phase 5 |

当前代码已在 Go 1.27.1 / Windows amd64 环境通过：

```text
go test ./...
go vet ./...
gofmt -l .
```

其中 `gofmt -l .` 无输出。现有测试覆盖配置默认值、配置覆盖及主要失败路径，包含关闭超时的默认值和非法值校验；同时验证健康检查、method 限制、Request ID、请求日志，以及 `app.Run` 在 context 取消时退出和监听地址冲突时返回错误。Phase 1 测试还覆盖请求体大小边界、请求探针解析与校验、上游 POST 原始 body 保留、context 取消后停止等待上游响应、hop-by-hop header 过滤、多值 header 保留，以及客户端认证和 Request ID 不会绕过网关托管规则。

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

响应状态为 `200 OK`、body 为 `OK`，并包含 `X-Request-ID`。按 Ctrl+C 会触发信号驱动的优雅关闭，服务按照 `server.shutdown_timeout` 等待在途 Handler 完成；默认等待 5 秒，超时后返回错误并由进程退出。配置中的 backend origin 会在启动时校验；`POST /v1/chat/completions` 将非流式请求发送到该 origin 的同名路径，需要自行启动兼容 backend。

不指定 `-config` 时使用内置默认配置：

```bash
go run ./cmd/argusgate
```

## 当前配置

配置使用 JSON。当前支持监听地址、关闭超时和恰好一个 backend：

```json
{
  "server": {
    "address": "127.0.0.1:8080",
    "shutdown_timeout": "5s"
  },
  "backends": [
    {
      "origin": "http://127.0.0.1:8081",
      "authorization": ""
    }
  ]
}
```

`shutdown_timeout` 使用 Go duration 格式，例如 `5s`、`500ms` 或 `1m`。`backends` 当前必须恰好包含一项；`origin` 必须是没有凭据、业务路径、query 或 fragment 的 HTTP(S) origin。`authorization` 当前作为完整 header 值原样设置到上游请求；留空时不设置该 header。环境变量引用尚未实现；需要在本机联调时，真实凭据只能放在不跟踪的本地配置文件中，绝不能提交。客户端 Authorization 默认丢弃。

配置加载器还会拒绝未知字段、多个顶层 JSON 值和非法配置；配置扩展的校验路径由 `internal/config` 测试覆盖。请求体目前固定限制为 4 MiB，尚无对应配置项；其余 HTTP 超时、健康检查和传输参数仍按路线图逐步加入。D7 上游请求失败暂统一返回安全的 502 JSON；408/502/504 精细分类与超时策略由 D8 完成。

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

提交前请检查变更中不包含密钥、本地配置、日志、构建产物、模型文件或个人机器路径。示例中的 `authorization` 保持为空；当前配置尚不支持环境变量引用，因此不要把真实凭据写入示例配置或任何提交文件。

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
