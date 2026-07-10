# 前端请求、状态与启动生命周期实施计划

## 0. Preconditions

- [ ] 完成或复用 `07-10-protocol-lifecycle-test-coverage` 的 hook lifecycle harness。
- [ ] 确认 installer P1 任务对 `scripts/install.sh` 的修改已稳定，避免并行冲突。

## 1. Fetch timeout

- [ ] 集中定义 client timeout，并与 backend/server/proxy budgets 写注释和 docs。
- [ ] 将 timeout、unmount 和 superseded request 收敛到 controller cleanup。
- [ ] 覆盖 timeout error、unmount abort、late response 和 manual retry。

## 2. Visibility and scheduling

- [ ] 把 delay/jitter 保持为纯函数并可注入时间/随机源。
- [ ] hidden 时清 timer，visible 时重算并去重 reload。
- [ ] 更新 stale/partial/failure delay，验证 `stale_until` 优先级。
- [ ] 测试多次 visibility events、in-flight 隐藏和恢复。

## 3. Pure persistence

- [ ] 从 reducer 删除 localStorage 写入。
- [ ] 在 provider effect/adapter 中持久化两个既有 keys。
- [ ] 覆盖 StrictMode 风格重复 render、storage throw 和无关 action。

## 4. CSP bootstrap

- [ ] 新增复用 `darkMode.js` 的同源 bootstrap entry。
- [ ] 删除 `index.html` inline executable script。
- [ ] 验证 Vite build 产物、CSP header 和首帧 body class。
- [ ] 更新 installer render test，禁止重新放宽 `script-src`。

## 5. Docs/spec/changelog

- [ ] 更新 frontend/API/quality specs、development、operations、deployment、AGENTS。
- [ ] 记录 polling、timeout、visibility 和 CSP 行为变化。

## Validation

```bash
pnpm --dir frontend lint
pnpm --dir frontend test
pnpm --dir frontend build
pnpm --dir frontend audit:prod
pnpm --dir frontend audit:dev
bash scripts/install_test.sh
shellcheck scripts/*.sh
git diff --check
```

## Review Gates

- 所有 timer/listener/controller 必须有 cleanup 测试。
- reducer 不得包含 I/O。
- CSP 不得通过加入 `unsafe-inline` 修复。
- 自动请求频率必须有部署限流计算依据。
- 不引入新的数据 fetching framework。

## Rollback Points

- Request timeout/abort。
- Visibility/rate schedule。
- Persistence effect。
- CSP bootstrap。

每个阶段先通过 focused lifecycle tests，再进入下一阶段。
