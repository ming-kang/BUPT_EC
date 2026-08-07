# 前端审计结论（2026-08-07）

对照基线：`archive/2026-07/07-26-arch-simplify-refactor/research/audit-frontend.md`。  
证据来源：`frontend/src/**`、`frontend/package.json`、`vite.config.js`、`scripts/check-bundle-size.mjs`。  
总评：批次④之前的前端现代化（SWR、原生表、antd-vendor、Modal 派生、ErrorBoundary、aria-pressed、依赖升级）**大部分已落地**；剩余大块是 **TypeScript / React 19 / prop-types 债**、**三 Picker 未抽取**、**antd B/C 档体积**，以及若干契约边角（capacity=0、时区、楼栋别名）。

## 07-26 对照总表（主要条目）

| 07-26 条目 | 现状 | 本轮状态 |
|---|---|---|
| 手写数据层 → SWR | `useTodayClassrooms.js` 基于 `useSWR`；领域逻辑仍在 `reloadSchedule`/`classroomDataValidity` | **已落地** |
| 原生 Table 替 antd Table | `TodayClassroomTable.jsx` 原生 `<table>` | **已落地** |
| 内联 icons 删 @ant-design/icons | `components/icons.jsx`；package.json 无该依赖 | **已落地** |
| antd-vendor + visualizer + 体积预算 | `vite.config.js:10-42`；`BUDGET_BYTES=230888` | **已落地** |
| Modal 单状态 + 实时派生 | `openedRoom` + `activeRoom` 派生（`TodayClassroomTable.jsx:14,68-82`） | **已落地** |
| App 校区 effect 闪烁 | 渲染期 `chooseCampusId`（`App.jsx:56-60`） | **已落地** |
| ClassTimePicker visibility 重同步 | `ClassTimePicker.jsx:30-46` | **已落地** |
| ErrorBoundary 顶层 + componentDidCatch | `main.jsx` + `ErrorBoundary.jsx:14` | **已落地** |
| antd / React / Vite 升级 | antd ^5.29.3、react ^18.3.1、vite ^7.3.6 | **已落地**（React 停在 18） |
| dayjs / pnpm.overrides 陈旧信号 | 无 dayjs；overrides 已清 | **已落地** |
| log_id 透传展示 | fetcher + `GlobalEmpty` | **已落地** |
| setupFiles / build.target | `vite.config.js:31-33,47-53` | **已落地** |
| ToggleButtonGroup 统一三 Picker | Building/ClassTime/Campus 仍各写一套 | **仍有效** |
| TypeScript + 生成类型 | 无 tsconfig/jsconfig；仍 prop-types | **仍有效** |
| React 19 | 仍 18.3 | **仍有效** |
| antd B/C 档（去 Card/Typography…） | 仍用 Card/Typography/Modal/Tag/Empty/… | **仍有效** |
| eslint-plugin-jsx-a11y | 未引入（已有手工 aria-pressed 测试） | **仍有效（降级）** |
| Building display_name 下沉 | `BUILDING_ALIASES` 仍在前端 | **仍有效** |
| capacity `\|\| "未知"` | 仍在；测试锁定 0→未知 | **仍有效** |
| CampusSettingsModal 本地时区 | `toLocaleString()` 无 timeZone | **仍有效** |
| 组件测试覆盖 | 主要组件已有测试；逻辑层仍厚 | **已过时**作为「8 组件零测试」 |

## 批次④遗留（前端相关）判定

| 项 | 判定 | 依据 |
|---|---|---|
| TypeScript | **继续做**（分阶段） | 防御检查与 prop-types 仍在；与后端 model JSON tag 对齐收益高；可先 checkJs/`api-types` 再逐文件。 |
| React 19 | **降级**（或与 TS 同任务末期） | 18.3 已够用；19 主要逼删 prop-types。单独升 19 而不上 TS 收益偏低。 |

---

## 发现清单

### F-01 · TypeScript 迁移（含生成 API 类型）

- **状态相对 07-26**：仍有效
- **类别**：可维护性 / DX
- **证据**：`frontend/package.json:17-22`（prop-types + JS）；无 `tsconfig`/`jsconfig`；`service/model/realtime_data.go` 有完整 JSON tag；源码仍大量 `Array.isArray`/`typeof`
- **建议**：路径：`allowJs`+逐步 `checkJs` → tygo/手写 `api-types.d.ts` → `normalizeResponse` 单点校验 → 逐文件 `.tsx` → 删 prop-types。
- **收益**：契约漂移早发现；删零信息 PropTypes；为 React 19 铺路。
- **风险**：中 — 迁移期双轨；勿一次大爆炸。
- **工作量**：L
- **契约影响**：无（类型层）
- **建议后续任务**：`frontend-typescript`（可拆：类型生成 / 核心模块 / 组件层）

### F-02 · React 19 升级

- **状态相对 07-26**：仍有效
- **类别**：DX
- **证据**：`package.json` react/`@types/react` 18.x；prop-types 在 19 无运行时校验价值
- **建议**：在 F-01 中后期或完成后升级；跟官方 migration；删 prop-types。
- **收益**：跟上生态；去掉死依赖。
- **风险**：中 — concurrent/严格性可能暴露残留不纯路径（当前 SWR 化后风险已降）。
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：挂在 `frontend-typescript` 末期，或独立 `react-19` 但依赖类型迁移进度

### F-03 · 三 Picker 重复 → `ToggleButtonGroup`

- **状态相对 07-26**：仍有效
- **类别**：可维护性
- **证据**：`BuildingPicker.jsx:31-44`、`ClassTimePicker.jsx:112-149`、`CampusButtonGroup.jsx:27-44`；CSS 多份高度相似；测试已分别锁 `aria-pressed`
- **建议**：抽 `<ToggleButtonGroup mode="multiple|single">` + 一份 CSS；保留各 Picker 的领域 props。
- **收益**：减重复、统一 a11y；后续改样式一处生效。
- **风险**：低–中 — 需回归三组测试与视觉。
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：`toggle-button-group`（独立轻中型）

### F-04 · antd 剩余体积（B/C 档）

- **状态相对 07-26**：仍有效
- **类别**：性能
- **证据**：仍 import Alert/Button/Card/ConfigProvider/Divider/Empty/Modal/Spin/Switch/Tag/Typography/theme；总预算 230888 B（`check-bundle-size.mjs:25`）；注释承认首屏因 antd-vendor 从 ~183KB→~207KB
- **建议**：默认**不**上 C 档全删。可选 B：替 Card/Typography/Divider/Tag/Empty 为轻量 CSS（先 `BUNDLE_REPORT=1` 量化）。首屏可考虑 lazy 非首屏 antd（Modal/设置）再压 first-load。
- **收益**：中 — 总包已 −28%；再抠边际收益递减。
- **风险**：中 — 暗色双轨 CSS 可能回潮。
- **工作量**：M–L
- **契约影响**：无
- **建议后续任务**：**降级**；仅当预算吃紧或首屏 KPI 需要时开 `antd-b-tier`

### F-05 · `capacity || "未知"` 与 0 语义

- **状态相对 07-26**：仍有效
- **类别**：可维护性
- **证据**：`TodayClassroomTable.jsx:110,206`；测试明确锁定 0→「未知」（`TodayClassroomTable.test.jsx:112-121`）
- **建议**：与后端约定：缺失用 omit/`null`，0 为真实零座；前端改 `== null` 判断。若 JW 确实用 0 表示未知，在 spec/注释写死并关闭本项。
- **收益**：避免真实 0 座教室误显示。
- **风险**：低 — 需确认 JW 数据语义。
- **工作量**：S
- **契约影响**：可能需 CHANGELOG（展示语义）
- **建议后续任务**：`capacity-zero-semantics`（轻量，先调研）

### F-06 · CampusSettingsModal 更新时间用浏览器本地时区

- **状态相对 07-26**：仍有效
- **类别**：可维护性
- **证据**：`CampusSettingsModal.jsx:57` `toLocaleString()`；业务日锁定 `Asia/Shanghai`（`classTimeUtils.js:1-13`）
- **建议**：`toLocaleString("zh-CN", { timeZone: "Asia/Shanghai", ... })` 或复用既有格式化工具。
- **收益**：与全应用时区一致，避免跨时区用户误解。
- **风险**：极低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：可并入任意前端小卫生任务

### F-07 · 楼栋别名硬编码前端

- **状态相对 07-26**：仍有效
- **类别**：可维护性
- **证据**：`BuildingPicker.jsx:6-15` `BUILDING_ALIASES`；`BuildingInfo` 仅 `name`/`rooms`（`model/realtime_data.go:50-53`）
- **建议**：`BuildingInfo` 增加可选 `display_name`（或 alias）由后端规范化；前端只渲染。
- **收益**：别名单源；改名不发前端。
- **风险**：低–中 — API 加法
- **工作量**：S–M
- **契约影响**：需 CHANGELOG（加法）
- **建议后续任务**：`building-display-name`（跨前后端轻量）

### F-08 · ClassTimePicker 仍用 effect 收敛 store（可接受）

- **状态相对 07-26**：已过时作为缺陷（渲染期 prune 已做）
- **类别**：可维护性
- **证据**：`ClassTimePicker.jsx:62-88` — 渲染期 `prunedSelected` + effect 写回 store
- **建议**：保持；除非 Selection 模型改为「派生不入库」再删 effect。
- **收益**：—
- **风险**：—
- **工作量**：—
- **契约影响**：无
- **建议后续任务**：关闭

### F-09 · SWR 层与 `mergeFetchResult` 双轨复杂度

- **状态相对 07-26**：新发现（SWR 落地后的残留复杂度）
- **类别**：可维护性
- **证据**：`useTodayClassrooms.js:55-87,225-294` — SWR data/error 双轨 + 手写 `mergeFetchResult`/`pollingInterval`/`retryOnError`
- **建议**：不回退手写 fetcher；可考虑把 merge/spin 策略抽到纯函数模块并维持现有测试；避免再引入第二套缓存库。
- **收益**：钩子更薄，利于 TS 迁移。
- **风险**：低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：可附带在 `frontend-typescript` 前期整理

### F-10 · jsx-a11y / 首屏预算细化

- **状态相对 07-26**：仍有效（降级）
- **类别**：DX / 性能
- **证据**：无 `eslint-plugin-jsx-a11y`；`aria-pressed` 已有组件测试；`check-bundle-size.mjs:17-24` 记录总预算 vs 首屏权衡
- **建议**：a11y 插件可选；首屏若要优化，加「静态可达」体积门或 lazy Modal，而非只盯总预算。
- **收益**：边际
- **风险**：低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：降级；与 F-04 联动时再开

---

## 建议执行顺序（前端）

1. **F-06**（时区一行级修复）— 可随时夹带  
2. **F-03** ToggleButtonGroup — 高性价比可维护性  
3. **F-05 / F-07** — 小契约澄清（需产品/数据确认）  
4. **F-01** TypeScript — 批次④主前端项  
5. **F-02** React 19 — 跟在 TS 后  
6. **F-04 / F-10** — 体积 KPI 驱动时再做  
