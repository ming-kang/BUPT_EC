# Design: Run/Shutdown 生命周期 + 取消传播

## 现状证据

| 位置 | 现状 | 问题 |
|---|---|---|
| `service/warmup.go:76-99` | `StartWarmup(ctx)` fire-and-forget，派生 `warmupCtx` | 调用方不知道何时结束 |
| `service/warmup.go:162-180` | `WaitBackground(ctx)` cancel + 等 done + 排空 workers | 与 StartWarmup 非对称 |
| `service/classroom_service.go:61-66` | `backgroundMu/backgroundStopping/warmupStarted/warmupDone/warmupCancel` | 5 字段侧门 |
| `service/refresh_coordinator.go:133-145` | worker: `WithoutCancel(reqCtx)` + 30s 超时 | SIGTERM 取消不了 |
| `service/token_manager.go:272-274` | `sharedOperationContext`: `WithoutCancel` + 12s | SIGTERM 取消不了 |
| `main.go:86-88,140-148` | `gracefulShutdownTimeout=50s`；stopBackground → Shutdown → WaitBackground | 尾延迟被动 |

## 目标 API

```go
// Run starts at most one warmup scheduler and blocks until ctx is canceled.
// Second call returns ErrAlreadyRunning (or equivalent sentinel).
func (s *ClassroomService) Run(ctx context.Context) error

// Shutdown cancels the lifecycle, waits for the scheduler to exit, then
// drains refresh workers; all bounded by ctx. Idempotent.
func (s *ClassroomService) Shutdown(ctx context.Context) error
```

main.go 组合（顺序保持审计前语义：先停调度器，再排空 HTTP，最后排空 worker）：

```go
appCtx, stop := context.WithCancel(context.Background())
defer stop()
go func() { runErr <- app.classroomService.Run(appCtx) }()  // 或 main 直接阻塞式包装
...
case sig := <-stop:
    stop()                    // ① lifecycle 取消：调度器退出 + in-flight JW 中止
    ctx := context.WithTimeout(Background, gracefulShutdownTimeout)
    server.Shutdown(ctx)      // ② 排空 HTTP（等待者因刷新取消而快速返回）
    app.classroomService.Shutdown(ctx)  // ③ 幂等兜底：等 scheduler done + 排空 workers
```

注：`Shutdown` 内部对已取消的 lifecycle 不重复 cancel；`Run` goroutine 由 main
持有，`Shutdown` 只做 join + drain，不再拥有调度器生命周期。

## 状态模型（字段收敛）

```go
backgroundMu    sync.Mutex
lifecycleCtx    context.Context    // nil until Run; 由 Run 写入一次
lifecycleCancel context.CancelFunc // Run 写入；Shutdown 调用
schedulerDone   chan struct{}      // Run defer close
```

- `backgroundStopping` 判定改为：`lifecycleCtx != nil && lifecycleCtx.Err() != nil`
  （`startClassroomRefresh` 在 `backgroundMu` 下检查）。**lifecycleCtx == nil ⇒ 允许
  worker**，保持「未 Run 也可按需刷新」的现有语义（验收 6）。
- `warmupStarted` 判定改为：`lifecycleCtx != nil`。`Run` 二次调用在此判定后返回错误。

### ClassroomService 拥有 lifecycle，TokenManager 绑定引用

`Run` 在写入自身 lifecycle 字段的同时调用
`s.tokenManager.bindLifecycle(lifecycleCtx)`（`TokenManager.mu` 下存 ctx 引用，不复制
cancel——取消只经 `ClassroomService.Shutdown`，单一所有权）。
`sharedOperationContext` 从自由函数改为 `TokenManager` 方法：

```go
func (m *TokenManager) sharedOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
    opCtx, cancel := context.WithTimeout(context.WithoutCancel(nonNilContext(ctx)), jwRequestTimeout)
    m.mu.Lock(); lifecycle := m.lifecycleCtx; m.mu.Unlock()
    if lifecycle == nil || lifecycle.Err() != nil {
        return opCtx, cancel
    }
    stop := context.AfterFunc(lifecycle, cancel)
    return opCtx, func() { stop(); cancel() }  // 复合 cancel 防 AfterFunc goroutine 泄漏
}
```

## 取消传播（B-01 核心）

刷新 worker（`refresh_coordinator.go`，锁序与 WaitGroup 语义不变）：

```go
parent := context.WithoutCancel(nonNilContext(ctx))   // 保留 log_id 等 values
go func() {
    defer s.refreshWorkers.Done()
    refreshCtx, cancel := context.WithTimeout(parent, ClassroomRefreshLimit)
    defer cancel()
    if lc := s.currentLifecycle(); lc != nil {        // backgroundMu 快照
        stop := context.AfterFunc(lc, cancel)         // SIGTERM → 取消 in-flight JW
        defer stop()
    }
    result := s.refreshTodayClassrooms(refreshCtx)
    s.finishClassroomRefresh(attempt, result)
}()
```

效果链：SIGTERM → lifecycle cancel → refreshCtx cancel → `queryCampus` HTTP 中止 →
`finishClassroomRefresh` 关闭 attempt.done → HTTP 等待者拿到失败/部分结果快速响应 →
`server.Shutdown` 排空变快。token 登录/API URL 同理（12s 预算内立即中止）。

**保持不变**：单个 waiter 的 ctx 取消仍被 `WithoutCancel` 剥离，不影响共享操作
（验收 5）；`staleRefreshWait`/backoff 阶梯/抑制指标全部不动。

## 测试迁移地图

| 现测试 | 迁移 |
|---|---|
| `warmup_test.go` TestStartWarmupRunsImmediately…/WithCanceledContext…/SecondCallIsNoOp | 改 `go svc.Run(ctx)` + `svc.Shutdown(ctx)`；二次调用断言返回错误 |
| `warmup_test.go` TestWaitBackgroundPreventsNewRefreshWorkers | 改名 TestShutdownPreventsNewRefreshWorkers；断言 lifecycle 取消后 `startClassroomRefresh` 拒绝 |
| `testsupport_test.go` cleanup `WaitBackground` | 改 `Shutdown`（幂等，容忍未 Run 的 service） |
| `realtime_data_test.go:292` `StartWarmup(Background)` | `go svc.Run(runCtx)` + cleanup |
| `export_test.go` `warmupSchedulerDone`/`isBackgroundStopping` | 改 `schedulerDone(svc)` / `isLifecycleCanceled(svc)` 快照缝 |
| 新增 | in-flight 刷新在 lifecycle cancel 后及时返回的取消传播测试（mock JWClient 阻塞在 `ctx.Done()`）；token 共享登录同型测试 |
| `main_test.go` | 保留超时断言；可补 Run/Shutdown 组合编译级断言 |

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| AfterFunc goroutine 泄漏 / 双 cancel | 复合 cancel `stop(); cancel()`；`-race` + 现有 drain 测试 |
| Shutdown 与并发 `startClassroomRefresh` 竞态 | 沿用 `backgroundMu` 下检查 lifecycle Err 再 Add 的既有模式 |
| 测试时钟/时序抖动 | 迁移测试全部用 channel 同步，不 sleep 依赖真实时间 |
| 关机中刷新被取消导致缓存留下 partial | 现有 outcome 契约已覆盖（failed/partial 路径不变） |
| 回滚 | 单 commit revert，无数据/契约迁移 |

## 明确不改

`gracefulShutdownTimeout`(50s) / `httpWriteTimeout`(45s) 数值、B-06 singleflight 迁移
（仅补文件头注释）、B-03 冷路径行为。
