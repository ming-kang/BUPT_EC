# 上游错误与启动失败安全设计

## Safe Remote Message Pipeline

```text
raw upstream message
  -> trim + collapse control whitespace
  -> redact sensitive key/value and bearer-like tokens
  -> cap at 256 Unicode runes
  -> fallback when empty
  -> internal jwError/log only
```

脱敏 helper 保持纯函数和确定性。不要在这里读取运行时 credentials，也不要把真实凭据
作为 redaction 参数传入日志层。

建议敏感模式：

- `token|authorization|password|passwd|username|account` 后的分隔值；
- `学号|账号|密码|令牌` 后的分隔值；
- `Bearer <value>`；
- 明显长的无空格 credential-like 值只在敏感 key 上下文内处理，避免误删普通中文。

## Startup Error Flow

```text
main.Init/config composition
  -> logs.Init(...) error
  -> NewHTTPServer(...) error
  -> main returns safe startup error
  -> process entrypoint prints once and exits non-zero
```

包级函数不得自行调用 `log.Fatal` 或 `os.Exit`。已打开的 writer 若未来需要 close，应由
composition root 持有；本任务不要求改变 lumberjack 生命周期。

## HTTPServer Constructor

建议签名：

```go
func NewHTTPServer(
    classroomService classroomDataService,
    hasJWCredentials func() bool,
) (*HTTPServer, error)
```

使用通用 nil-interface 检查覆盖 typed nil。credentials callback 保持 nil -> false，除非
实现审查证明生产边界应强制提供；核心非空约束是 service。

## Compatibility

- 正常调用点只增加 error handling。
- API envelopes 和日志字段名不变。
- 上游消息可能被截断/脱敏，这是有意的安全行为变化，应记入 changelog。

## Rollback

消息清洗、logs.Init 签名、HTTPServer 构造器可分提交回滚，但 main 和全部测试调用点必须
与签名保持原子一致。
