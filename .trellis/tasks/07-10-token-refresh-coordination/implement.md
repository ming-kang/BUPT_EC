# Token 并发刷新协调 — Implementation Plan

## Implementation

- [ ] 搜索所有 `setToken`、`clearTokenIfCurrent`、`EnsureToken(true)` 调用点。
- [ ] 为 token value 增加 source 状态并调整 override/login 写入路径。
- [ ] 实现 `RefreshAfterAuthFailure(ctx, failedToken)`，在 singleflight 内复查新 token。
- [ ] 将 queryCampus auth retry 改为新 API。
- [ ] 使用 `DoChan` + bounded `WithoutCancel` context 改善 token/API URL waiter cancellation。
- [ ] 保留一次重试和现有安全错误分类。
- [ ] 更新开发文档、runtime/cache spec 和 changelog。

## Focused Tests

- [ ] simultaneous auth failures => one login。
- [ ] delayed second auth failure after first login => no second login。
- [ ] override source rejection invalidates override permanently for process。
- [ ] login source rejection does not alter unrelated override state。
- [ ] canceled waiter returns quickly while surviving waiter receives token。
- [ ] API URL singleflight has equivalent cancellation behavior。

## Validation

```powershell
gofmt -w service
go test ./service
go test -race ./service
go vet ./...
```

## Rollback Point

若 source/cancellation 重构出现回归，可整体回滚子任务 commit；不需要清理任何持久化状态，进程重启会重置 token。
