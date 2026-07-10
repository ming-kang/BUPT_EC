# 前端请求、状态与启动生命周期设计

## Request State Machine

保留现有 hook API，但将请求资源显式建模：

```text
reload request
  -> AbortController
  -> timeout timer (40s)
  -> fetch
  -> normalized success | timeout failure | transport failure
cleanup: clear timeout + abort controller
```

建议提供纯 helper 区分 abort 原因，避免 unmount abort 被显示为错误。不得依赖浏览器私有
错误字符串。

## Visibility-aware Scheduler

新增小型 hook/helper 管理：

- `visibilitychange` listener；
- hidden 时清除 reload timer；
- visible 时调用 `nextReloadDelay` 重新计算；
- deadline 已过则 enqueue 一次 background reload；
- 使用 ref 防止同一 visible event 重复 enqueue。

调度策略仍由 `reloadSchedule.js` 纯函数拥有，React effect 只负责资源生命周期。

## Rate-aware Policy

建议基线：

| State | Delay |
| --- | --- |
| ordinary stale | 15s + bounded jitter |
| partial campus | at least 30s + jitter |
| hard/client failure | 10s, 20s, 30s, then 60s cap |
| fresh | expires_at deadline |
| invalid/cross-day | hard failure policy |

jitter 必须可注入测试，不改变 `stale_until` 绝对上限；若缓存到期，UI 先清除不可用数据。

## Pure Persistence

```text
initSelectionState -> safe read existing keys
selectionReducer   -> pure state transition only
SelectionProvider effect -> safe write changed preference fields
```

effect 应避免把无关 selection 更新写入 storage，可比较对应字段或拆分 effects。

## CSP Bootstrap

新增同源 module entry（例如 `darkModeBootstrap.js`）并在 main entry 前引用。它导入
`darkMode.js` 的相同 helper，在 DOM 可用时应用 body class。Vite production build 必须
输出 CSP `script-src 'self'` 可加载的外部资源，不添加 `unsafe-inline`、nonce 或动态 eval。

## Compatibility and Rollback

- `useTodayClassrooms` 返回结构、selection actions 和 storage key 不变。
- 请求频率会降低，这是有意的部署稳定性变化。
- CSP、request scheduler、persistence 可分层实现，但最终验证需覆盖它们的 effect 交互。
