# 技术设计：前端现代化（依赖升级、SWR、原生表格、chunk 拆分、正确性修复）

证据基础：`research/deps-upgrade.md`、`research/swr-datalayer.md`、`research/antd-ui-fixes.md`、`research/build-chunks.md`（全部含 file:line 与外部版本实查）。本文只记决策与结论。

## 0. 研究对 PRD 的三处事实修正

1. **当前 Vite 是 6.4.3 不是 5**（3c98705 已升 6），本次真实迁移面 = antd 5.12→5.29、react 18.2→18.3、vite 6→7；dayjs 已在子任务 1 删除（9ad81ec），R1 该项跳过。
2. **生态已超前**：antd latest=6.5.2、vite latest=8.1.5、eslint latest=10.8.0、@vitejs/plugin-react 6.x peer 仅 vite ^8——**全部按 dist-tag 锁线，禁止裸 `pnpm update --latest`**。
3. **PRD R3 的 `keepPreviousData` 是误配**：该选项仅在 key 切换时生效，本项目 key 恒定属 no-op；"失败保留快照"的真正机制是 SWR data/error 双轨存储 + 渲染期 `mergeFetchResult` 派生。

## 1. 版本矩阵（锁定，不可越线）

| 包 | 当前 | 目标 | 备注 |
|---|---|---|---|
| antd | 5.12.6 | **^5.29.3**（latest-5） | 项目零弃用 props 命中 |
| react / react-dom | 18.2.0 | **^18.3.1** | 18.x 终点；@types 18.3 线 |
| vite | 6.4.3（钉死无^） | **^7.3.6**（previous） | 统一回 caret |
| @vitejs/plugin-react | 4.7.0 | **^5.2.0** | 6.x 仅 vite8，禁 |
| vitest / @testing-library | 3.2.7 / 16.3.0+10.4.0 | **不动 / patch 随动** | vitest 原生兼容 vite7，不升 4 |
| eslint / @eslint/js | 9.39.4 | ^9.39.5（maintenance） | eslint 10 不做 |
| jsdom | 26.1.0 | 可选 ^29.1.1 | 测试红了即回 26 并记录 |
| pnpm.overrides ×4 | @babel/runtime 等 | **全删** | 均为 3c98705 的 CVE 钉版，fresh resolve 后失效；audit 双门禁实证，复红则单条保留+注明 CVE |
| 新增 | — | swr ^2.4.2、rollup-plugin-visualizer ^7.0.1 | |

明确不做：antd 6、vite 8、react 19、eslint 10、vitest 4、plugin-react 6、pnpm 10/11。

## 2. 关键决策

### D1 升级拆两个提交（回归归因隔离）

提交 A = 运行时/UI 面（antd/react/@types/testing-library/eslint + 删 overrides + eslint.config.js `react.version: "detect"`）；提交 B = 构建面（vite 7 + plugin-react 5.2.0 + 可选 jsdom）。中间态无死锁（plugin-react 4.7.0/vitest 3.2.7 同时兼容 vite 6/7）。

### D2 build.target 显式锁定（不接受 Vite 7 新默认）

Vite 7 默认 target 抬到 Safari 16/Chrome 107，会无声切断老 iOS（<16）设备——校园用户场景不可接受。提交 B 在 vite.config.js 显式 `build.target: ['es2020', 'safari14']`（≈ Vite 6 'modules' 基线，现状兼容面不变）。抬基线留批次④独立评估。

### D3 SWR 化架构（R3/R5）

hook 重构为：`useSWR("/api/get_data", fetchTodayClassrooms, config)` + **渲染期派生层**（`mergeFetchResult`/`hasUsableClassroomData`/`shouldFullPageSpin` 原样复用，`useTodayClassrooms.test.js` 相应段零改动存活）。配置与胶水（全部有源码级依据，见 research/swr-datalayer.md §2）：

- **refreshInterval**：模块级稳定函数 + 防 falsy 包装 `(data) => nextReloadDelay(data, { failureCount: 0 }) ?? MIN_FRESH_DELAY_MS`——`nextReloadDelay(undefined)` 返回 null，稳定身份下首载即链死（SWR 返回 0/null 直接终止轮询链），这是 SWR 化第一翻车点，**必须新增"首载后轮询确实启动"测试**。
- **onErrorRetry**：抽纯函数 `retryDelayFor(retryCount, latestData) = nextReloadDelay(latestData, { failureCount: retryCount })`（retryCount 起值 1 与 10/20/30/60 阶梯零换算对齐；不得裸用 failureRetryDelay，否则丢失退避路径的 stale_until 钳制）；setTimeout 回调内自查 `document.visibilityState`，隐藏则放弃（SWR 已排定的 retry timer 无可见性门控，revalidateOnFocus 会在回归时补发）。
- **revalidateOnFocus: true 必须保持**（关掉会破坏"隐藏时重试暂停"的前置条件）；`focusThrottleInterval: 15_000`（默认 5s 会超 Nginx 30r/m 多标签预算）。
- `refreshWhenHidden` 默认 false（隐藏零 fetch，与现状"取消 timer"机制不同、可观察行为等价）；`dedupingInterval` 生产默认、**测试 0**；测试必须 `SWRConfig provider: () => new Map()` 防跨 case 缓存污染。
- fetcher：`AbortSignal.timeout(40_000)`（TimeoutError → 现有安全文案）；抛 `ApiError`（携带 status/code/logId）。
- 语义变化点（接受并写入测试）：focus 失败会重置退避阶梯（现状仅成功清零，更激进一档）；unmount 不再 abort 在途请求（lifecycle 测试②改写为"晚到响应无害"）。
- **逃生判据**（触发任一即回退为手写层 useReducer 瘦身，R4 不受影响）：(a) fake timers 下 SWR 链测试 flaky 且 2h 无解；(b) 胶水代码超过被删手写核心（~139 行）的一半；(c) focus 重验请求量调不进 30r/m。

### D4 R4 独立先行（与 SWR 解耦）

`normalizeResponse` 去 throw 改判别返回值（`{ok:true, resp} | {ok:false, reason}`，throw 收敛到 fetcher 边界一次）；真实 HTTP status 结构化保留（不再一律猜 500）；log_id 取 `payload.log_id || X-Log-Id 头`，GlobalEmpty 错误态加一行淡色小字。`todayClassroomsResponse.test.js` 的 4 个 toThrow 断言改返回值断言。

### D5 原生表格（R6）与图标（R7）

- `.room-info__table` 核心（TodayClassroomTable.css:83-125）改名为共享 `.ec-table` 体系，Modal 表与主表同用；主表结构 `div.ec-table-wrap > table > thead(3×th scope="col") + tbody`，React key 保持 `${building}-${display_name}`；不引入 sticky（现状未启用）；补可选 tbody hover。空态维持组件早退，无表内空态。
- 删 `.ant-table-*` 覆盖 8 段；`!important` 清理按实情：`:129` 冗余直删、`:145` 改 `td.room-info__empty` 提特异性；**`.ant-tag` 覆盖保留**（Tag 在 A 档保留范围）。
- 图标：新建 `components/icons.jsx` 导出 SettingIcon/GithubIcon（path 已从 icons-svg@4.3.1 提取入研究文档；`fill="currentColor" width/height="1em" aria-hidden`），3 文件 4 渲染位替换，删 @ant-design/icons 直接依赖（lockfile 仍含其 antd 传递依赖，AC 按 package.json 口径）。

### D6 chunk 拆分与体积预算（R8，顺序硬约束：必须在 R6 之后）

- **若在删 Table 前套 antd-vendor 分组，rc-table ~95KB gzip 会被拽进首屏**——执行序固定为 R6/R7 → R8。
- manualChunks 锚定真实包名段：`/\/node_modules\/(antd|@ant-design|rc-[a-z-]+|@rc-component)\//`（pnpm peer 后缀免疫；禁止裸 `id.includes('antd')`）。Rollup 4 默认把传递依赖（dayjs/@babel/runtime 等）自动归并入组。
- visualizer 7.0.1：`BUNDLE_REPORT=1` 环境变量开关，产物 `frontend/bundle-stats.local.html`（复用现有 `*.local` gitignore 规则；**严禁写入 dist/**——会被 go:embed 进发布二进制并被公网服务）。
- 体积预算：零依赖脚本 `frontend/scripts/check-bundle-size.mjs`（node:zlib level 9，dist 下 js/css/html/svg 总量），quality.yml 在 Build frontend 后新增一步；阈值 = R8 落地后**同脚本实测 × 1.10** 回填（vite 控制台 gzip 口径不同不可混用）。升级前基线（gzip -9）：总 293,407 B / 首屏 ≈183.6 KB。
- web/dist 全链路（embed、Taskfile、CI artifact）无文件名依赖，Go 侧零改动（已核实）。

### D7 Modal 修复（R9）

4 state 合一为 `activeRoomKey`；activeRoom 从 **buildings 全量**（非 emptyClassrooms）渲染期 find——Modal 内容不因筛选波动丢失；room 从数据中彻底消失时**保持打开**，free_times 空数组走现有"暂无空闲节次"兜底行（不突兀关闭）；**Modal JSX 移出空态早退分支**（修"凭空消失/弹回"伴生 bug）；`capacity || "未知"` 转换移到渲染时。

### D8 effect 派生状态（R10）——方案 A，对 PRD 字面的有据偏离

纯"渲染期 dispatch"不可行（store 在 SelectionProvider 父组件，子组件渲染期 dispatch 触发 React 跨组件更新警告）。采用**派生渲染 + 保留极薄收敛 effect**：

- App：`activeCampusId = chooseCampusId(...)` 渲染期算出，`selectedCampusData` 与 CampusButtonGroup 选中态都用派生值（首帧即正确，消除空帧闪烁）；effect 仅同步 store（触发 reducer 的 buildings/times 重置）。
- ClassTimePicker：渲染全部改用 `prunedSelected = pruneEndedClassTimes(...)` 派生（按钮态/toggle 基础/isAllChecked），收敛 effect 保留守卫照旧——"防死循环隐式契约"降级为"多一次收敛 dispatch"。Table 经 store 收敛最终一致（与现状等价，不退化）。
- 时钟上移 App 的彻底方案 B 记为 R12 顺带升级选项，本次不做。

### D9 时钟重同步（R11）

单 effect 内加 visibilitychange 监听：visible 时 clearTimeout（丢弃被节流的旧闹钟）→ `setNow(new Date())` 立即重同步 → 按 5 分钟栅格重排。与 SWR revalidateOnFocus 互不相干（数据层 vs UI 时钟）。

### D10 R12/R13

- R13 必做（改 2 文件）：ErrorBoundary 补 `componentDidCatch` + console.error；main.jsx 顶层包裹（fallback 纯 HTML 不依赖 antd）；保留 App 内层局部 boundary。
- R12 拆两档：**必做** aria-pressed 补齐三个 Picker 按钮（现状全项目零 aria-pressed）；**可裁剪** ToggleButtonGroup 完整抽象（净删 ~100 行 CSS；settings 按钮留组外、全选按钮不进组）。时间不够只做前者。

### D11 组件测试基础设施（R9 前置）

现状组件层 0 测试、无 setupFiles。首个组件测试落地时加 `test.setupFiles`（matchMedia stub 等 antd 渲染必需品）；vitest 保持默认 node 环境 + 文件级 `@vitest-environment jsdom` 指令模式。

## 3. 人工视觉回归安排

自动化推进不中途打断；两处回归点合并为提交序列完成后**一次性人工回归**（清单：校区/楼栋/节次选择与持久化开关、主表视觉、stale Alert、错误态重试与 log_id、Modal 后台刷新跟随、暗色模式、移动端 767/479 两档）。提交 A（antd 升级）保留独立提交点，视觉异常可 bisect。

## 4. 测试账本

- 零改动存活：reloadSchedule.test.js（300 行）、classroomDataValidity.test.js、useTodayClassrooms.test.js 纯函数段、campusSelection/classTimeUtils/selectionContext/darkMode 测试。
- 小改：todayClassroomsResponse.test.js（toThrow→返回值）；useTodayClassrooms.test.js 删 nextFailureCount 段。
- 重写：lifecycle 8 case（6 迁移 / ②改"晚到响应无害" / ⑥ stub AbortSignal.timeout——fake timers 大概率推不动 jsdom 内部计时器，先 spike）。
- 新增：轮询启动防链死、retryDelayFor 单测、log_id 展示、Modal 内容跟随更新、时钟重同步、aria-pressed、ErrorBoundary。

## 5. 提交切分与回滚

① 升级 A（antd/react + overrides 清理）→ ② 升级 B（vite 7 + build.target）→ ③ R4 数据层预备 → ④ SWR 化（R3/R5，含逃生出口）→ ⑤ 原生表格+图标（R6/R7）→ ⑥ chunk+visualizer+预算（R8，含 quality.yml/Taskfile 接线）→ ⑦ 正确性修复（R9/R10/R11 + 测试设施）→ ⑧ R12/R13（可裁剪）。每步独立提交可 revert；④ 若触发逃生判据则该步改为手写层瘦身并同步修订 PRD 注记。
