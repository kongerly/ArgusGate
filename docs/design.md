# ArgusGate Design

> 使用 Go 构建的 OpenAI-compatible AI Inference Gateway

| 属性 | 值 |
| --- | --- |
| Version | v0.1 |
| Status | Draft |
| Audience | 项目作者与未来贡献者 |
| Source | [前期讨论](https://chatgpt.com/share/6aa0f1a5-964c-83e8-adde-dfa3f23fea78) |

本文是 ArgusGate v0.1 的唯一设计基线，统一描述项目定位、范围、接口、架构、关键行为和测试约束。具体排期、开发任务和发布清单见 [`v0.1_roadmap.md`](./v0.1_roadmap.md)。设计会随实现演进；影响系统边界或行为语义的变化，应先更新本文或补充 ADR。

## 1. 项目概述

ArgusGate 位于 AI 应用与模型推理服务之间。客户端只访问一个 OpenAI-compatible 地址，ArgusGate 负责选择可用后端、转发请求、返回普通或流式响应，并记录请求生命周期。

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

ArgusGate 不运行模型。llama.cpp、vLLM 或其他兼容服务始终作为独立 inference backend 存在。

### 1.1 名称含义

Argus 来自希腊神话中的百眼守望者 Argus Panoptes，代表观察、守望和全局感知，对应 Health Check、Metrics、Logging 与 Request Tracking。

Gate 代表入口、边界和控制层，对应 API Gateway、Routing 与 Traffic Control。

因此，ArgusGate 的设计语言是：

> 一个持续观察并管理 AI 推理流量的入口层。

### 1.2 为什么做这个项目

项目有两个同等重要的目标：

1. 构建一个可运行、可测试、可连接真实模型服务的独立 inference gateway。
2. 通过完整工程实践学习 Go 后端、并发、网络服务和 AI Infrastructure 基础。

它不是商业 AI 平台，也不是为了堆叠技术栈的演示工程。每个组件都应由真实需求驱动，并能解释“解决了什么问题、为什么这样设计、如何验证正确”。

## 2. 目标与设计原则

### 2.1 产品目标

- 为客户端提供统一的 OpenAI-compatible 推理入口。
- 正确代理普通 JSON 响应和 SSE 流式响应。
- 在客户端断开后取消上游请求，避免推理资源继续消耗。
- 在多个后端之间按 model 和健康状态进行确定性路由。
- 自动隔离异常后端，并在其恢复后重新加入路由。
- 通过结构化日志和指标回答请求去了哪里、耗时多久、为何失败。
- 支持本地直接运行、自动测试和 Docker 运行。

### 2.2 学习目标

完成 v0.1 后，应能结合代码解释：

- Go module、package、interface 和 error 的工程用法。
- `net/http` 中 Server、Handler、Client 与 Transport 的生命周期。
- `context.Context` 如何沿调用链传播取消信号。
- SSE 为什么需要 flush，缓冲可能发生在哪些层。
- goroutine、ticker、channel、mutex 和 atomic 的适用边界。
- 如何安全维护 backend 的健康和请求状态。
- 如何用 `httptest`、race detector、benchmark 和 pprof 验证服务。
- 如何设置项目边界、组织提交并完成一个可发布版本。

### 2.3 设计原则

1. **正确优先**：先定义行为和失败语义，再考虑性能。
2. **简单优先**：不为尚未出现的需求提前建立复杂抽象。
3. **可观测优先**：日志和指标从请求生命周期设计开始就存在。
4. **需求驱动**：包、接口和并发结构随真实功能出现而演进。
5. **标准库优先**：第一阶段使用 `net/http`、`log/slog` 等标准库理解底层行为。
6. **可验证交付**：每个阶段都必须形成可运行、可测试的闭环。

### 2.4 学习优先级

设计描述的是 v0.1 最终应具备的行为，不代表所有内容都要在项目开始时一次性掌握。

#### Must Understand

这些知识会直接影响正确性，应随对应 Phase 掌握并能够解释：

- Go module、package、struct、method 和 error
- `net/http` 请求与响应生命周期
- `context` 传播、取消和 deadline
- 请求 body、响应 body 与资源关闭
- SSE、buffer 和 Flush
- goroutine 生命周期与 owner
- mutex、共享状态和 race detector
- `httptest` 与外部可观察行为测试

#### Understand Later

这些内容只在项目出现对应需求时深入，不作为开工前置知识：

- Transport 连接池调优
- Prometheus label cardinality 与 histogram 细节
- benchmark、pprof 和系统级性能分析
- OpenTelemetry
- 熔断、有限重试和高级路由策略
- Kubernetes 与动态服务发现

学习顺序服从 [`v0.1_roadmap.md`](./v0.1_roadmap.md)。不因文档出现一个术语，就在当前 Phase 提前实现它。

## 3. v0.1 范围

### 3.1 必须完成

- `POST /v1/chat/completions`
- 非流式透明代理
- SSE 流式透明代理
- Request ID 生成、传播与回传
- 客户端取消和上游阶段超时
- 静态配置的多个 backend
- 按 model 精确匹配 backend
- healthy backend 间 round-robin
- 后台健康检查与自动恢复
- `/healthz`、`/readyz`、`/metrics`
- 结构化请求日志和基础 Prometheus 指标
- 优雅关闭及超时后的强制取消
- 单元测试、集成测试、流式测试和 race test
- Dockerfile、配置示例、README、架构说明和可复现压测记录

### 3.2 明确不做

- RAG、Agent、Prompt、Conversation 或 Knowledge Base 管理
- 模型下载、模型加载、模型进程托管、GPU 或 CUDA 管理
- 用户系统、API key 管理、RBAC 或复杂鉴权
- 数据库、管理后台或控制面
- 动态注册、etcd、Consul 或 Kubernetes
- 内容、成本或延迟感知的智能路由
- 请求重试、熔断、限流或配额
- 完整覆盖 OpenAI API
- 分布式 tracing

重试暂不实现是有意决策。生成请求不一定幂等，而且一旦向客户端写出任何响应字节，就不能安全切换上游。后续只能在明确失败阶段和重试条件后加入有限重试。

### 3.3 完成定义

只有同时满足以下条件才发布 v0.1：

1. OpenAI SDK 或 `curl` 能通过 ArgusGate 调用至少一种真实后端。
2. 普通响应和 SSE 路径都有自动化集成测试。
3. 客户端取消后，上游测试服务能观测到 context 取消。
4. 异常 backend 达到阈值后不再被选择，恢复后能自动重新加入。
5. `go test ./...`、`go test -race ./...` 与 `go vet ./...` 通过。
6. 服务停止接收新请求，在宽限期内等待在途请求，超时后能够强制退出。
7. 新读者能依据 README 启动、调用和验证服务。
8. 所有公开性能数字都有仓库内的环境说明、命令和原始结果支撑。

## 4. 外部接口

### 4.1 数据面

#### `POST /v1/chat/completions`

- 请求体沿用 OpenAI Chat Completions JSON。
- 网关只解析路由必需字段：`model` 和 `stream`。
- `model` 必须存在，并与至少一个配置 backend 的 model 精确匹配。
- 请求体默认限制为 4 MiB，限制值可配置。
- 其他字段不重新定义、不修改，按原始字节转发。
- 上游返回的状态码、JSON 或 `text/event-stream` 内容尽量原样传递。

请求 body 是一次性流。Handler 必须在大小限制内读取原始字节，从副本解析 `model` 和 `stream`，再使用原始字节重建上游 body。禁止把解析后的 JSON 重新编码后转发，以免改变数字、空白或未知字段。

v0.1 不实现完整 OpenAI schema 校验。网关只验证自身真正依赖的内容，将业务字段校验交给 inference backend。

### 4.2 运维接口

- `GET /healthz`：进程存活即返回 200，不检查 backend。
- `GET /readyz`：仅当应用处于 `RUNNING`、配置已加载且至少存在一个 healthy backend 时返回 200；进入 `DRAINING` 或条件不满足时返回 503。
- `GET /metrics`：返回 Prometheus 文本格式指标。

### 4.3 请求与响应约定

- 接受客户端 `X-Request-ID`；缺失时生成新 ID。
- 响应始终回传最终采用的 `X-Request-ID`。
- 网关采用的 Request ID 覆盖客户端发往上游的同名 header。
- 上游请求 `Host` 使用目标 backend 的 host，客户端不能覆盖。
- 过滤固定 hop-by-hop headers，并解析 `Connection` 中动态声明的 header 名后一起移除。
- 配置 `api_key_env` 时，用对应环境变量重建上游 `Authorization`。
- 未配置 backend key 时，只有显式启用 `forward_client_authorization` 才透传客户端认证，默认丢弃。
- v0.1 不转发 HTTP trailers；出现真实需要并补齐测试后再启用。
- 网关永不记录认证信息、完整 prompt 或完整响应。

在响应开始前发生的网关错误使用统一 JSON 格式。上游响应一旦开始写入，后续错误只能终止连接并记录，不能改写为新的 JSON 错误。

```json
{
  "error": {
    "message": "no healthy backend for model",
    "type": "gateway_unavailable",
    "code": "no_healthy_backend",
    "request_id": "req_..."
  }
}
```

### 4.4 Security Boundary

```text
Untrusted Client
      |
      | validate input, filter headers, limit resources
      v
+-------------------+
| ArgusGate         |
| trust boundary    |
+-------------------+
      |
      | controlled backend URL and credentials
      v
Inference Backend
```

ArgusGate v0.1 负责：

- 将所有客户端请求视为不可信输入。
- 校验 method、content type、body 大小、model 和目标路径。
- 过滤 hop-by-hop headers，禁止客户端覆盖 backend host。
- 只向已配置、已校验的 backend origin 发起请求。
- 禁止 HTTP Client 自动跟随 redirect，避免 3xx 把请求和凭据带到未配置 origin。
- 控制上游连接、TLS、响应头等待和健康探测的超时。
- 保护 backend secret，不写入日志、响应或版本库。
- 限制日志和指标中的敏感内容与高基数字段。

ArgusGate v0.1 不负责：

- 用户身份认证与授权。
- 多租户隔离、配额和计费。
- backend 自身安全、模型安全或输出内容审核。
- 公网 TLS 终止与边缘防护；正式公网部署应由受信任的反向代理或平台承担。

“v0.1 不做用户认证”不表示客户端可信，只表示认证能力不在当前版本范围内。默认部署边界是本机或受信任网络，不应把未配置认证的服务直接暴露到公网。

## 5. 配置设计

v0.1 使用 JSON 配置，避免仅为 YAML 引入依赖。启动参数只指定配置文件位置：

```text
argusgate -config ./configs/argusgate.json
```

建议配置：

```json
{
  "server": {
    "address": ":8080",
    "read_header_timeout": "5s",
    "read_timeout": "30s",
    "idle_timeout": "60s",
    "max_header_bytes": 1048576,
    "shutdown_timeout": "5s",
    "max_request_body_bytes": 4194304
  },
  "health_check": {
    "interval": "5s",
    "timeout": "2s",
    "max_concurrency": 8,
    "failure_threshold": 3,
    "success_threshold": 2
  },
  "upstream": {
    "connect_timeout": "3s",
    "tls_handshake_timeout": "3s",
    "response_header_timeout": "30s",
    "idle_conn_timeout": "90s"
  },
  "backends": [
    {
      "name": "local-llama",
      "base_url": "http://127.0.0.1:8081",
      "chat_completions_path": "/v1/chat/completions",
      "models": ["qwen-local"],
      "health_path": "/health",
      "api_key_env": "LLAMA_API_KEY",
      "forward_client_authorization": false
    }
  ]
}
```

配置规则：

- 默认值只在 `config` 包定义。
- 配置文件不存在、JSON 非法、backend 重名、URL 非法或 models 为空时启动失败。
- `base_url` 只表示 origin：必须是绝对 HTTP(S) URL，只允许 scheme 和 host，path 只能为空或 `/`，禁止 userinfo、query 和 fragment。
- `chat_completions_path` 与 `health_path` 必须以单个 `/` 开头，禁止 `//`、反斜杠、userinfo、query 和 fragment。
- 目标 URL 只能通过复制已经校验的 backend origin，再设置解析后的 `Path`/`RawPath` 构造；禁止使用可能把 `//host/path` 解释为新 authority 的 reference resolution。
- v0.1 不接受客户端 query 参数，也不把它们转发到 backend。
- secret 只通过环境变量名引用，不直接写入配置文件。
- duration 在加载阶段校验为 `time.Duration` 可解析格式，在使用边界解析为实际超时值。
- v0.1 不支持热加载，修改后需要重启服务。
- 不为整个生成请求设置固定 `http.Client.Timeout`。
- 连接、TLS 和等待响应头分别设置上限；长时间响应体读取由客户端 context 控制。
- `read_timeout` 只约束客户端上传请求（包含 body）的时间，不约束上游生成和下游响应流。
- `max_header_bytes` 限制客户端请求 header；上游 header 限制依赖 Go Transport，发现真实需求后再设计额外保护。
- `ReadHeaderTimeout` 在 Handler 执行前由 `net/http.Server` 处理，超时时不保证进入 middleware 或返回统一 JSON；`read_timeout` 导致的 body 读取超时才由 Handler 在仍可写时映射为 408。

## 6. 系统架构

```text
                         +-------------------+
Client ---------------->| HTTP API          |
                         | request ID/error  |
                         +---------+---------+
                                   |
                                   v
                         +-------------------+
                         | Router            |
                         | model + health    |
                         +---------+---------+
                                   |
                                   v
                         +-------------------+
                         | Backend Registry  |<---- Health Checker
                         | state + in-flight |
                         +---------+---------+
                                   |
                                   v
                         +-------------------+
                         | Proxy             |
                         | HTTP + SSE        |
                         +---------+---------+
                                   |
                         +---------+---------+
                         |                   |
                         v                   v
                    llama.cpp              vLLM

Observability 横切整个请求生命周期，但不参与路由决策。
```

### 6.1 目录结构

```text
ArgusGate/
├── cmd/
│   └── argusgate/
│       └── main.go
├── internal/
│   ├── app/            # 依赖组装、启动与关闭
│   ├── config/         # 默认值、加载与校验
│   ├── httpapi/        # 路由、middleware、错误响应
│   ├── backend/        # Backend、Registry、动态状态
│   ├── routing/        # model 匹配与 round-robin
│   ├── proxy/          # HTTP 与 SSE 转发
│   ├── healthcheck/    # 周期探测与状态转换
│   └── observability/  # slog 与 metrics
├── configs/
│   └── argusgate.example.json
├── docs/
├── go.mod
├── Dockerfile
└── README.md
```

目录按需求逐步创建，不在第一天生成全部空包。暂不建立 `pkg/`，因为 v0.1 没有承诺给外部程序复用的 Go API。

### 6.2 组件职责

#### App

负责组装配置、Registry、Router、Proxy、Health Checker 和 HTTP Server，并统一拥有后台 goroutine、`RUNNING`/`DRAINING` 状态与关闭顺序。业务组件不自行读取全局配置或捕获系统信号。

#### HTTP API

负责 endpoint、Request ID、输入限制、最小字段解析和统一错误响应。Handler 不直接管理 backend 状态，也不实现路由算法。

#### Backend

静态字段包括 name、base URL、models、health path 和认证配置；启动后保持只读。动态字段包括 health、连续成功/失败次数和 active requests，由 Registry 封装更新。

#### Registry

维护 backend 及其动态状态，对外提供只读快照和受控更新。调用方不得直接修改共享状态。v0.1 先使用 `sync.RWMutex` 保证 race-free；只有在测量证明有必要时才调整锁粒度或使用 atomic。

#### Router

输入 model 和 Registry 的 eligible backend 快照，输出一个 backend 或明确错误。Router 不发送 HTTP 请求、不执行健康探测，也不写业务日志。

最小抽象可以是：

```go
type Selector interface {
    Select(model string) (*backend.Backend, error)
}
```

只有当测试替身或第二个实现确有价值时才保留接口，否则优先使用具体类型。

#### Proxy

拥有共享 `http.Client` 和配置良好的 `http.Transport`。禁止每次请求创建 Client。Proxy 构造上游请求、处理 header、发送请求、转发普通或流式响应，并返回可供日志和指标使用的结果。

`http.Client.Do` 成功后必须立即 `defer response.Body.Close()`，所有正常、错误和取消路径都必须执行。完整复制 body 后允许连接复用；取消或中途错误时关闭 body，不为复用而无限 drain 异常响应。

Client 必须设置 `CheckRedirect` 返回 `http.ErrUseLastResponse`，不自动访问 `Location` 指向的地址。数据面把 backend 的 3xx 原样返回客户端；健康检查把 3xx 视为失败。该规则同时适用于代理和健康探测，防止 backend credential 被带到未配置 origin。

#### Health Checker

在 `app` 管理的 goroutine 中周期运行。每次探测有独立 timeout，每轮并发数有上限。服务关闭时停止调度并等待退出。

#### Observability

定义统一日志字段、请求结果和指标记录方式。它观察组件行为，但不改变路由和错误处理结果。

### 6.3 Package Dependency Rules

```text
cmd/argusgate
      |
      v
internal/app -----------------------------+
      |                                   |
      +--> config                         |
      +--> httpapi --> routing --> backend|
      +--> httpapi --> proxy ------------>|
      +--> healthcheck --> backend         |
      +--> observability                   |
                                          |
基础组件不得反向依赖 app、cmd 或 httpapi <-+
```

依赖规则：

- `cmd/argusgate` 只负责进程入口、参数和退出码。
- `app` 是 composition root，可以依赖所有内部组件并负责组装。
- `httpapi` 可以依赖窄接口，不负责创建具体 Router、Proxy 或 Registry。
- `routing` 只依赖 backend 的只读模型，不依赖 HTTP handler 或 Proxy。
- `proxy` 不依赖 Router；它只接受已经选定的 backend 和请求数据。
- `backend` 不依赖 routing、proxy、httpapi 或 app。
- `healthcheck` 通过 Registry 的受控方法更新状态，不直接访问 handler。
- `observability` 提供通用记录能力，不导入业务上层包。
- 发现循环依赖时先重新检查职责归属，不通过复制类型或创建“common”杂物包绕过问题。
- 接口定义在使用方附近；只有出现真实替身或多个实现时才引入。

## 7. 请求生命周期

```text
Receive
  -> Identify
  -> Validate
  -> Select
  -> Account in-flight
  -> Forward
  -> Stream/Copy
  -> Finish or Cancel
  -> Observe
```

具体步骤：

1. HTTP Server 接收请求并记录开始时间。
2. 采用或生成 Request ID，写入 context 和响应 header。
3. 检查 method、content type 与 body 上限。
4. 缓存原始 body，并从副本解析 `model` 和 `stream`。
5. Router 从匹配 model 的 healthy backend 中选择目标。
6. Registry 增加目标 backend 的 active requests，并用 `defer` 保证所有出口都减少。
7. Proxy 以原始 body 重建 reader，使用 handler context 创建上游请求。
8. 上游返回后，先复制允许的响应 header，再写状态码。
9. 普通响应使用受控 buffer copy；SSE 响应逐 chunk 写入并及时 flush。
10. 客户端断开或 deadline 到达时，handler context 取消上游请求。
11. 对上游 response body 注册 `defer Close`，确保所有出口释放连接资源。
12. 完成后记录 backend、状态、错误类型、耗时、传输字节和首字节时间。

### 7.1 SSE 约束

- 以上游实际 `Content-Type: text/event-stream` 决定是否进入 SSE 路径。
- 请求中的 `stream=true` 只表达客户端预期，不能把上游 JSON 错误误判为 SSE。
- 在写响应头前确认当前 `ResponseWriter` 支持 flush。
- 包装 `ResponseWriter` 的 middleware 必须保留或正确暴露 flush 能力。
- v0.1 不解析、不重组 SSE event，只做字节级转发。
- 首次成功写入下游 body 的时间记为网关观测的 TTFB；SSE 每次成功写入后立即 flush。
- TTFB 不是模型内部 TTFT；文档和指标不得混用两个概念。
- 上游响应一旦开始，流中错误只能终止连接并记录，不能更改 HTTP 状态。

### 7.2 取消与超时

- 客户端断开由 `request.Context()` 传播到上游请求。
- 连接和 TLS 建立失败返回 502。
- 等待上游响应头超时返回 504。
- 客户端已取消时不再尝试写网关 JSON 错误。
- 不为整个生成过程设置固定总时限，避免截断长时间推理流。
- 后续如需要服务端生成时限，应作为显式配置并与客户端 context 组合。

v0.1 的取消分类采用以下优先级，避免把服务关闭误记为客户端取消：

1. 强制关闭 context 的 cause 为 `shutdown_forced` 时，记录 `shutdown_canceled`。
2. 其余 handler context 取消记录 `client_canceled`。

App 使用 `context.WithCancelCause` 为强制关闭设置可识别原因。未来新增服务端生成总时限时，必须设置独立 cause，并新增不同于等待响应头 `upstream_timeout` 的错误类别。

### 7.3 关闭顺序

`http.Server.Shutdown` 会停止接收新连接并等待 handler，但不会主动取消仍在运行的 handler。ArgusGate 使用两阶段关闭：

1. 收到 SIGINT/SIGTERM 后，App 立即进入 `DRAINING`，使 `/readyz` 返回 503；随后停止 Health Checker 新一轮调度，并以 `shutdown_timeout` 调用 `Server.Shutdown`。
2. 如果 handler 在宽限期内完成，关闭共享 Transport，取消应用根 context，等待后台 goroutine 退出。
3. 如果宽限期耗尽，取消作为 `Server.BaseContext` 父级的强制关闭 context，使在途上游请求收到取消；然后调用 `Server.Close`、关闭 Transport 并等待组件退出。

应用必须显式拥有并等待自己创建的 goroutine，不能只依赖进程退出回收资源。

## 8. 路由、健康与并发

### 8.1 v0.1 路由

```text
exact model match
  -> filter HEALTHY backends
  -> round-robin among eligible backends
  -> no candidate: 503 no_healthy_backend
```

每个 model 使用独立 round-robin 游标，避免不同 model 的流量相互扰动。Least-in-flight 延后到 v0.2，等待 active requests 统计通过并发验证后再加入。

### 8.2 健康状态机

```text
UNKNOWN --success_threshold--> HEALTHY
UNKNOWN --failure_threshold--> UNHEALTHY
HEALTHY --failure_threshold--> UNHEALTHY
UNHEALTHY --success_threshold--> HEALTHY
```

- 启动时所有 backend 为 UNKNOWN，不参与路由。
- 启动后立即执行首轮探测，之后按 interval 运行。
- 使用 `GET health_path` 和空 body，任何 2xx 都视为成功。
- 网络错误、超时和非 2xx 都视为失败。
- 成功清零连续失败次数，失败清零连续成功次数。
- 每轮使用有上限的并发，默认最多同时探测 8 个 backend。
- 默认 `success_threshold=2` 时，readiness 最早在第二次成功探测后成立。
- 只在状态变化时写健康日志，周期性成功不重复刷屏。

### 8.3 并发状态所有权

主要并发来源：

- `net/http` 为请求启动的并发 handler。
- Health Checker 后台循环及有限并发探测。
- 请求结束、健康探测和 Router 同时访问 backend 状态。

并发规则：

- 静态 backend 配置启动后不可变。
- 动态状态只能通过 Registry 读写。
- 不在持锁期间执行网络 I/O。
- Router 使用一致的只读快照做一次选择。
- active requests 的增减必须成对，失败路径同样执行。
- goroutine 必须有明确 owner、停止信号和等待机制。
- 在引入并发状态后持续运行 race test，而非发布前一次性检查。

### 8.4 State Model

#### Application State

```text
STARTING
   |
   | config valid, server listening, startup checks scheduled
   v
RUNNING
   |
   | SIGINT / SIGTERM / fatal serve error
   v
DRAINING
   |
   | graceful completion or forced cancellation
   v
STOPPED
```

- `STARTING`：配置和依赖正在初始化，readiness 为 503。
- `RUNNING`：服务接受数据面请求；至少一个 backend HEALTHY 时 readiness 为 200。
- `DRAINING`：停止接受新流量，readiness 立即变为 503，在途请求进入宽限期。
- `STOPPED`：listener、Transport 和应用拥有的 goroutine 均已结束。
- v0.1 不支持从 `DRAINING` 返回 `RUNNING`。

#### Backend State

```text
                success threshold
UNKNOWN --------------------------------> HEALTHY
   |                                         |
   | failure threshold                       | failure threshold
   v                                         v
UNHEALTHY <------------------------------ HEALTHY
   |
   | success threshold
   +-------------------------------------> HEALTHY
```

- Backend 状态只由 Health Checker 通过 Registry 更新。
- Router 只读取状态，不触发探测或改变状态。
- active requests 是负载统计，不改变健康状态。
- 状态转换必须伴随一次结构化日志；重复状态不重复产生日志。

## 9. 错误与可观测性

### 9.1 错误分类

响应尚未提交时，网关返回统一 JSON，其中 `type` 表示稳定的错误大类，`code` 表示可供客户端判断的具体原因：

| 场景 | HTTP | Public `type` | Public `code` / internal result |
| --- | ---: | --- | --- |
| method 不支持 | 405 | `invalid_request_error` | `method_not_allowed` |
| content type 不支持 | 415 | `invalid_request_error` | `unsupported_media_type` |
| body 超过限制 | 413 | `invalid_request_error` | `request_too_large` |
| JSON 非法 | 400 | `invalid_request_error` | `invalid_json` |
| model 缺失 | 400 | `invalid_request_error` | `missing_model` |
| Handler 读取 request body 超时 | 408 | `invalid_request_error` | `request_timeout` |
| 未配置请求 model | 404 | `not_found_error` | `model_not_found` |
| model 存在但无 healthy backend | 503 | `gateway_unavailable` | `no_healthy_backend` |
| DNS、连接或 TLS 建立失败 | 502 | `upstream_error` | `upstream_connect_error` |
| 等待上游响应头超时 | 504 | `upstream_error` | `upstream_timeout` |
| 网关内部不变量被破坏 | 500 | `internal_error` | `internal_error` |
| 客户端主动断开 | 不再写响应 | — | `client_canceled` |
| 宽限期后服务强制取消 | 终止请求 | — | `shutdown_canceled` |
| 提交响应后读取上游 body 失败 | 终止响应 | — | `upstream_body_error` |
| 提交响应后写客户端失败 | 终止响应 | — | `downstream_write_error` |

405 响应同时设置允许的方法。`ReadHeaderTimeout` 发生在 Handler 之前，遵循 Go Server 的连接级行为，不承诺统一 JSON、Request ID 或请求结束日志。

### 9.2 日志

每个请求结束时写一条结构化日志：

```text
request_id, method, route, model, stream, backend,
status, committed, error_type, duration_ms, ttfb_ms,
bytes_in, bytes_out, client_canceled
```

`status` 记录实际已写给客户端的 HTTP 状态；尚未提交时记录计划返回的网关状态。`committed` 表示响应头是否已经写出。`bytes_out` 只统计成功写入下游的 body 字节；无 body 或写入失败前没有成功写入时，TTFB 为空。

状态变化和生命周期事件另写日志，包括启动、关闭、backend 健康转换和配置错误。

### 9.3 指标

v0.1 计划暴露：

- `argusgate_http_requests_total{route,status}`
- `argusgate_http_request_duration_seconds{route}`
- `argusgate_proxy_requests_total{backend,model,result}`
- `argusgate_proxy_time_to_first_byte_seconds{backend,model}`
- `argusgate_backend_healthy{backend}`
- `argusgate_backend_inflight{backend}`
- `argusgate_upstream_errors_total{backend,type}`

Request ID、原始 URL、prompt 和其他高基数或敏感值不得作为 label。

## 10. 测试设计

### 10.1 单元测试

- 配置默认值与所有启动失败条件。
- model 精确匹配和独立 round-robin 顺序。
- UNKNOWN、HEALTHY、UNHEALTHY 过滤。
- 健康阈值与计数重置。
- 固定和 `Connection` 动态 hop-by-hop header 过滤。
- 错误分类和 JSON 错误格式。

### 10.2 集成测试

使用 `httptest.Server` 构造可控制上游：

- 原始请求体、允许的 headers、状态码和普通响应体透传。
- SSE 分段写入与 flush，客户端在上游完成前收到首个 event。
- 客户端取消后，上游 handler context 结束。
- 上游连接失败、响应头超时和中途断流。
- backend 返回 3xx 时不自动跟随；数据面透传 3xx，健康检查将其判定为失败，且不会向 Location origin 发送凭据。
- 客户端上传 body 超时；Handler 已开始且响应尚可写时返回 408 `request_timeout`，连接已不可写时至少记录正确分类。
- 客户端 request header 超时由 `net/http.Server` 在 Handler 前处理，不断言统一错误体。
- 普通响应与 SSE 在提交响应后的上游读取错误、下游写入错误及统计口径。
- 多 backend 分流、隔离与恢复。
- 进入 draining 后 `/readyz` 立即变为 503；Shutdown 拒绝新请求、等待在途请求并在超时后以可识别 cause 强制取消。

并发测试优先使用 channel、context 和 WaitGroup 同步，不用任意 `time.Sleep` 猜测顺序。

### 10.3 自动化验收门槛

| 行为 | 本地测试门槛 |
| --- | --- |
| SSE 及时转发 | 上游立即写首 event 并继续保持 2 秒，客户端在 500 ms 内读到首 event |
| 取消传播 | 取消后上游 context 在 1 秒内结束，连续执行 50 次均通过 |
| 并发安全 | 100 个客户端各请求 20 次，race detector 无报告 |
| 健康隔离 | 达到失败阈值后的下一次选择不再返回该 backend |
| 健康恢复 | 达到成功阈值后的下一次选择可以再次返回该 backend |
| goroutine 回收 | 通过显式 channel 或 WaitGroup 证明组件拥有的 goroutine 已退出 |

时间门槛用于发现明显错误，测试应为慢速 CI 提供合理放宽入口，但不能用延长 Sleep 掩盖不确定行为。

### 10.4 工程检查

```text
go test ./...
go test -race ./...
go vet ./...
go fmt ./...
git diff --exit-code -- '*.go'
```

## 11. Design Decisions

本文记录稳定结论；当一个选择具有长期影响、存在多个合理方案或需要保留被拒方案时，再创建独立 ADR。不得为了“看起来完整”预建空 ADR。

| ID | Decision | Reason | Rejected / Deferred |
| --- | --- | --- | --- |
| DD-001 | 使用 Go | 适合网络服务与并发，也是本项目的核心学习目标 | 其他语言不进入 v0.1 评估 |
| DD-002 | v0.1 使用 `net/http` | 直接学习 Handler、ResponseWriter、Transport 和生命周期 | Gin、Fiber、Echo 延后到真实生产力需求出现时评估 |
| DD-003 | 提供 OpenAI-compatible API | 客户端生态成熟，可替换底层 inference backend | 自定义协议会增加客户端适配成本 |
| DD-004 | v0.1 SSE 做字节级透明转发 | 保持代理职责，避免错误重组事件 | 事件解析和转换延后 |
| DD-005 | 静态 JSON 配置 | 标准库支持、行为明确、足够覆盖 v0.1 | YAML、热加载和动态配置延后 |
| DD-006 | v0.1 使用 round-robin | 简单、确定、容易验证 | Least-in-flight 延后到 active requests 统计成熟后 |
| DD-007 | 不实现自动重试 | 生成请求不一定幂等，响应提交后无法安全切换 backend | 仅在安全失败窗口内的有限重试留待 v0.2 |

独立 ADR 的命名规则为 `docs/adr/NNN-short-title.md`，至少包含 Context、Decision、Consequences 和 Rejected Alternatives。只有第一个真实 ADR 出现时才创建 `docs/adr/`。

## 12. 待确认决策

以下问题在进入对应阶段前确认：

1. model 名原样转发，还是支持客户端 model 到上游 model 的显式映射。
2. 指标使用 Prometheus 官方 client，还是手写最小文本格式继续保持标准库优先。

当前默认建议：model 原样转发，并使用 Prometheus 官方 client。

## 13. v0.2 候选方向

v0.1 发布后，根据真实使用和压测结果再排序：

- Least-in-flight 路由。
- 仅在安全失败窗口内的有限重试。
- 熔断、速率限制和并发上限。
- `/v1/models` 聚合。
- model alias 与显式映射。
- 配置热加载。
- OpenTelemetry tracing。
- backend 管理 API 或动态服务发现。

候选能力进入开发前必须回答两个问题：它解决了哪个已经出现的真实问题，以及如何自动验证其正确性。
