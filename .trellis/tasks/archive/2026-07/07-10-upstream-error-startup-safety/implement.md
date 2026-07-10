# 上游错误与启动失败安全实施计划

## 1. Message sanitization

- [ ] 收集现有 JW Login/Query/parse 错误构造点和日志路径。
- [ ] 新增控制字符规范化、敏感模式脱敏和 rune 截断纯函数。
- [ ] 表驱动测试覆盖中英文、CRLF、Bearer、key/value、超长和空消息。
- [ ] 验证客户端仍只使用 `SafeErrorMessage`。

## 2. Logging initialization

- [ ] 将 `logs.Init` 改为返回 error，移除 `log.Fatalf` import/调用。
- [ ] main composition root 显式传播并安全输出启动错误。
- [ ] 用不可创建日志目录的测试路径验证失败，不修改真实 `run_log/`。

## 3. HTTPServer construction

- [ ] 增加 nil/typed-nil dependency helper或复用同类模式。
- [ ] `NewHTTPServer` 返回 error，更新 main、handler/router tests 和所有调用点。
- [ ] 断言失败发生在 route registration/listen 前。

## 4. Documentation

- [ ] 更新 error-handling、logging、directory/quality specs。
- [ ] 更新 AGENTS、development/operations 和 changelog。

## Validation

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
rg -n "log\.Fatal|os\.Exit|safeRemoteMessage|NewHTTPServer" --glob "*.go" .
git diff --check
```

## Review Gates

- 脱敏测试不得包含真实 JW 凭据。
- 截断按 rune 而不是任意切断 UTF-8 bytes。
- 包级初始化不得拥有退出进程的权力。
- 构造器错误不得泄露配置值。

## Rollback Points

- Sanitizer 与测试。
- `logs.Init` + main 调用点。
- `NewHTTPServer` + 全部构造调用点。

任何签名变更必须在同一个提交中保持可构建。
