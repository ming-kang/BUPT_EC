# PRD: 生命周期 Run/Shutdown + 取消传播（lifecycle-run-shutdown）

来源：`08-07-project-audit-optimize` backlog 优先级 1（B-01 + B-02，批次④唯一强继续项）。

## 问题陈述

1. **B-01 关机尾延迟不可预测**：共享刷新 worker 与 token 共享操作都用
   `context.WithoutCancel(reqCtx)` + 独立超时（`service/refresh_coordinator.go:133-139`、
   `service/token_manager.go:272-274`）。SIGTERM 取消不了任何 in-flight JW 操作，
   关机最坏要等完整刷新预算（`ClassroomRefreshLimit` 30s）+ HTTP 排空；
   systemd `stop` 默认 90s 内虽不至于必杀，但尾延迟完全取决于运气。
2. **B-02 生命周期 API 是侧门**：`StartWarmup` fire-and-forget + `WaitBackground`
   stop-and-drain 两个半方法，靠 `ClassroomService` 上 5 个互斥锁字段
   （`backgroundMu`/`backgroundStopping`/`warmupStarted`/`warmupDone`/`warmupCancel`，
   `service/classroom_service.go:61-66`）拼装，「谁负责 cancel」认知成本高。

## 目标

- `ClassroomService` 暴露标准 `Run(ctx) error` / `Shutdown(ctx) error` 生命周期 API
  （类比 `http.Server` 的 `ListenAndServe`/`Shutdown`），收敛侧门字段。
- 进程收到 SIGTERM/SIGINT 后，lifecycle 取消**传播进**所有 in-flight JW 共享操作
  （课堂刷新 worker、token 登录、API URL 拉取），同时保留请求级 values（log_id）。
- 关机排空语义不回退：lifecycle 取消后拒绝新增刷新 worker；已有 worker 与 HTTP
  排空顺序保持「先停调度器 → 再排空 HTTP → 最后排空 worker」。

## 非目标

- 不改任何 HTTP API / wire 契约（审计判定契约影响：无）。
- 不做 B-03 冷路径有界等待 / 503，不改 `httpWriteTimeout` / `gracefulShutdownTimeout`
  数值（可作后续小任务）。
- 不迁移 singleflight（B-06 已降级）；仅在协调器文件头补「为何不用 x/sync/singleflight」注释。
- 不引入 `context.Merge` 类第三方依赖。

## 验收标准

1. `service.ClassroomService` 公开 API 为 `Run(ctx) error` 与 `Shutdown(ctx) error`；
   `StartWarmup` / `WaitBackground` 删除，`main.go` 改用新 API 组合生命周期。
2. `Run` 每个 service 至多启动一个调度器，首次刷新立即尝试；重复调用返回错误而非静默 no-op。
3. `Run` 阻塞到 ctx 取消后返回（调度器 goroutine 由调用方或 Run 本体拥有，语义明确）。
4. lifecycle 取消后：
   - in-flight 刷新 worker 的 JW 调用收到取消（不需要等满 30s 预算）；
   - in-flight token 登录 / API URL 共享操作收到取消；
   - `startClassroomRefresh` 拒绝新 worker（语义等同现 `backgroundStopping`）。
5. 单个 waiter 取消**仍然不能**取消共享操作（既有行为保持；lifecycle 是唯一额外取消源）。
6. 未调用 `Run` 的 service（纯按需刷新、单测路径）行为不变：刷新与 token 操作照常工作。
7. `go test -race ./...` 全绿；`gofmt` / `go vet` 干净。
8. `.trellis/spec/backend/runtime-state-and-cache.md` 的
   「Warmup Scheduler and Background Shutdown」场景签名与契约同步更新；
   `CHANGELOG.md` Unreleased 记录关机行为变化（运维可见）。

## 约束

- 遵循 `runtime-state-and-cache.md`：可变状态全部挂在 `ClassroomService`，无包级状态。
- `context.AfterFunc` 是唯一允许的取消桥接手段（stdlib）；操作正常完成后必须
  `defer stop()` 防 goroutine 泄漏。
- 锁序不变：`backgroundMu` → `refreshMu`；不得在持锁时发起网络调用。
