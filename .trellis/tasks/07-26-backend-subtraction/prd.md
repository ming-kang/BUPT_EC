# 后端减法：删 cache 包、Noop 指标、死代码清理

父任务：`07-26-arch-simplify-refactor`（批次②）。复杂任务：start 前需 design.md + implement.md。
证据来源：父任务 `research/audit-backend.md`（条目 S1、S4-S7、S9、S11、S12、O8 部分）。
依赖顺序：建议在子任务 1（hygiene-quick-wins）之后、子任务 3（degin-http-rewrite）之前执行——本任务收缩 service 层内部，子任务 3 重写 HTTP 层，分开减少冲突面。

## Goal

删除 service/cache 层的冗余抽象与死代码，对外行为（API 响应、指标名、日志格式）不变。预计净删 ~300 行生产代码 + go-cache 依赖。

## Requirements

- R1 删除 `cache/` 包与 `patrickmn/go-cache` 依赖：`service` 内改用 `atomic.Pointer[model.TodayClassrooms]`（含 UpdatedAt 判断），删除 `TodayClassroomCache` 接口（classroom_service.go:37-40）与 `cacheExpiration`（realtime_data.go:281-291）。**不做**过期模型收敛（Date/ExpiresAt/StaleUntil/Stale 字段与判断逻辑保持现状，属批次④）。
- R2 Metrics 默认注入 `NoopMetrics`（NewClassroomService options 处理，与 Clock/BackoffRandom 风格一致），删除 20+ 处 `!= nil` 判空与 4 个 observeXxx 包装、PrometheusMetrics 各方法的 nil-receiver 检查。
- R3 删除死代码与仅测试可达 API：`model.QueryResponse`、`logs.LogIDKey`、`jw_error.go:66-69` ctx 死分支、`forceRefresh`/`loginPerformed`/`ClassroomService.Login`（集成测试改直接调 JWClient）、`QueryOne`/`QueryAll` 改非导出或移入测试。
- R4 删除 `errgroupNoCancel`（realtime_data.go:302-317）改 `sync.WaitGroup` 内联；全仓清理 Go 1.22 前的循环变量重绑定。
- R5 合并两套退避阶梯（refresh_coordinator.go:41-46 与 warmup.go:30-41）为单一 `backoffLadder(attempt)`。
- R6 三份 reflect 判空工具（handler.go:29-40、service/dependency.go:5-16、utils/http.go:109-120）删除或收敛为一份。
- R7 小项：now() 冗余判空、runtime_status.go `copy := t` 遮蔽内建（字段改 time.Time + omitzero，删 cloneTime）、addQuery 内联、`hasJWCredentials func() bool` 改 bool、`interface{}` → `any`。
- R8 测试收敛（为本任务及子任务 3 铺路）：抽 `export_test.go` 种子函数（seedCache/forceFailure 等），`warmupJitter` 改为 ClassroomServiceOptions 字段；`realtime_data_test.go` 拆分，需凭据的集成测试加 `//go:build integration`。

## Out of Scope

- 过期模型收敛为 freshness 三态（批次④）。
- 刷新协调器改 singleflight（S3，批次④——依赖过期模型收敛后一起做更安全）。
- 生命周期 Run/Shutdown 改造（O1/O2，批次④）。
- utils 并入 service / internal/httpapi 迁移（可在子任务 3 顺带评估）。

## Acceptance Criteria

- [ ] `go.mod` 无 `patrickmn/go-cache`；`go mod tidy -diff` 干净。
- [ ] `grep -r "gocache\|go-cache" --include="*.go"` 无命中；`cache/` 目录不存在。
- [ ] `/api/get_data`、`/healthz`、`/readyz`、`/metrics` 响应结构与字段与改造前一致（handler_test、metrics_endpoint_test 不改断言即通过，或仅因内部构造方式变化调整测试装配代码）。
- [ ] Prometheus 指标名与标签完全不变。
- [ ] `go test -race ./...` 全绿；`go vet ./...` 干净；集成测试 `go test -tags integration` 可单独跳过/运行。
- [ ] 净删行数为正（生产代码显著减少）。
