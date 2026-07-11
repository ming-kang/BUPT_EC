# 前端重载截止时间与抖动修复设计

## Scheduling Model

nextReloadDelay 分成三个明确阶段：

~~~text
response state + failureCount + now
  → base delay by state
  → one-sample positive bounded jitter
  → absolute stale_until clamp
  → timer
~~~

base delay owner 仍是 reloadSchedule.js，不把 policy 分散到 React hook。

## Jitter Helper

建议把当前 JITTER_RATIO 改为语义明确的最大正向比例，并增加绝对上限：

~~~text
sample = normalizeRandom(random called once)
spread = min(base * 0.10, 5000ms)
jittered = base + sample * spread
~~~

- sample=0 返回 base；
- sample=1 返回 base + spread；
- invalid sample 使用 0.5；
- delay 为 null、非有限或非正时保持现有 defensive 行为。

正向 jitter 保证 rate-limit 最小间隔不会被缩短，同时仍能打散多 tab 请求。

## Hard Deadline Clamp

当 data 是当前上海业务日且 stale_until 有效：

~~~text
hardDelay = max(0, staleUntilMs - nowMs)
return min(jitteredDelay, hardDelay)
~~~

clamp 必须在 jitter 后执行。若 hardDelay 小于基础间隔，业务数据失效优先于限流间隔。
invalid/cross-day payload 继续走当前快速重新获取路径，但 timer callback 必须先清除旧数据。

## Visibility State

React effect 继续以 pageVisible、spinning、reloading 为 gate。增加生命周期测试，而不是
引入第二套 scheduler：

1. hidden 时 effect cleanup 清 timer。
2. fake clock 推进到 stale_until 之后。
3. visibilitychange 到 visible。
4. effect 重新计算 invalid state 并只 enqueue 一个 reload。
5. 发起 fetch 前 UI 已清除旧 campuses。

如 React 相同 state 更新不足以保证去重，可用 generation/ref 记录最后一次 visibility
enqueue，但只有测试证明需要时才增加该状态。

## Test Matrix

| Case | Expected |
| --- | --- |
| sample 0 / 0.5 / 1 | base / bounded midpoint / bounded max |
| random spy | exactly one call |
| random throws/NaN/Inf | deterministic finite fallback |
| stale_until before base | exact hard deadline |
| stale_until between base and jittered | clamp to hard deadline |
| fresh expiry well before midnight | base at expiry plus bounded positive jitter |
| hidden crosses deadline | no hidden fetch; visible clears and reloads once |

## Compatibility and Rollback

- hook API 和 response shape 不变。
- 若 positive-only jitter 需要回滚，stale_until post-jitter clamp 和 single-sample fix 必须保留。
- 本任务在同一 commit 更新 owning API spec、operations/AGENTS 和 CHANGELOG；最终
  hygiene 子任务只做跨任务一致性审计。
