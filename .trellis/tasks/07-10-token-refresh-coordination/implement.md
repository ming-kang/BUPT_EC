# Token 并发刷新协调 — Implementation Plan

## Implementation

- [x] 搜索所有 `setToken`、`clearTokenIfCurrent`、`EnsureToken(true)` 调用点。
- [x] 为 token value 增加 source 状态并调整 override/login 写入路径。
- [x] 实现 `RefreshAfterAuthFailure(ctx, failedToken)`，在 singleflight 内复查新 token。
- [x] 将 queryCampus auth retry 改为新 API。
- [x] 使用 `DoChan` + bounded `WithoutCancel` context 改善 token/API URL waiter cancellation。
- [x] 保留一次重试和现有安全错误分类。
- [x] 更新开发文档、runtime/cache spec 和 changelog。

## Focused Tests

- [x] simultaneous auth failures => one login。
- [x] delayed second auth failure after first login => no second login。
- [x] override source rejection invalidates override permanently for process。
- [x] login source rejection does not alter unrelated override state。
- [x] canceled waiter returns quickly while surviving waiter receives token。
- [x] API URL singleflight has equivalent cancellation behavior。

## Validation

```powershell
gofmt -w service
go test ./service
go test -race ./service
go vet ./...
```

## Rollback Point

若 source/cancellation 重构出现回归，可整体回滚子任务 commit；不需要清理任何持久化状态，进程重启会重置 token。
