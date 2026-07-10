# 上游错误与启动失败安全

## Goal

限制并清洗 JW 上游消息，消除日志初始化直接退出和 HTTPServer nil 依赖等隐式启动失败。

## Background

- `service/jw_error.go:88-92` 的 `safeRemoteMessage` 只做 TrimSpace，上游任意长度、
  控制字符或疑似账号/token 内容仍可能进入内部 error 和 warning log。
- `logs/log_util.go:26-31` 在创建日志目录失败时调用 `log.Fatalf`，包内部直接终止
  进程，绕过 composition root 的错误处理和测试。
- `handler.go:26-34` 的 `NewHTTPServer` 接受 nil `classroomService`，直到请求访问或
  readiness 检查才 panic。
- 客户端已经通过 `SafeErrorMessage` 获取固定安全文案；本任务主要收紧内部日志和
  启动边界，不改变公开错误类型。

## Requirements

### R1 — 上游消息必须有界且可安全记录

- 规范化 CR/LF、tab 和其他控制字符，避免多行/伪造结构化日志。
- 对 Unicode rune 长度设硬上限，推荐 256；截断需稳定、可测试。
- 对常见敏感 key/value（token、authorization、password、username、account、
  学号/账号/密码）及 bearer-like 值进行保守脱敏。
- 空白或清洗后为空时使用固定 fallback，不回显原始 payload。
- API 客户端仍只看到 `SafeErrorMessage` 的固定中文安全文案。

### R2 — 日志初始化返回错误

- `logs.Init` 返回 error，不得调用 `log.Fatal`、`os.Exit` 或 panic。
- `main.go` composition root 负责决定启动失败输出和唯一退出点。
- 错误中只包含安全路径/操作信息，不包含 env 或凭据内容。

### R3 — HTTPServer 构造必须拒绝 nil service

- `NewHTTPServer` 对 nil 和 typed-nil `classroomDataService` 返回明确错误。
- `hasJWCredentials` 是否继续作为可选 callback需保持现有 false-default兼容；如果改为
  必需，必须同步所有生产/测试调用点并记录理由。
- 构造失败在 route registration 和监听端口之前被处理。

### R4 — 测试和文档

- 增加控制字符、超长中文、token/password/account 片段和空消息测试。
- 增加日志目录失败、HTTPServer nil/typed-nil 和正常构造测试。
- 同步 error/logging/directory specs、AGENTS、development/operations 和 changelog。

## Acceptance Criteria

- [ ] 任意上游消息进入日志前为单行、长度有界且敏感片段被替换。
- [ ] API error body 不包含原始上游消息、token、账号或凭据。
- [ ] `logs` 包不再直接退出进程，初始化失败由 main 返回/处理。
- [ ] nil 或 typed-nil service 无法构造 `HTTPServer`。
- [ ] 正常启动、handler fake 注入和 readiness 测试继续通过。
- [ ] `gofmt`、`go vet ./...`、`go test -race ./...`、完整构建通过。
- [ ] 不改变 JW protocol、API error type 或公开成功 payload。

## Out of Scope

- 对所有日志字段实施通用 DLP 系统。
- 更换 slog/lumberjack 或日志存储方案。
- 修改 JW 登录、token singleflight 或 refresh outcome 语义。
