# Implement: lifecycle-run-shutdown

执行顺序（每步后跑对应验证；步骤 1-4 为核心实现，5-6 为配套迁移，7 为收尾）。

## 1. ClassroomService 状态收敛（B-02 前半）

- [x] `service/classroom_service.go`：删除四侧门字段，新增 `lifecycleCtx/lifecycleCancel/schedulerDone`（`backgroundMu` 守护）。
- [x] 哨兵错误 `ErrAlreadyRunning`（warmup.go）。

## 2. Run / Shutdown（B-02 后半）

- [x] `StartWarmup` → 阻塞式 `Run(ctx) error`（立即首刷；预取消 ctx 不启 worker；defer 在 backgroundMu 下 cancel + close(schedulerDone)；返回 nil）。
- [x] `WaitBackground` → `Shutdown(ctx) error`（backgroundMu 下 cancel → 等 schedulerDone → 排空 refreshWorkers；幂等；未 Run 直接排空返回 nil）。
- [x] `Run` 写 lifecycle 字段时调用 `tokenManager.bindLifecycle`。

## 3. 取消传播（B-01）

- [x] worker：`WithoutCancel` 保留 values + `AfterFunc(lifecycle, cancel)` + `defer stop()`；lifecycle 取消判定替换 `backgroundStopping`。
- [x] 文件头补「为何不用 x/sync/singleflight」注释。
- [x] `sharedOperationContext` 改 TokenManager 方法（lifecycle 非 nil 即挂 AfterFunc，含已取消态）；`bindLifecycle`；`lifecycleCtx` 入 mu 守护组。
- [x] 审查修复：lifecycle 已取消时不再跳过 AfterFunc（关机期间新操作不再拿满 12s 预算）；testsupport 日志文案同步。

## 4. main.go 接线

- [x] `go Run(appCtx)` + `stopLifecycle()` → `server.Shutdown` → `service.Shutdown` → 读 runErr。
- [x] `main_test.go` 两个超时断言保留不动。

## 5. 测试迁移与新增

- [x] 迁移 warmup_test.go 四用例 + testsupport cleanup + realtime_data_test.go + export_test.go 缝（schedulerDone/isLifecycleCanceled）。
- [x] 新增 TestShutdownCancelsInFlightRefreshWorker / TestShutdownCancelsSharedTokenLogin / TestOnDemandRefreshWorksBeforeRun。
- [x] 修复 realtime 迁移用例 runCancel 与 release 的竞态（先收结果再取消）。

## 6. 验证命令

- [x] gofmt / go vet / `go test -race ./...`（含 -count=3 service 包）全绿；`go build -tags embed_assets` 通过；前端 lint/test 通过（pnpm audit 失败为存量 devDependency 漏洞，与本次无关）。

## 7. 收尾（Phase 3）

- [x] spec：runtime-state-and-cache.md（场景重写 + 刷新协调 + token 场景）与 directory-structure.md 同步。
- [x] CHANGELOG.md Unreleased：SIGTERM 取消传播 + Run/Shutdown。
- [x] docs/development.md 残留 WaitBackground 清理。
- [x] Conventional Commit：`refactor(service): collapse lifecycle into Run/Shutdown with cancellation propagation`。

## 回滚点

- 每个步骤独立可编译；最终单 commit 交付，revert 即回滚。
- 步骤 3 若 AfterFunc 引入不稳定，可先合入 1/2/4/5（API 收敛本身独立有价值），
  取消传播单独成 commit 二次交付。
