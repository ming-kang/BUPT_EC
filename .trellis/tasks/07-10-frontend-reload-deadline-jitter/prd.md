# 前端重载截止时间与抖动修复

## Goal

保证前端自动重载的 jitter 不会越过 stale_until 或上海业务日边界，随机源每次调度
只采样一次，并保持隐藏页面、失败退避和 last-good-data 生命周期合同。

## Background

- frontend/src/reloadSchedule.js:23 的 withJitter 在有限性检查和 sample 赋值时调用
  random 两次。
- nextReloadDelay 先用 stale_until - now 截断 base delay，最后在 line 84 再施加
  正向或负向 jitter。
- 当 random=1 且 base 等于 stale_until 剩余时间时，最终 delay 可越过硬截止时间。
- 对称 jitter 还会把文档声明的 stale 至少 15s、partial 至少 30s 和 failure 基础间隔
  向下缩短。
- 现有测试只使用恒定 0.5 样本验证 deadline，未覆盖 random=1、随机调用次数或非法样本。
- lifecycle tests 覆盖 hidden 不轮询，但没有覆盖隐藏跨过 stale_until 后恢复可见。

## Dependencies

- 没有后端代码依赖；为降低并行冲突，按父任务顺序在 backend jitter 后实施。
- 最终 hygiene 子任务依赖本任务锁定的 delay/deadline 测试合同。

## Requirements

### R1 — 单次随机采样

- withJitter 每次调用最多读取一次 random。
- random 缺失、抛错、NaN、Inf 或越界时使用稳定 fallback，不得返回 NaN。
- helper 保持纯函数和可注入，生产默认 Math.random。

### R2 — 最小间隔与 jitter 边界

- ordinary stale 的基础最小间隔保持 15s。
- partial 的基础最小间隔保持 30s。
- hard/client failure 保持 10s、20s、30s、60s cap。
- jitter 使用非负延迟，不能把请求提前到基础最小间隔之前。
- 推荐正向 jitter 上限为 min(base 的 10%，5 秒)，避免长 fresh delay 产生过大漂移。

### R3 — stale_until 绝对上限

- 对仍可展示的 snapshot，最终 delay 必须小于等于 stale_until - now。
- clamp 必须发生在 jitter 之后，或 jitter helper 接收明确 hard limit。
- 当 hard limit 早于基础最小间隔时，业务截止时间优先。
- timer 唤醒时若 snapshot 已跨日或过 stale_until，先清除 campuses/filter/table，再开始
  background reload。

### R4 — Visibility 和 lifecycle

- hidden 页面不保留 reload timer，也不启动新的后台请求。
- 恢复 visible 时重新读取当前 now、snapshot 和 deadline，只排队一次必要 reload。
- 页面隐藏期间跨过 stale_until，恢复后不得继续展示旧 snapshot 等待普通 poll。
- manual retry 不受自动间隔限制，in-flight/timeout/unmount cleanup 合同保持。

### R5 — 测试与兼容性

- 纯函数测试覆盖 random=0/0.5/1、调用次数、非法 sample 和 deadline clamp。
- lifecycle 测试覆盖 hide → deadline passed → visible → clear + single reload。
- 不改变 useTodayClassrooms 返回结构、API envelope normalization、SelectionContext 或
  full-page spinner 语义。

## Acceptance Criteria

- [x] random spy 证明每次 delay 只调用一次随机源。
- [x] 非法 random 不产生 NaN、负 delay 或未捕获异常。
- [x] stale/partial/failure delay 不早于对应基础间隔，除非 stale_until 更早。
- [x] 任意 random sample 下最终 delay 都不超过 stale_until - now。
- [x] 上海午夜附近不会因正向 jitter 继续展示昨天 snapshot。
- [x] hidden 跨过截止时间后恢复 visible 只触发一次清理和 background reload。
- [x] 现有 last-good-data、timeout、manual retry、partial warning 和 spinner 测试继续通过。
- [x] frontend lint、全部 Vitest、production build、两级 audit 和 git diff --check 通过。

## Out of Scope

- 引入 React Query/SWR、service worker 或跨 tab leader election。
- 修改后端 TTL、Nginx rate limit 或 API payload。
- 取消所有 jitter；目标是安全限幅而不是恢复固定同步轮询。
