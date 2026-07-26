# 前端架构审计结论（2026-07-26）

来源：全量阅读 frontend/src（1634 行源码 + 1343 行测试）的架构审计。总评：纯逻辑模块（reloadSchedule/campusSelection/classTimeUtils/todayClassroomsResponse/classroomDataValidity）抽取干净、测试充分，问题集中在 React 层。

## 核心发现

### 手写数据层 → SWR（最高优先级）
- useTodayClassrooms.js:96-234（139 行，6 useState + ref + 3 effect）手写实现了 SWR 内置能力全集：
  - pageVisible（:105,109-118）→ revalidateOnFocus
  - abort+40s 超时（:120-196）→ `AbortSignal.timeout(40_000)`
  - mergeFetchResult（:47-79）→ keepPreviousData
  - failureCount（:38-40,103）→ onErrorRetry retryCount
  - 重调度 effect（:205-225）→ `refreshInterval: (data) => nextReloadDelay(data)`（函数签名完全吻合）
  - retry（:198-203）→ mutate()
- **保留 reloadSchedule.js 与 classroomDataValidity.js 不动**（真正的领域逻辑：stale_until 钳制、抖动、Nginx 30r/m 限流感知）。
- SWR ~4KB gzip；退路：6 useState 合并为 1 个 useReducer。
- 正确性隐患（SWR 化后自然消除）：
  - respRef 渲染期写入 + setResp updater 内副作用（:106-107,159-163,177-181）——违反纯度约定，React 19/concurrent 下会暴露。
  - normalizeResponse 用 throw 做同函数控制流、HTTP 状态码丢失（todayClassroomsResponse.js:43-66，errorEnvelope 默认 500）、:157-163 与 :176-181 逐行重复。

### antd 体积与 UI 复杂度不匹配
- 实际用到 13 个组件：Alert/Button/Card/ConfigProvider/Divider/Empty/Modal/Spin/Switch/Table/Tag/Typography/theme。
- **Table 零特性使用**（TodayClassroomTable.jsx:151-163：无排序/筛选/分页），拉入 rc-table+rc-virtual-list 全链路；同文件 :180-205 已手写了原生 `<table>`（room-info__table，样式在 TodayClassroomTable.css:93-150）→ 主表改原生，删 .ant-table-* 覆盖（含 !important），省 20-30KB gzip。
- 19 处 `!important` 对抗 antd；暗色是 antd algorithm + 20 处 body.dark 手写 CSS 双轨并行。
- @ant-design/icons 只用 2 个图标（SettingOutlined、GithubOutlined）→ 内联 SVG，删依赖（省 20-40KB min）。
- 三档方案：A 档（推荐低风险）删 Table+icons+拆 chunk；B 档再替 Card/Typography/Divider/Tag/Empty（~60 行 CSS）；C 档全移除 antd（原生 dialog + role=switch + ~250 行 CSS，估算 150KB→45KB gzip）。以 rollup-plugin-visualizer 实测数据决策。

### 构建
- vite.config.js:8-20 manualChunks 只拆 react-vendor，antd 全落主 chunk——任何业务改动用户重下整个 antd → 加 antd-vendor 分组（antd/rc-*/@ant-design）+ visualizer + CI 体积预算。
- 缺 build.target、test.coverage、setupFiles、eslint-plugin-jsx-a11y（多选按钮缺 aria-pressed）。

### 组件层重复与缺陷
- 三个 Picker 是同一套多选按钮组的三份拷贝（BuildingPicker.jsx:31-44、ClassTimePicker.jsx:112-149、CampusButtonGroup.jsx:27-44；前两者 toggle 代码逐字符相同；CSS 193 行高度重复）→ 抽 `<ToggleButtonGroup mode="multiple|single">` + 一份 CSS（~70 行），顺带补 aria-pressed。
- TodayClassroomTable.jsx:7-10,88-95 用 4 个 useState 存 Modal 且复制数据快照——后台刷新后 Modal 显示过期信息 → 改 activeRoomKey 单状态 + find 实时派生。
- 两处 effect 派生状态：App.jsx:74-83（数据到达先渲染一帧空 → 闪烁）、ClassTimePicker.jsx:47-72（依赖含 selectedClassTimes 又 dispatch，靠 pruneEndedClassTimes 返回原引用的隐式契约避免死循环）→ 改渲染期派生。
- ClassTimePicker.jsx:20-32 setTimeout 链在后台标签页被节流，恢复后不重同步 → visibilitychange 时 setNow（useTodayClassrooms:109-118 已有正确处理但两处各写各的，抽共享 hook）。
- ErrorBoundary 只包 Table 且无 componentDidCatch（静默吞异常）→ 顶层再包一层 + console.error。
- 小项：useMemo 包 O(1) 三元式（TodayClassroomTable.jsx:11-27）；SelectionProvider.jsx:18-41 的 previousPrefs ref 无必要（24→6 行）；GithubLink/EmptyCard/formatNode 重复；Typography 解构在组件体内。

### TypeScript（强烈建议）
- 源码 24 处 Array.isArray + 20 处 typeof 防御检查；prop-types 在 React 19 中是死代码（7 处 PropTypes.object/array 零信息量）。
- 后端 service/model/realtime_data.go 有完整 JSON tag 结构体 → tygo 等工具生成 TS 类型，契约自动对齐。
- 路径：checkJs+jsconfig → 生成 api-types.d.ts → normalizeResponse 单点校验（可用 zod）→ 删防御检查 → 逐文件改名 → 删 prop-types。

### 依赖版本（lockfile 从未刷新）
- antd 实锁 5.12.6（2023-12）、react 18.2.0（2022-06）、@ant-design/icons 5.2.6；devDeps 全最新（eslint 9.39、vitest 3.2.7、vite 6.4.3 精确锁定无注释）。
- dayjs 声明为直接依赖但源码 0 次使用 → 删。
- 升级顺序：antd 5.x 最新（低破坏）→ Vite 7 → React 19（需 @types/react 19 + 删 propTypes）。
- pnpm.overrides 4 条补丁式覆盖是依赖树陈旧的信号，升级后多数可删。

### 接口契约
- 只有 1 处 fetch（/api/get_data），无 API 层抽象 → 抽 src/api/client.js（前瞻）。
- 三种错误形态并存（硬信封/软 stale/抛出 Error）→ 统一 FetchOutcome 判别联合。
- 后端 log_id 被前端丢弃（normalizeResponse 只取 code/msg/data），GlobalEmpty 让用户反馈却无定位凭据 → 透传展示。
- 楼栋别名硬编码前端（BuildingPicker.jsx:6-15 BUILDING_ALIASES）→ BuildingInfo 加 display_name 下沉后端。
- CampusSettingsModal.jsx:57 toLocaleString 用浏览器本地时区，与全应用锁定 Asia/Shanghai 不一致。
- Capacity 为 0 时显示"未知"（TodayClassroomTable.jsx:120 `capacity || "未知"`）——0 语义需与后端约定。
- 8 个组件 0 测试（逻辑层 1343 行测试很充分）。

## 优先级表（原审计 Top）
1. 移除 antd Table 改原生（高/低成本）
2. SWR 替换手写数据层（高/中）
3. ToggleButtonGroup 统一三 Picker（高/中）
4. TypeScript + Go struct 生成类型（高/高）
5. manualChunks antd-vendor + visualizer + 体积预算（中高/极低）
6. 消除两处 effect 派生状态（中高/低）
7. Modal 状态合一修陈旧 bug（中高/低）
8. antd 升最新 5.x（中高/低）
