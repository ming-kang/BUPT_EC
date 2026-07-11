# 自适应刷新退避抖动与确定性测试实施计划

## 0. Baseline

- [x] 固定现有 total/partial/full coordinator 状态测试。
- [x] 列出所有 svc.now 直接赋值测试和所有 production time.Now/time.Since 调用。
- [x] 确认 metrics 子任务接口已稳定，避免同时修改 ClassroomServiceOptions。

## 1. Single Clock seam

- [x] 删除 ClassroomService 的可变 now func 字段，改为读取 injected Clock 的方法。
- [x] 保持 Asia/Shanghai 转换集中在 service now method。
- [x] 新增 thread-safe fakeClock test helper。
- [x] 把跨日、TTL、status、refresh、login 时间测试迁移到 options.Clock。
- [x] 搜索并消除测试对 svc.now 的直接赋值。

## 2. Jitter policy

- [x] 定义 RandomSample/BackoffRandom 可选注入和 production 默认值。
- [x] 保留 totalFailureBackoffBase 纯阶梯。
- [x] 实现 sample normalization、对称 offset、5s 上限和正 duration 防御。
- [x] 在 finishClassroomRefresh total-failure 分支只采样一次。
- [x] 保持 partial 固定 30s、full reset 和 metrics 顺序。

## 3. Deterministic tests

- [x] base ladder 30s/1m/2m/5m/cap。
- [x] sample=0、0.5、1 的最小/基准/最大 effective delay。
- [x] NaN、Inf、负数和大于 1 样本。
- [x] 连续 total failures、full reset、partial 不升级。
- [x] suppression 不创建 worker且只计一次。
- [x] request/warmup/concurrent caller 共享状态。
- [x] 午夜前失败、跨午夜 suppression、nextAllowed 到达后的新日成功。
- [x] race test 覆盖 fake clock 和 coordinator 并发读取。

## 4. Validation

~~~powershell
$env:GOTOOLCHAIN='go1.25.12'
gofmt -l .
go vet ./...
go test -race ./service ./...
go build ./...
git diff --check
~~~

### Validation results (2026-07-11)

| Check | Result |
| --- | --- |
| `gofmt -l .` | clean |
| `go vet ./...` | pass |
| `go test -race ./...` | pass |
| `go build ./...` | pass |
| `git diff --check` | pass |

Directed: ladder/jitter bounds, single sample, suppression, concurrent readers, midnight crossing, fakeClock race.

## 5. Contract sync and evidence

- [x] 更新 runtime/error/quality specs、operations、AGENTS 的 Clock/backoff 合同。
- [x] 在同一实现 commit 更新 CHANGELOG [Unreleased]。
- [x] 记录 fake Clock/jitter/race 验证结果和实现 commit 后再归档。

## Review Gates

- jitter bounds 必须由 policy clamp，不能信任 injected function。
- random 每个 total outcome 只调用一次。
- partial 不能继承 total jitter 或递增 failure count。
- Clock refactor 后 TokenManager 与 ClassroomService 必须持有同一实例。
- 不以 sleep 作为核心状态断言。
- 跨午夜测试必须同时断言旧 cache 被拒绝和新 refresh 最终可启动。

## Rollback Points

- Clock field-to-method migration。
- RandomSample injection。
- jitter policy/state integration。
- 如果 production jitter 回滚，fake Clock 和确定性状态测试仍应保留。
