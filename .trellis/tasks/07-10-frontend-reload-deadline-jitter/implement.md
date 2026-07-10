# 前端重载截止时间与抖动修复实施计划

## 0. Characterization

- [ ] 新增 random=1 时超过 stale_until 的失败测试。
- [ ] 新增 random spy，证明当前 helper 调用两次。
- [ ] 固定现有 stale/partial/failure/fresh 和 hidden lifecycle 基线。

## 1. Pure scheduling helper

- [ ] 将 random 调用结果缓存为单个 sample。
- [ ] 增加 throw/NaN/Inf/越界 normalization。
- [ ] 将 jitter 改为有界正向分布，保留部署最小间隔。
- [ ] 在 jitter 后执行 stale_until absolute clamp。
- [ ] 对 null、invalid data 和负 hard deadline 保持防御性返回。

## 2. Lifecycle behavior

- [ ] 覆盖 hidden 时 timer cleanup。
- [ ] 覆盖 hidden 跨越 stale_until 后 visible。
- [ ] 断言旧 campuses 在 background fetch 前清除。
- [ ] 断言多次相同 visibility event 只触发一次 fetch。
- [ ] 复核 manual retry、timeout、unmount 和 superseded abort。

## 3. Regression matrix

- [ ] ordinary stale 最小 15s。
- [ ] partial 最小 30s。
- [ ] failure 10/20/30/60 cap。
- [ ] fresh expires_at 调度。
- [ ] stale_until 早于 expiry/backoff/jitter。
- [ ] 上海午夜边界。
- [ ] random=0/0.5/1 和 invalid source。

## 4. Validation

~~~powershell
cd frontend
pnpm lint
pnpm test
pnpm build
pnpm audit:prod
pnpm audit:dev
cd ..
git diff --check
~~~

## 5. Contract sync and evidence

- [ ] 更新 API spec、operations、AGENTS 的最终 delay/jitter/deadline 合同。
- [ ] 在同一实现 commit 更新 CHANGELOG [Unreleased]。
- [ ] 记录 pure helper、lifecycle、build/audit 结果和实现 commit 后再归档。

## Review Gates

- deadline clamp 必须应用于最终 delay，不接受只 clamp base。
- random 每次 helper invocation 只调用一次。
- jitter 不得缩短部署约定的最小间隔。
- hidden/visible 测试必须真实 mount hook，不只测试纯 helper。
- 不把 deadline policy复制到 useTodayClassrooms。

## Rollback Points

- random normalization。
- positive bounded jitter。
- post-jitter hard deadline clamp。
- visibility lifecycle test/ref 去重机制。
