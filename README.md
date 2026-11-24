# ArgusGate

ArgusGate 是一个使用 Go 构建的 OpenAI-compatible AI 推理网关。它位于 AI 应用与 llama.cpp、vLLM 等独立推理服务之间，本身不运行模型，致力于统一承担请求校验、后端路由、普通与 SSE 响应转发、取消传播、健康检查和可观测性。

> [!NOTE]
> **当前状态**：**v0.1 / Phase 0（基础服务）已完成，Phase 1（非流式单后端代理）进行中，尚未达到阶段验收闭环**。目前已接入 `POST /v1/chat/completions` 非流式单 backend 代理主链路；超时策略与 `408`/`502`/`504` 精细故障分类、独立 mock backend 以及真实后端联调 Demo 仍待完成。

## 项目定位与架构

客户端只需访问 ArgusGate 提供的统一 OpenAI-compatible 入口，模型推理仍由独立的 inference backend 进程承担：

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

**v0.1 计划达成目标：**

- 提供 `POST /v1/chat/completions`，透明转发普通 JSON 与 SSE 流式响应。
- 在客户端断开后取消上游请求，避免继续占用推理资源。
- 按 `model` 精确匹配后端，并在健康实例之间进行 round-robin 路由。
- 自动隔离异常后端，并在恢复后重新加入路由。
- 提供 `/healthz`、`/readyz`、`/metrics` 与结构化请求日志。
- 支持本地运行、自动化测试、`race test` 与 Docker 运行。

## 当前进度

| 阶段 / 模块 | 状态 | 当前能力说明 |
| --- | --- | --- |
| Phase 0：基础服务 | 已完成 | 包含 `github.com/kongerly/ArgusGate` 命令入口、JSON 配置加载与严格校验、HTTP Server 与 `GET /healthz`、`X-Request-ID` 生成与结构化请求日志、`app.Run` 生命周期与 `SIGINT`/`SIGTERM` 优雅关闭，以及 GitHub Actions CI。 |
| Phase 1：单 backend 配置 | 已完成 | 校验监听地址与关闭超时，要求恰好配置 1 个合法 HTTP(S) origin 的 backend（含可选 `authorization`）；示例配置已同步。 |
| Phase 1：请求校验与探针 | 已接入 | `POST /v1/chat/completions` 校验请求方法与 `application/json` 类型，应用固定 4 MiB 请求体上限，仅解析 `model` 与 `stream`，拒绝空 `model` 与 `stream=true`，其余字段保留原始字节。 |
| Phase 1：非流式代理转发 | 已接入 | 复用共享 `http.Client`（禁止自动跟随 3xx 重定向）与请求 context，过滤 hop-by-hop 及网关托管 header，显式设置 `X-Request-ID` 与 backend `Authorization`，透传上游状态码、允许的 header 与响应体；提交前返回统一 JSON 错误，提交后失败仅记录日志。 |
| Phase 1：阶段收尾项 | 待完成 | 超时策略与 `408`/`502`/`504` 精细故障分类（当前上游失败暂统一返回安全 `502` JSON）、独立 mock backend 以及真实后端 Demo 尚未完成。 |
| Phase 2：SSE 与取消传播 | 未开始 | SSE 实时 flush、TTFB 统计、客户端断连取消传播与强制关闭分类。 |
| Phase 3：多后端与健康检查 | 未开始 | Backend Registry、状态机、后台周期探测、多后端 round-robin 与 `GET /readyz`。 |
| Phase 4：可观测性与指标 | 未开始 | 请求全生命周期统一日志与 Prometheus `GET /metrics`。 |
| Phase 5：Docker 与发布验证 | 未开始 | Dockerfile、可复现压测与 v0.1 发布检查。 |

现有自动化测试覆盖配置默认值与严格解析校验、`/healthz` 与 method 限制、`X-Request-ID` 与请求日志、`app.Run` 优雅关闭与监听冲突处理。针对 `POST /v1/chat/completions` 的测试还覆盖 4 MiB 请求体边界与探针校验、原始请求体保留、context 取消传播、hop-by-hop 与网关托管 header 隔离（防止客户端认证与 Request ID 绕过）、多值 header 保留，以及上游响应透传与提交后错误处理。

## 快速开始

### 环境要求

- Go 1.27.3 或更新的受支持安全补丁版本（最低版本以 [`go.mod`](./go.mod) 的 `go` directive 为准）
- Git

> [!IMPORTANT]
> 2026-10-22 将最低 Go 版本提升至 1.27.3，以覆盖标准库 HTTP/2 响应处理和证书验证等安全修复，见 [Go 发布记录](https://go.dev/doc/devel/release#go1.27.3)、[GO-2026-6610](https://pkg.go.dev/vuln/GO-2026-6610) 和 [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)。已有二进制必须用修复版工具链重新构建并在部署时替换、重启；仅升级本机 Go 或修改 `go.mod` 不会修复旧产物。更新后，使用 `go version -m <binary>` 核验构建版本。

### 启动服务

克隆仓库后，在项目根目录使用示例配置启动：

```bash
go version
go mod download
go run ./cmd/argusgate -config ./configs/argusgate.example.json
```

不指定 `-config` 时，服务使用内置默认配置启动：

```bash
go run ./cmd/argusgate
```

预期启动日志类似：

```text
time=... level=INFO msg="starting HTTP server" address=127.0.0.1:8080
```

按 `Ctrl+C` 或发送 `SIGTERM` 会触发信号驱动的优雅关闭，服务按 `server.shutdown_timeout`（默认 `5s`）等待在途请求完成，超时后返回错误并退出进程。

### 接口验证

在另一个终端验证探活接口：

```bash
curl -i http://127.0.0.1:8080/healthz
```

正常返回状态码 `200 OK`、响应体 `OK`，并携带网关生成的 `X-Request-ID`。

如需验证 `POST /v1/chat/completions` 非流式代理，请先在配置指定的 backend origin（默认为 `http://127.0.0.1:8081`）启动兼容 OpenAI API 的推理后端，再发送请求：

```bash
curl -i -X POST http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen-local","messages":[{"role":"user","content":"Hello"}]}'
```

- 网关会将原始请求体转发到配置 backend origin 的 `/v1/chat/completions` 路径，并透传上游状态码、允许的响应头与响应体。
- 当前阶段仅支持非流式请求（省略 `stream` 或设为 `false`）；传入 `"stream": true` 会返回 `400`（`stream_not_supported`）。
- 请求体固定限制为 4 MiB，超出返回 `413`（`request_too_large`）。

## 配置说明

当前使用静态 JSON 配置（见 [`configs/argusgate.example.json`](./configs/argusgate.example.json)）：

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

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `server.address` | `127.0.0.1:8080` | HTTP 服务监听地址，不能为空。 |
| `server.shutdown_timeout` | `5s` | 优雅关闭等待在途请求的超时时间，使用 Go duration 格式（如 `500ms`、`5s`、`1m`）。 |
| `backends[].origin` | `http://127.0.0.1:8081` | 上游 HTTP(S) origin；当前 `backends` 必须恰好包含 1 项，且禁止包含 userinfo、业务路径（仅允许为空或 `/`）、query 或 fragment。 |
| `backends[].authorization` | `""` | 可选；非空时作为完整 `Authorization` header 值原样发往上游，留空时不设置该 header。 |

**配置规则与当前限制：**

- **严格校验**：加载配置时会拒绝未知字段、多个顶层 JSON 值以及非法字段值。
- **凭据安全**：客户端请求的 `Authorization` 默认丢弃，不会透传给上游。当前尚未支持环境变量引用，示例配置保持留空；本地联调如需真实凭据，请仅放在未跟踪的本地配置文件中，切勿提交到版本库。
- **阶段限制**：请求体大小目前固定为 4 MiB（暂未开放配置项）；其余 HTTP 超时、健康检查、多后端与传输参数将在后续阶段按路线图加入。上游请求失败当前暂统一返回安全的 `502` JSON，`408`/`502`/`504` 的精细分类将在 Phase 1 后续任务中完善。

## 开发与验证

在仓库根目录执行日常格式、静态检查与测试：

```bash
go fmt ./...
go vet ./...
go test ./...
```

后续引入共享并发状态后（Phase 3），还需执行：

```bash
go test -race ./...
```

GitHub Actions CI 会在 `push` 和 `pull_request` 时读取 [`go.mod`](./go.mod) 安装对应工具链（`GOTOOLCHAIN=local`），检查 `gofmt -l .`、`go vet ./...` 和 `go test ./...`。提交前请确认变更中不包含密钥、本地配置、日志、构建产物、模型文件或个人机器路径。

## v0.1 路线与边界

| 阶段 | 状态 | 主题 | 可验收结果 |
| --- | --- | --- | --- |
| Phase 0 | 已完成 | 基础服务 | 服务启动、探活、结构化日志和优雅退出 |
| Phase 1 | 进行中 | 单后端非流式代理 | 可代理真实 Chat Completions 请求 |
| Phase 2 | 未开始 | SSE 与取消传播 | 流式及时 flush，断连后停止上游请求 |
| Phase 3 | 未开始 | 多后端与健康检查 | model 路由、故障隔离、恢复和 race-free 并发 |
| Phase 4 | 未开始 | 可观测性 | 请求生命周期日志和 Prometheus 指标 |
| Phase 5 | 未开始 | 发布质量 | Docker、可复现测试与压测、v0.1 发布检查 |

完整任务、验收门槛和 Knowledge Checkpoint 见 [`docs/v0.1_roadmap.md`](./docs/v0.1_roadmap.md)。

### v0.1 边界

v0.1 聚焦一个正确、可测试的推理代理，不包含以下能力：

- 用户系统、API key 管理、RBAC 或复杂鉴权
- 自动重试、熔断、速率限制与动态服务发现
- 完整 OpenAI API 覆盖或 `/v1/models` 聚合
- 配置热加载、管理控制台与 OpenTelemetry tracing
- 模型运行、调度或内容安全审核

> [!WARNING]
> 在没有认证能力的情况下，v0.1 服务只应部署在本机或受信任网络，不应直接暴露到公网。

## 文档索引与贡献约定

- [`docs/design.md`](./docs/design.md)：v0.1 的定位、范围、接口、架构和行为基线。
- [`docs/v0.1_roadmap.md`](./docs/v0.1_roadmap.md)：分阶段任务、验收条件与发布清单。
- [`docs/engineering.md`](./docs/engineering.md)：代码、测试、CI、Git 和安全规范。
- [`AGENTS.md`](./AGENTS.md)：所有 AI Agent 必须遵守的授权边界和变更约束。

当文档之间出现冲突时，系统行为以 `design.md` 为准，阶段范围以 `v0.1_roadmap.md` 为准，工程执行方式以 `engineering.md` 为准。

**贡献约定：**

- 只为当前阶段创建包和抽象，不预建未来目录。
- 行为变化与对应测试一起提交。
- 影响系统边界、接口、配置、错误语义或并发所有权的修改，应先更新设计文档或补充 ADR。
- Commit Message 使用 `<类型>: <中文说明>`，例如 `feat: 增加健康检查接口`。
