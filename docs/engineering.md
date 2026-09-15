# ArgusGate 工程规范

本文记录 ArgusGate 的仓库结构、开发工作流、Go 编码规则、测试、CI、Git、安全和公开策略。

项目定位与系统行为见 [`design.md`](./design.md)，v0.1 阶段任务与验收见 [`v0.1_roadmap.md`](./v0.1_roadmap.md)。当三份文档描述同一事项时：系统行为以 `design.md` 为准，v0.1 阶段范围以 `v0.1_roadmap.md` 为准，开发与质量执行方式以 `engineering.md` 为准。

## 1. 仓库结构

目标结构按阶段逐步形成：

```text
ArgusGate/
├── cmd/
│   ├── argusgate/
│   │   └── main.go
│   └── mockbackend/        # Phase 1 出现，仅用于开发与演示
├── internal/
│   ├── app/                # Composition root 与生命周期
│   ├── config/             # 默认值、加载与校验
│   ├── httpapi/            # Handler、middleware 与错误响应
│   ├── backend/            # Backend 与 Registry
│   ├── routing/            # model 匹配与选择策略
│   ├── proxy/              # 普通与 SSE 转发
│   ├── healthcheck/        # 后端健康探测
│   └── observability/      # slog 与 metrics
├── configs/
│   └── argusgate.example.json
├── docs/
│   ├── design.md
│   ├── v0.1_roadmap.md
│   ├── engineering.md
│   └── adr/                # 第一个真实 ADR 出现时才创建
├── .github/
│   └── workflows/
├── .editorconfig
├── .gitattributes
├── .gitignore
├── go.mod
├── go.sum
├── Dockerfile
├── README.md
└── LICENSE
```

原则：

- 不提前创建大量空目录、空接口或占位文档。
- 新包必须对应正在实现的职责，并满足 `design.md` 的依赖方向。
- 可复用性未经验证前，代码放在 `internal/`；v0.1 不建立 `pkg/`。
- mock、fixture 和 test helper 不得进入生产请求路径。
- 构建产物、覆盖率输出、本地配置、日志和模型文件不得提交。
- 不提交仅适用于个人机器的绝对路径、端口偏好或模型位置。

## 2. 本地开发工作流

所有 Go 命令从仓库根目录执行。项目支持的最低 Go 版本以 `go.mod` 的 `go` directive 为唯一基线。安装或更新 PATH 后，如果已有终端或开发工具仍找不到 `go`，应重启对应进程并重新执行 `go version`，不要在仓库脚本中写入个人 Go 安装路径。

### 2.1 首次准备

```bash
go version
go mod download
go test ./...
```

### 2.2 日常检查

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
```

`go fmt ./...` 会修改文件，提交前需要检查 diff。CI 先运行该命令，再用 Git 检查 Go 文件是否产生 diff；出现 diff 即失败。

race test 在出现共享并发状态后成为阶段必检项。若本地平台暂不具备 race detector 条件，应在受支持的 CI 环境执行，并在交付说明中明确本地未执行原因。

### 2.3 本地运行

```bash
go run ./cmd/argusgate -config ./configs/argusgate.example.json
```

Phase 1 加入 mock backend 后，可在另一个终端运行：

```bash
go run ./cmd/mockbackend
```

实际端口和示例请求应记录在 `README.md` 或 `v0.1_roadmap.md` 的 Demo Gate，不把个人端口写死在代码中。

## 3. 代码与文档语言

| 内容 | 语言 |
| --- | --- |
| 包、文件、变量、函数、类型与常量 | 英文 |
| API path、JSON field、Log 与 Error Message | 英文 |
| 正式代码注释 | 中文为主，保留必要英文术语 |
| Commit Message | 类型前缀使用约定英文关键字，说明正文使用中文 |
| README 与 `docs/*.md` | 中文为主，技术名词按需保留英文 |
| 临时调试注释 | 可使用中文，提交前删除或整理 |

长期注释应说明设计原因、边界、不变量或易错点，不复述代码表面行为。公开标识符只有在职责不直观或形成包级 API 时才补充有效说明，不为满足形式写无信息量注释。

## 4. Go 编码规范

### 4.1 命名与包

- 包名使用简短、小写、单数单词，不使用下划线和含糊缩写。
- 文件名使用小写；需要分词时使用下划线，例如 `request_id.go`。
- 导出标识符使用 PascalCase，非导出标识符使用 camelCase。
- 常量使用 Go 惯例命名，不机械采用全大写下划线风格。
- acronym 在同一项目内保持一致，例如 `HTTPServer`、`RequestID`、`URL`。
- 避免 `util`、`helper`、`common`、`manager` 等无法表达职责的包名。
- 接口定义在消费方附近，名称通常描述能力，例如 `Selector`、`HealthRecorder`。
- 不为只有一个实现且无需替身的类型提前创建接口。

### 4.2 函数与类型

- 函数保持单一职责；当错误处理和生命周期无法一眼理解时及时拆分。
- 构造函数只建立有效对象，不在未说明的情况下启动 goroutine。
- 需要启动后台工作的组件应显式提供 `Run(ctx)`、`Start`/`Stop` 或由 App 统一管理。
- 配置与静态 backend 数据在初始化后保持不可变。
- 可变共享状态必须隐藏在拥有它的类型后面，不暴露可直接修改的 map、slice 或指针。
- 优先返回有意义的零值；无法安全使用零值的类型应通过构造函数创建。

### 4.3 格式与静态检查

- `gofmt` 是唯一格式标准，不手动争论空格和换行风格。
- `go vet ./...` 是 v0.1 的基础静态检查。
- 不使用注释或构建标签掩盖可以直接修复的问题。
- 只有基础流程稳定且出现真实收益时再引入额外 linter；不得一次开启大量规则制造无关重构。
- 不在功能提交中混入无关的大规模 rename 或 format rewrite。

## 5. Error 规范

- error 是调用边界的一部分，不使用 panic 处理普通输入、配置或网络失败。
- 只有进程入口决定退出码；内部包返回 error，不调用 `os.Exit` 或 `log.Fatal`。
- 使用 `%w` 包装需要保留 cause 的错误，并用 `errors.Is`/`errors.As` 判断。
- 只有调用方确实需要稳定分类时才定义 sentinel 或 typed error。
- 错误消息使用英文、小写开头、无句末标点，便于包装成完整错误链。
- 不同时记录并返回同一个错误，避免重复日志；请求边界或进程边界统一记录。
- 对外错误遵循 `design.md` 的稳定 error type/code，不暴露内部路径、secret 或原始底层错误。
- 响应提交后不得尝试写第二份 JSON 错误，只记录 post-commit 失败并结束响应。

## 6. Context 与生命周期

- `context.Context` 作为需要取消、deadline 或请求作用域数据的函数第一个参数。
- 不把 context 存入长期存活的 struct，也不传入 nil context。
- 不使用 context 传递普通配置或可选参数。
- handler 创建的上游请求必须继承 handler context。
- 组件创建 goroutine 时必须明确 owner、停止信号和等待方式。
- goroutine 不得脱离调用链无限存活；后台循环应响应 context cancellation。
- timer 和 ticker 由创建方停止，channel 由发送方或明确 owner 关闭。
- 服务关闭遵循 `design.md` 中 RUNNING → DRAINING → STOPPED 的两阶段语义。
- 强制关闭使用可识别的 cancel cause，不能误记为客户端取消。

## 7. 并发规范

- 先实现正确的串行版本，再为真实共享状态引入同步。
- 不在持锁期间执行 HTTP 请求、文件 I/O、日志输出或其他不可控阻塞操作。
- mutex 保护不变量，而不只是某个字段；相关字段应在同一临界区保持一致。
- Router 基于一次 Registry 快照完成选择，不在选择过程中反复读取可变状态。
- active requests 的增加与减少必须成对，获取成功后立即注册 `defer`。
- 并发数量必须有上限；禁止按不可信输入无界创建 goroutine。
- channel 用于所有权转移或事件协调，不用 channel 取代简单锁。
- atomic 只用于可以独立变化、语义明确的简单值；多个字段的不变量仍使用锁。
- 并发测试使用 channel、barrier、context 或 WaitGroup 同步，不用任意 `time.Sleep` 猜测顺序。
- `go test -race` 无报告是必要条件，但不能替代状态机和业务不变量测试。

## 8. HTTP 与 Proxy 规范

- 共享 `http.Client` 和 Transport，禁止每个请求新建 Client。
- Client 的 `CheckRedirect` 必须返回 `http.ErrUseLastResponse`，代理和健康检查均不得自动访问未配置 origin。
- Client 不设置覆盖整个生成过程的固定总 timeout；按 `design.md` 配置连接、TLS 和响应头阶段。
- 读取客户端 body 前先应用大小限制和读取时限。
- 原始 JSON 只解析必要字段，不重编码后转发。
- `http.Client.Do` 成功后立即安排 `response.Body.Close()`。
- 过滤固定 hop-by-hop headers 和 `Connection` 动态声明的 headers。
- backend URL 只能由经过验证的 origin 与 endpoint path 构造，不接受客户端提供目标地址。
- 未经显式配置不透传客户端 Authorization。
- 包装 `ResponseWriter` 时保留 Flusher 等真实需要的可选接口。
- SSE 每次成功写入下游数据后及时 Flush，不解析或重组 event。
- HTTP 状态和 header 提交后不可改写；统计必须区分 committed 与未提交状态。
- 日志使用路由模板或固定 route name，不记录包含敏感 query 的原始 URL。

## 9. Testing

测试与纵向功能一起提交，不集中拖到版本末尾。

### 9.1 测试类型

- Unit Test：配置解析、状态转换、路由策略、header 过滤和错误映射。
- Handler Test：method、状态码、header、错误 JSON 和请求限制。
- Integration Test：代理、SSE、取消、Shutdown、健康检查和跨组件行为。
- Race Test：Registry、Router、Health Checker 与 active requests 并发访问。
- Benchmark：仅用于有明确问题的热路径和版本发布数据。

### 9.2 基本要求

- 测试外部可观察行为，不机械复制实现步骤。
- 默认使用 `httptest.Server` 或本地 mock，不依赖付费 API、个人密钥和不稳定网络。
- 真实 inference backend 测试必须显式选择，并提供清楚的环境要求和跳过原因。
- table-driven test 的 case 名应描述输入或预期行为。
- helper 调用 `t.Helper()`；清理资源优先使用 `t.Cleanup()`。
- Bug 修复应在可行时先加入能够复现问题的测试。
- 不用覆盖率百分比代替有意义的边界断言；v0.1 暂不设置任意覆盖率门槛。
- 不因测试困难而暴露本应私有的生产实现细节。
- 时间敏感测试使用明确事件同步和有上限的 deadline，不能依赖长 Sleep 提高通过率。
- 测试结束必须证明服务、response body、listener 和组件 goroutine 已释放。

### 9.3 测试命令

```bash
go test ./...
go test -race ./...
go test -count=1 ./...
```

`-count=1` 用于排除测试缓存影响，不要求每次本地循环都执行。需要定位偶发并发问题时，可以对目标包增加重复次数，但不能用重复通过证明不存在竞态。

## 10. 依赖管理

- 标准库可以清楚完成的 v0.1 核心行为，优先使用标准库。
- 新增第三方生产依赖前说明用途、替代方案和长期维护成本。
- 使用 `go get` 调整依赖后执行 `go mod tidy`，同时提交 `go.mod` 与 `go.sum`。
- 不手工编辑 `go.sum`，不提交本地 module cache。
- CI 根据 `go.mod` 安装 Go 并下载依赖。
- v0.1 不使用 vendoring；只有离线构建等真实需求出现时再评估 `vendor/`。
- 测试依赖同样需要保持克制，优先使用标准库 `testing` 与 `httptest`。

## 11. Continuous Integration

初始 CI 在 push 和 pull request 时运行：

```text
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
```

格式检查通过 `gofmt -l .` 的输出是否为空进行验证，CI 不自动改写或提交文件。依赖由 Go 命令按 `go.mod` 下载。出现共享并发状态后增加独立 race job：

```text
go test -race ./...
```

CI 原则：

- Go 版本读取 `go.mod`，避免本地和 CI 漂移。
- 缓存依赖，但不依赖缓存保证构建成功。
- 本地通过不等于远端通过，阶段验收需要核对实际 CI 结果。
- 基础流程稳定后再增加 Docker build、安全扫描或 release job。
- 外部真实 backend 测试不进入默认 CI，除非提供稳定、无密钥的受控环境。

## 12. Git 与变更管理

- 一个提交聚焦一个可解释、可验证的变更。
- Commit Message 使用中文，格式为 `<类型>: <中文说明>`，例如 `feat: 增加健康检查接口`。
- 类型前缀使用约定的英文关键字：`feat`（功能）、`fix`（修复）、`docs`（文档）、`test`（测试）、`refactor`（重构）、`chore`（维护）、`ci`（持续集成）和 `build`（构建）。
- 中文说明使用简洁的动宾结构，描述本次变更的目的；结尾不加句号。
- 提交前检查 diff，确认没有 secret、日志、构建产物和无关修改。
- 不重写他人提交，不丢弃未确认的本地修改。
- 行为、配置、目录或启动方式变化时同步更新对应文档和示例。
- 依赖变更在提交说明中写明原因。
- 重构与行为变化尽量分开提交。
- 不提交仅为“以后可能用到”的接口、配置项和空目录。

### 12.1 Agent 授权与路线约束

- 所有 AI Agent 必须遵守仓库根目录的 [`AGENTS.md`](../AGENTS.md)。
- Agent 只能实施用户明确要求的工作及其不可缺少的最小配套修改；不得把改进建议视为实施授权。
- 未经用户明确允许，Agent 不得自行添加功能、接口、配置、依赖、目录或预建抽象。
- 未经用户明确允许，Agent 不得新增、删除、提前、推迟、重排或替换路线图任务，不得改变当前 Phase、版本范围或项目开发方向。
- 发现范围外需求或路线调整机会时，Agent 应先说明依据、影响和可选方案，取得用户明确授权后才能修改实现或路线文档。
- 即使测试或重构能够顺带实现额外能力，也必须保持在当前授权范围内；会改变外部行为、公共接口、配置格式或路线安排的修复需先由用户决定。

## 13. Security

v0.1 不实现用户认证，但必须遵守 `design.md` 的 trust boundary：

- 所有客户端输入、headers、model 名和 body 都视为不可信。
- backend origin 和路径只能来自经过验证的配置。
- API key、token、password、cookie 和私钥不得提交、打印或返回客户端。
- 示例配置只包含环境变量名或安全占位符，不包含真实 secret。
- 默认不透传客户端 Authorization。
- 日志不记录完整 prompt、响应、Authorization、Cookie 或原始敏感 URL。
- 指标 label 不包含用户输入、高基数标识符或 secret。
- 服务默认只部署在本机或受信任网络；无认证配置时不得直接暴露公网。
- 测试 fixture 和 bug 报告使用合成数据，不复制真实用户输入或日志。
- 发现 secret 进入 Git 后，删除文件不足以完成处置；还必须轮换凭据并清理公开历史。

## 14. 公开仓库策略

### 14.1 可以公开

- `README.md`、`LICENSE`、项目文档和 ADR
- `.gitignore`、`.gitattributes`、`.editorconfig` 和 CI 配置
- Dockerfile、示例配置和 mock backend
- 源码、测试及经过审查的合成 fixture
- 有环境说明和复现命令的 benchmark 结果

### 14.2 不得公开

- 真实 API key、token、password、cookie、证书和私钥
- 含 secret 的本地配置文件
- 未脱敏的 prompt、响应、用户数据和日志
- 私有 backend 地址、个人绝对路径和机器专用配置
- 模型权重、个人数据集和大型生成产物
- IDE 缓存、Go build/module cache、coverage output 和临时二进制

`.gitignore` 不能保护已经跟踪的文件，也不可能覆盖所有敏感格式。提交前必须检查暂存区内容。

## 15. 文档与 ADR 职责

- `README.md`：面向新读者的项目介绍、当前能力、快速启动和演示。
- `docs/design.md`：稳定的定位、边界、系统行为、架构和设计决策索引。
- `docs/v0.1_roadmap.md`：v0.1 阶段任务、Knowledge Checkpoint、验收与发布清单。
- `docs/engineering.md`：开发流程、Go 规则、测试、CI、Git、安全和公开策略。
- `AGENTS.md`：AI Agent 的授权边界、路线约束和仓库内变更要求。
- `docs/adr/*.md`：具有长期影响的单项决策及其背景、后果和被拒方案。

只在真实内容出现时创建新文档，不创建 `api.md`、`development.md`、`learning-log.md` 等空占位。API 契约目前属于 `design.md`，学习检查属于 `v0.1_roadmap.md`；当内容规模或使用者真的需要独立入口时再拆分。

ADR 创建条件：

- 存在多个合理方案，并且选择会长期影响系统。
- 决策改变包依赖、公共 API、配置格式、并发模型或安全边界。
- 需要保留被拒方案和未来重新评估条件。

普通实现细节、可轻易撤销的局部选择和尚未发生的设想不创建 ADR。

## 16. Definition of Done

提交代码前确认：

1. 代码已由 `gofmt` 格式化。
2. 相关 `go vet`、unit、integration 和 race 检查已按阶段执行。
3. 新行为具备有意义的测试，错误和资源释放路径同样覆盖。
4. `design.md`、`v0.1_roadmap.md`、`engineering.md`、`README.md`、配置示例与实际实现不矛盾。
5. diff 不包含 secret、缓存、二进制、日志或无关修改。
6. 新依赖有明确用途，`go.mod` 与 `go.sum` 保持同步。
7. 新 goroutine 有 owner、停止信号和等待机制。
8. 新 HTTP 路径具备输入限制、错误语义和敏感信息检查。
9. 未执行的检查及原因在交付说明中明确列出。
10. 变更仍处于当前 Phase 和 v0.1 边界内。
