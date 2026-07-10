# HTTP 协议与日志关联实施计划

## 1. Characterization

- [ ] 补充当前 gzip、API NoRoute、SPA fallback 和 refresh cancellation 基线测试。
- [ ] 明确 Gin 对未知 method 的当前状态码，再决定是否启用 405 处理。

## 2. Gzip negotiation

- [ ] 新增 `acceptsGzip` 纯函数和表驱动测试。
- [ ] 在 middleware 中使用 parser，正确维护 Vary/Content-Length。
- [ ] 覆盖 q=0、大小写、wildcard、多 coding、格式错误和 health/readiness。

## 3. API log context

- [ ] 增加 path-aware engine middleware，确保 API 路径只生成一次 log ID。
- [ ] 让 API NoRoute/NoMethod 复用 context 并返回 header/body log ID。
- [ ] 保持非 API 静态文件和 SPA fallback 行为。

## 4. Refresh correlation

- [ ] 将 refresh worker context 改为保留 values 的 detached bounded context。
- [ ] 测试发起请求取消、后续 waiter 取消、warmup 发起和 log ID 保留。
- [ ] 检查 token/JW 子调用继续使用同一 refresh deadline。

## 5. Docs/spec

- [ ] 更新 API、logging、runtime-state specs、AGENTS、development/operations 和 changelog。

## Validation

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
git diff --check
```

## Review Gates

- 不使用 substring 判断 encoding。
- 不允许一个请求生成两个不同 log ID。
- 不把共享 refresh 生命周期重新绑定到单个客户端请求。
- SPA fallback 和 health/readiness 必须有明确回归测试。

## Rollback Points

- Gzip parser/middleware。
- API context/NoRoute envelope。
- Detached refresh context。

三个部分可分别回滚，但最终变更应保持 specs 与测试同步。
