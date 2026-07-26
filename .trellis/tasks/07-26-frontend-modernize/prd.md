# 前端现代化：依赖升级、SWR、原生表格、chunk 拆分

父任务：`07-26-arch-simplify-refactor`（批次③前端）。复杂任务：start 前需 design.md + implement.md。
证据来源：父任务 `research/audit-frontend.md`。
依赖顺序：与子任务 2/3 无代码冲突，可在子任务 1 之后任意时点执行；建议放最后（升级后需人工视觉回归）。

**已确认决策**：antd 处理为 **A 档**（只删 Table + icons + 拆 chunk；B/C 档留待后续任务）。React 本次升 18.3.1（React 19 与 TypeScript 迁移一起属批次④，因 prop-types 在 19 中失效）。

## Goal

刷新锁死在 2023-12 的生产依赖，用 SWR 替换 139 行手写数据层，antd Table 换原生表格，修复两个真实正确性缺陷（Modal 过期数据、effect 派生状态闪烁）。

## Requirements

依赖升级（先行，独立提交）：
- R1 `pnpm update`：antd → 最新 5.x，react/react-dom → 18.3.1，@testing-library 等随动；Vite → 7（确认 @vitejs/plugin-react 兼容）；升级后评估删除 pnpm.overrides 中已不需要的补丁条目；删除 `dayjs`（若子任务 1 未做）。vite 版本前缀策略统一并加注释。
- R2 升级后 `pnpm test`、`pnpm lint`、`pnpm build` 全绿，人工视觉回归（亮/暗色、移动端宽度）。

数据层：
- R3 引入 `swr`，重写 `useTodayClassrooms`：`refreshInterval: (data) => nextReloadDelay(...)`、`keepPreviousData`、`onErrorRetry` 用现有 failureRetryDelay 阶梯、fetcher 内 `AbortSignal.timeout(40_000)`。**保留** `reloadSchedule.js`、`classroomDataValidity.js`、`todayClassroomsResponse.js` 领域逻辑及其测试。
- R4 normalizeResponse 改为返回值而非 throw 控制流，保留真实 HTTP status；顺带透传后端 `log_id` 并在 GlobalEmpty 错误态展示（低成本运维收益）。
- R5 现有 `useTodayClassrooms.*.test.jsx` 生命周期语义（可见性暂停、退避、stale 保留快照）在 SWR 化后仍有等价测试覆盖。

antd A 档：
- R6 `TodayClassroomTable` 主表改原生 `<table>`，复用 `.room-info__table` 样式体系；删除 `.ant-table-*` 覆盖与相关 `!important`。
- R7 内联 2 个 SVG 图标（SettingOutlined、GithubOutlined），删除 `@ant-design/icons` 依赖。
- R8 `vite.config.js` manualChunks 增加 antd-vendor 分组（antd/rc-*/@ant-design）；接入 rollup-plugin-visualizer（仅本地/CI 报告）；CI 加构建产物 gzip 体积预算断言。

正确性修复：
- R9 Modal 4 state 合一为 `activeRoomKey`，内容实时派生（修后台刷新后 Modal 显示过期空闲节次的 bug）。
- R10 消除两处 effect 派生状态：App.jsx:74-83 campus 选择改渲染期派生（消除数据到达后的空帧闪烁）；ClassTimePicker.jsx:47-72 裁剪改渲染期派生（消除隐式死循环契约）。
- R11 ClassTimePicker 时钟在 visibilitychange → visible 时重同步 `now`。

次要（时间允许则做，可裁剪）：
- R12 抽 `<ToggleButtonGroup>` 统一三个 Picker 及其 CSS，补 `aria-pressed`。
- R13 顶层 ErrorBoundary + componentDidCatch console.error。

## Out of Scope

- React 19、TypeScript 迁移、prop-types 移除（批次④）。
- antd B/C 档（替换 Card/Typography 等或全移除）。
- 楼栋 display_name 下沉后端（涉及后端契约，另议）。
- 组件测试全面补齐（仅为本次改动的组件补必要测试）。

## Acceptance Criteria

- [ ] `pnpm-lock.yaml` 中 antd ≥ 5.20、react = 18.3.x、vite = 7.x；`pnpm audit --prod` 通过。
- [ ] `pnpm test`、`pnpm lint --max-warnings 0`、`pnpm build` 全绿。
- [ ] 构建产物：antd 独立 chunk；主 chunk 因业务改动变化时 antd-vendor hash 不变；总 gzip 体积相比升级前不增加（预期显著下降，visualizer 报告佐证）。
- [ ] 功能回归清单人工确认：校区/楼栋/节次选择与持久化、stale Alert、错误态重试、Modal 打开时后台刷新内容跟随更新、暗色模式、移动端布局。
- [ ] `package.json` 无 dayjs、无 @ant-design/icons；源码无 `.ant-table` 覆盖样式。
- [ ] 数据层行为等价：页面隐藏暂停轮询、失败退避阶梯、失败时保留上次快照并显示 stale 提示（测试覆盖）。
