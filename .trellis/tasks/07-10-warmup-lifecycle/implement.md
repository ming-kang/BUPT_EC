# Warmup 生命周期与跨日重试 — Implementation Plan

## Dependencies

先完成 `07-10-refresh-outcome-contract`，以便 scheduler 可靠判断 full、partial 和 failed。

## Implementation

- [ ] 在 `ClassroomService` 增加 scheduler once/completion 生命周期字段，保持状态实例级。
- [ ] 修改 `StartWarmup` 接收 context，并移除裸 `time.Sleep`。
- [ ] 实现 pure delay/state helper，包含 next midnight、backoff 和 capped retry。
- [ ] full/partial/no-cache 使用各自下一次调度策略。
- [ ] 重构 shutdown：cancel scheduler -> HTTP shutdown -> wait scheduler/refresh workers。
- [ ] 将 `WaitWarmup` 重命名或扩展为准确表达所有 background work 的接口。
- [ ] 更新 main tests、service tests、docs 和 backend specs。

## Focused Tests

- [ ] initial warmup starts immediately。
- [ ] midnight backoff schedules nextAllowed retry。
- [ ] failure delays cap at five minutes。
- [ ] full success resets retry and targets next midnight。
- [ ] partial targets no earlier than fresh TTL。
- [ ] cancellation exits scheduler；second StartWarmup is a no-op。
- [ ] shutdown ordering under `go test -race` does not add workers after wait begins。

## Validation

```powershell
gofmt -w main.go service
go test ./service ./...
go test -race ./...
go vet ./...
```

## Rollback Point

公共 HTTP/API 不变。若 scheduler 生命周期回归，可回滚该 commit 并恢复单次 startup warmup；刷新三态不依赖 scheduler 的具体实现。
