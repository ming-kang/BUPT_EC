# 前端请求、状态与启动生命周期

## Goal

增加请求超时与可见性调度，协调轮询限流，移除 reducer 持久化副作用并解决 CSP 下的深色启动脚本。

## Background

- `frontend/src/useTodayClassrooms.js:93-139` 只有 unmount/reload abort，没有显式请求
  timeout；上游或代理悬挂时浏览器可长期保持 in-flight。
- `useTodayClassrooms.js:165-186` 始终设置 timer，不检查 `document.visibilityState`。
- `reloadSchedule.js` 的普通 stale 5 秒轮询和失败 5/10/20/30 秒退避，在多用户/多 tab
  共享出口 IP 时可能与 Nginx 30 requests/minute 限流竞争。
- `selectionContext.js:28-52` 在 reducer 内写 localStorage，使 reducer 非纯并增加严格模式/
  测试重放风险。
- `frontend/index.html:13-28` 使用 inline 深色 bootstrap，而生产 CSP
  `script-src 'self'`（`scripts/install.sh:496`）会阻止它。

## Dependencies

- `07-10-protocol-lifecycle-test-coverage` 应先建立真实 hook mount/unmount、timer 和 fetch
  lifecycle 测试 harness。
- 若与 installer timeout/CSP 文件发生冲突，本任务排在两个 installer P1 任务之后。

## Requirements

### R1 — 有界 fetch 生命周期

- 每个 `/api/get_data` 请求拥有显式客户端 timeout，推荐 40 秒：高于 30 秒 backend
  refresh limit，低于 45/60 秒 server/proxy 外层预算。
- timeout、component unmount、下一次 reload 都通过同一个 AbortController 收敛。
- timeout 显示安全、可理解的请求超时文案；unmount abort 不更新 state。
- timer/controller 在 effect cleanup 中全部释放。

### R2 — 页面隐藏时停止自动轮询

- hidden 状态不启动新的后台 fetch，也不保留无意义 reload timer。
- 页面重新 visible 时重新计算 snapshot 有效性和下一次 deadline；若已到期则立即触发
  一次 background reload。
- 用户显式 retry 可以继续工作，不被 visibility policy 永久阻塞。

### R3 — 与部署限流协调

- 自动 stale/失败轮询使用集中策略和小幅 jitter，避免固定 5 秒同步请求。
- 推荐普通 stale 最短 15 秒、partial 最短 30 秒、hard failure 10/20/30/60 秒封顶；
  实施前用 Nginx 30r/m + burst 20 合同复核。
- 成功响应重置失败计数；手动 retry 不被自动最小间隔吞掉。

### R4 — reducer 保持纯函数

- `selectionReducer` 不直接访问 localStorage 或其他外部系统。
- 初始化仍安全读取现有 `showClassTime` / `canSelectAllDay` keys，保持用户偏好兼容。
- SelectionProvider effect 或显式 persistence adapter 在 state commit 后写入，存储失败不影响
  内存状态。

### R5 — CSP 合规的无闪烁深色启动

- 删除 inline executable script，改用 `script-src 'self'` 允许的同源 module/bootstrap。
- 启动逻辑复用 `darkMode.js`，不维护第二套偏好算法。
- production build 和生成 Nginx CSP 下，首帧在 React mount 前应用正确 body class。

### R6 — 文档和测试

- 增加真实 hook lifecycle、fake timer、visibility、timeout、persistence 和 CSP/build 回归。
- 同步 frontend/API/quality specs、deployment/operations/development、AGENTS 和 changelog。

## Acceptance Criteria

- [ ] 悬挂 fetch 在配置 timeout 后被 abort，UI 进入可重试状态且无 state-after-unmount。
- [ ] hidden 页面不继续自动轮询，visible 后只触发一次必要 reload。
- [ ] 自动请求计划符合集中 backoff/jitter 和 Nginx rate-limit 预算。
- [ ] `selectionReducer` 是确定性纯函数，现有 localStorage keys 和偏好保持。
- [ ] production CSP 不再阻止深色 bootstrap，且无重复深色算法。
- [ ] 现有 last-good-data、跨日失效、partial warning 和 full-page spinner 语义保持。
- [ ] frontend lint、真实 lifecycle tests、完整测试、build 和 audits 通过。
- [ ] installer render test 断言 CSP 与 bootstrap 兼容。

## Out of Scope

- 引入 React Query/SWR 等全新数据框架。
- 增加 service worker、PWA 离线缓存或跨 tab leader election。
- 修改后端刷新 TTL、业务日期或 Nginx 全局限流值。
