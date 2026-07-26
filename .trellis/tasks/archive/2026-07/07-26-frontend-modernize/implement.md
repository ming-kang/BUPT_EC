# 执行计划：前端现代化

前置：design.md 定稿。每步 = trellis-implement 派发（prompt 以 `Active task: .trellis/tasks/07-26-frontend-modernize` 开头）→ 主会话跑门禁 → 主会话提交。trellis-implement 不得 commit。

## 通用门禁（每步必跑，frontend/ 下）

```bash
pnpm lint          # --max-warnings 0 语义（脚本已含）
pnpm test
pnpm build
```

涉及依赖变更的步骤追加 `pnpm audit:prod && pnpm audit:dev`。Go 侧不受影响（web/dist 链路无文件名依赖，已核实），仅在收尾冒烟时联动验证一次。

## Step 1：依赖升级 A——运行时/UI 面（提交 ①）

- [ ] package.json：antd `^5.29.3`、react/react-dom `^18.3.1`、@types/react(-dom) 18.3 线、@testing-library patch 随动、eslint/@eslint/js `^9.39.5`、globals `^17.8.0`；**删除整个 pnpm.overrides 块**（4 条）。⚠️ 直接编辑 range 后 `pnpm install`，禁止 `pnpm update --latest`（antd6/eslint10 陷阱）。
- [ ] eslint.config.js:27 `react: { version: "18.2" }` → `"detect"`。
- [ ] 验证 lockfile 无 `brace-expansion@1.1.11`/`minimatch@3.1.2`/`flatted@3.2.9`/`@babel/runtime@7.2[0-5]`；audit 双绿（复红则单条恢复 override 并注明 CVE）。
- [ ] 门禁 + audit。预期零弃用警告（项目零弃用 props/defaultProps 命中）。

## Step 2：依赖升级 B——构建面（提交 ②）

- [ ] vite `^7.3.6`（顺带恢复 caret）、@vitejs/plugin-react `^5.2.0`；可选 jsdom `^29.1.1`（测试红了回 26 并在提交信息记录）。
- [ ] vite.config.js 显式 `build.target: ['es2020', 'safari14']`（design D2，拒绝 Safari 16 新默认）。
- [ ] 门禁 + `pnpm dev` 冒烟（proxy /api 正常）+ 构建产物 sanity（chunk 结构不变）。

## Step 3：R4 数据层预备（提交 ③）

- [ ] todayClassroomsResponse.js：normalizeResponse 去 throw 改判别返回值；toThrow 测试改写（~12 行）。
- [ ] 新 ApiError（status/code/logId 结构化）；现阶段仍由手写 hook 消费（throw 收敛点先落 fetch 辅助函数，SWR 化时直接复用）。
- [ ] GlobalEmpty 错误态展示 log_id（`payload.log_id || X-Log-Id 头`）+ 2-3 行 CSS + 测试。
- [ ] 门禁。

## Step 4：R3/R5 SWR 化（提交 ④，本任务最大风险步）

- [ ] `pnpm add swr@^2.4.2`。
- [ ] 重写 useTodayClassrooms：useSWR + 渲染期派生（mergeFetchResult/hasUsableClassroomData/shouldFullPageSpin 原样复用）；fetcher 用 Step 3 的 ApiError + `AbortSignal.timeout(40_000)`；配置照 design D3（稳定身份 refreshInterval 防 falsy 包装、retryDelayFor + 可见性自查的 onErrorRetry、focusThrottleInterval 15s、revalidateOnFocus 必须 true）。
- [ ] **先 spike**：fake timers × SWR 轮询链 + AbortSignal.timeout stub 各写一个最小测试确认可测性，再全面迁移。
- [ ] lifecycle 测试迁移（6 迁移/②改写/⑥stub；SWRConfig `provider: () => new Map()` + `dedupingInterval: 0` 样板）；新增防链死测试、retryDelayFor 单测。
- [ ] useTodayClassrooms.test.js 删 nextFailureCount 段。
- [ ] 门禁。**逃生出口**：触发 design D3 判据任一 → 本步改为手写层 useReducer 瘦身（消灭 respRef 渲染期写入与重复块、AbortSignal.timeout 化），删除 swr 依赖，汇报并修订 PRD R3/R5 注记。

## Step 5：R6/R7 原生表格 + 内联图标（提交 ⑤）

- [ ] TodayClassroomTable：antd Table → 原生 table（design D5；key/列语义/横向滚动等价；空态早退维持）。
- [ ] CSS：room-info 表核心改名 `.ec-table` 共享；删 `.ant-table-*` 8 段；`:129` 直删、`:145` 提特异性去 !important；**保留 `.ant-tag` 覆盖**；≤767/≤479 两档 media 用新类名重写。
- [ ] icons.jsx（SettingIcon/GithubIcon，path 在 research/antd-ui-fixes.md §3.2）；3 文件替换；package.json 删 @ant-design/icons。
- [ ] 新增 TodayClassroomTable.test.jsx（表头/行/free_nodes 过滤与 padStart/空态三分支）——如需 matchMedia stub 提前到本步铺 setupFiles。
- [ ] 门禁 + audit（依赖变更）。

## Step 6：R8 chunk 拆分 + visualizer + 体积预算（提交 ⑥）

- [ ] vite.config.js：ANTD_ID/REACT_ID 正则分组（design D6 写法）；visualizer（BUNDLE_REPORT=1 → `bundle-stats.local.html`，严禁 dist/ 内）。
- [ ] `frontend/scripts/check-bundle-size.mjs`（research/build-chunks.md §4 草案）+ package.json `"size"` 脚本；阈值 = 本步构建后同脚本实测 × 1.10 回填（对照升级前基线 293,407 B 确认"总 gzip 不增加"AC）。
- [ ] quality.yml：Build frontend 后新增 "Check bundle size budget" 步；Taskfile check 尾部同步（quality.yml 顶部锚点约定）。
- [ ] hash 稳定性验证：改 App.jsx 一行注释重构建，antd-vendor 文件名不变。
- [ ] 门禁 + `pnpm size`。

## Step 7：R9/R10/R11 正确性修复（提交 ⑦）

- [ ] 测试设施：vite.config.js `test.setupFiles`（matchMedia stub）。
- [ ] R9：activeRoomKey 单 state + buildings 全量渲染期 find + Modal 移出早退分支 + room 消失保持打开兜底 + capacity 渲染时转换；**Modal 内容跟随后台刷新**回归测试。
- [ ] R10：App activeCampusId 渲染派生 + 薄收敛 effect；ClassTimePicker prunedSelected 派生 + 收敛 effect 守卫照旧（design D8 方案 A）。
- [ ] R11：时钟 effect 加 visibilitychange 重同步 + fake timers 测试。
- [ ] 门禁。

## Step 8：R12/R13（提交 ⑧，可裁剪）

- [ ] R13：ErrorBoundary componentDidCatch + main.jsx 顶层包裹（fallback 纯 HTML）+ 测试。
- [ ] R12 必做档：三个 Picker 按钮补 aria-pressed。
- [ ] R12 完整档（时间允许）：ToggleButtonGroup 抽象（multiple/single、renderLabel、settings 按钮留组外、全选不进组）+ 测试 + CSS 合并（净删 ~100 行）。
- [ ] 门禁。

## 收尾（Phase 3）

- [ ] trellis-check 全量（AC 逐条 + design 决策表 + 测试账本对账）。
- [ ] 对抗验证双镜头（数据层契约镜头：api-contract.md Scenario 矩阵逐条；升级/体积镜头：版本锁线、audit、预算、hash 稳定性）。
- [ ] 冒烟：task build 全链路（含 Go embed）起服务过一遍前端流程。
- [ ] **提醒用户一次性人工视觉回归**（design §3 清单）。
- [ ] update-spec：api-contract.md 前端 Scenario 节（"cancel timers"措辞、测试文件点名、SWR 化后的承担者）、quality-guidelines（体积预算步）。
- [ ] 按步提交已在各 Step 完成；归档走 /trellis:finish-work。
