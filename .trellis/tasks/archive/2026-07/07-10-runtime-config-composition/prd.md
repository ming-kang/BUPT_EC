# 运行时配置边界与依赖组装

## Goal

将进程环境读取和启动校验收敛到单一配置边界，并由 `main.go` 作为
composition root 显式组装生产依赖，使 service、JW 协议层和 HTTP 边界不再
通过包级全局或热路径 `os.Getenv` 隐式获取运行条件，同时保持现有产品行为。

## Background and Confirmed Facts

- `config/config.go` 通过 `GlobalConfig` + `sync.Once` 保存校区配置，并在
  `HasJWCredentials` / `ValidateRuntimeConfig` 中直接读取进程环境。
- `cache/cache.go` 通过 `GlobalCache` + `sync.Once` 保存进程内缓存；
  `main.go::Init` 初始化后再传给 `service.NewClassroomService`。
- `service/defaultJWClient.Login` 在每次登录时读取 `JW_USERNAME` /
  `JW_PASSWORD`，`TokenManager.installEnvOverrideToken` 在取 token 时读取
  `JW_TOKEN`，因此协议层和 token 状态仍隐式依赖进程环境。
- `main.go` 直接读取 `APP_ADDR`，`logs.Init` 直接读取 `LOG_CALLER`；
  `.env` 当前由 `config.InitConfig` 通过 `godotenv.Load` 加载。
- `ClassroomService` 已持有 `CacheStore`、`JWClient`、时钟和 warmup jitter
  seam；handler 已通过 `NewHTTPServer` 注入 service 与凭据状态函数。
- README 和文档明确产品只查询西土城 `01`、沙河 `04`。本任务不新增校区
  配置表面，也不改变公开 API、缓存、刷新、鉴权或安装器契约。
- `JW_TOKEN` 在鉴权失败后失效直到进程重启。启动时快照该值与现有语义一致；
  本任务不提供运行时配置热加载。
- 过去的架构审查已指出 `GlobalConfig`、热路径 `os.Getenv` 和隐式
  `defaultJWClient` 是当前维护性路线中下一处高价值边界。

## Requirements

- R1：`.env` 和进程环境只在启动配置加载阶段读取；配置加载返回不可变值和
  明确错误，不依赖包级 `GlobalConfig`。
- R2：`.env` 缺失时继续使用系统环境；文件存在但无法解析时启动失败。系统环境
  继续覆盖 `.env` 同名值，错误信息不得包含配置内容。
- R3：启动配置必须覆盖现有运行项：JW 用户名/密码/token、`APP_ADDR`、
  `GIN_MODE`、`LOG_CALLER` 和固定校区列表，并保持当前默认值与优先级。
- R4：启动校验继续要求 `JW_TOKEN`，或同时具有 `JW_USERNAME` 与
  `JW_PASSWORD`；错误不得包含凭据内容。
- R5：JW 登录凭据与 token override 作为构造值进入 JW/token 依赖；
  `service/` 生产路径不得再直接读取 JW 环境变量。
- R6：`main.go` 是唯一生产 composition root，显式组装日志、缓存、JW client、
  `ClassroomService`、`HTTPServer` 和后台生命周期。
- R7：本轮同时移除 `cache.GlobalCache` / `InitCache`，由显式 cache 构造函数
  返回进程内实例；默认 JW client 必须显式接收凭据和 HTTP transport。
- R8：保留可测试 seam，不引入依赖注入框架；构造函数只暴露跨包真正需要的
  依赖，测试仍可使用 mock `JWClient`、独立 cache、假时钟与 jitter；必填依赖
  缺失时在构造阶段返回安全错误，不延迟到请求路径 panic。
- R9：保持 `JW_TOKEN` 来源追踪、鉴权失败失效、并发登录协调、
  Asia/Shanghai 缓存语义、健康/就绪响应和所有公开 JSON 不变。
- R10：不得记录、返回或在测试失败信息中展开 JW 用户名、密码或 token。
- R11：若构造函数、配置键或启动流程改变，必须同步 `AGENTS.md`、
  `docs/development.md`、相关 `.trellis/spec/backend/` 与 `[Unreleased]`。

## Acceptance Criteria

- [x] 配置加载和校验可使用显式环境映射进行表驱动测试，不依赖测试进程的
      全局环境残留。
- [x] `.env` 缺失、有效文件、系统环境覆盖、存在但格式错误四种路径均有测试。
- [x] `config.GlobalConfig` / `InitConfig` 不再是生产启动依赖。
- [x] `cache.GlobalCache` / `InitCache` 不再存在，生产与测试均通过显式实例使用
      `CacheStore`。
- [x] `service/` 中不存在生产凭据的 `os.Getenv` 热路径。
- [x] 默认 JW client 的凭据和 HTTP transport 均可从构造链追踪，HTTP 安全约束
      （禁止 redirect、响应体上限、timeouts）保持不变。
- [x] JW client / ClassroomService 构造器对缺失的必填依赖返回错误，且错误不含
      凭据值。
- [x] token override 注入后仍只在实际 override 被拒绝时失效，且失效状态保持
      到进程重启。
- [x] 默认地址仍为 `127.0.0.1:8080`，`LOG_CALLER`、Gin mode 与凭据校验行为
      保持兼容。
- [x] 生产依赖从配置加载到 JW 查询的值流可从 `main.go` 构造链清晰追踪。
- [x] 现有 Go 单元/竞态测试通过，并新增配置、构造和 token override 回归测试。
- [x] 文档、代码规范与 CHANGELOG 描述最终真实启动流程，且不包含凭据。

## Out of Scope

- 运行时配置热加载、凭据在线轮换或多租户。
- 通过环境变量或配置文件自定义校区列表。
- 改变 JW 协议、AES 密钥、API URL 白名单或 token 重试策略。
- 引入 Wire、Fx 等依赖注入框架。
- 借本任务进行 service 责任大拆分、缓存领域化、前端重构或 CI 去重。

## Scope Decision

本任务采用完整 composition root：同时收口配置快照、`cache.GlobalCache`、
生产 JW client 凭据和默认 HTTP transport。时钟/jitter 已有测试 seam，不借本任务
扩大为通用依赖容器；日志仍使用进程级默认 `slog`，但 `LOG_CALLER` 由配置值传入。

## Configuration Error Decision

`.env` 不存在不是错误，因为生产 systemd 通过 `EnvironmentFile` 注入环境；若
`.env` 实际存在但语法无法解析，则立即返回安全启动错误。系统环境保持高于
`.env` 的优先级，与 `godotenv.Load` 现有覆盖语义一致。

## Implementation Results

- `config.Load` 现在以显式 `LookupEnv` + 可选 dotenv 构造一次性
  `RuntimeConfig`，集中校验凭据、Gin mode 和监听地址，不修改进程环境。
- 删除 config/cache/HTTP 的运行时单例；`main.go::Init` 显式构造 cache、HTTP
  client、JW client、`ClassroomService` 和 `HTTPServer`。
- JW 用户名/密码与 token override 均改为构造值；`service/` 和 `logs/` 生产路径
  不再读取运行环境，override 来源追踪/失效和单飞登录语义保持不变。
- 构造器会拒绝 nil/typed-nil 必填依赖，校区 slice 在 service 构造时复制；新增
  config、cache、HTTP transport、JW 凭据和构造器回归测试。
- README、AGENTS、开发文档、后端 code-spec 与 CHANGELOG 已同步；公开 API、
  缓存、刷新、健康/就绪和前端契约未改变。

## Validation Results

- `gofmt -l .`：通过，无输出。
- `go vet ./...`：通过。
- `go test ./...`：通过；无真实凭据时 integration tests 正常跳过。
- `go test -race ./...`：通过。
- `go build -o <temporary-output> -v ./`：通过。
- `git diff --check`：通过（仅显示现有 Trellis task JSON 的 CRLF 提示）。
- `govulncheck ./...`：本机未安装，未执行；本任务没有修改依赖。
- 前端 lint/test/build 未执行，因为未修改前端源文件、包或公开 API 契约。
