# Research: antd A 档与 UI 正确性修复（R6/R7/R9/R10/R11/R12/R13）

- **Query**: antd 使用普查、Table 原生化、图标内联、Modal 过期数据、effect 派生状态、时钟重同步、ToggleButtonGroup/ErrorBoundary
- **Scope**: mixed（内部代码为主 + npm registry 版本核实）
- **Date**: 2026-07-27
- **代码基线**: main @ b2734d9；frontend 实锁 antd 5.12.6 / @ant-design/icons 5.2.6 / react 18.2.0

---

## 1. 全项目 antd 使用普查（组件 × 文件矩阵）

import 出现点（全部 11 条 import，均在 `frontend/src`）：

| 文件 | antd import（行号） | @ant-design/icons import |
|---|---|---|
| `App.jsx:2` | Alert, ConfigProvider, Spin, Typography, theme | — |
| `components/BuildingPicker.jsx:2` | Button, Card | — |
| `components/CampusButtonGroup.jsx:2,4` | Button | SettingOutlined |
| `components/CampusSettingsModal.jsx:2,3` | Button, Divider, Modal, Switch, Typography | GithubOutlined |
| `components/ClassTimePicker.jsx:2` | Button, Card | — |
| `components/TodayClassroomTable.jsx:2` | Card, Empty, Modal, **Table**, Tag | — |
| `components/Footer.jsx:1,2` | Typography, Button | GithubOutlined |
| `components/GlobalEmpty.jsx:2` | Button, Card, Empty | — |

组件维度汇总（A 档前 13 个 → A 档后 12 个）：

| antd 导出 | 使用文件 | A 档处置 |
|---|---|---|
| Table | TodayClassroomTable | **删除**（R6） |
| Alert | App | 保留 |
| Button | BuildingPicker/CampusButtonGroup/CampusSettingsModal/ClassTimePicker/Footer/GlobalEmpty | 保留（6 文件，最广） |
| Card | BuildingPicker/ClassTimePicker/TodayClassroomTable/GlobalEmpty | 保留 |
| ConfigProvider + theme | App | 保留（暗色 algorithm 依赖） |
| Divider / Switch | CampusSettingsModal | 保留 |
| Empty | TodayClassroomTable/GlobalEmpty（含 `Empty.PRESENTED_IMAGE_SIMPLE`） | 保留 |
| Modal | TodayClassroomTable/CampusSettingsModal | 保留 |
| Spin | App | 保留 |
| Tag | TodayClassroomTable（free_nodes 列） | 保留（主表原生化后 Tag 继续用在 `<td>` 内） |
| Typography | App/CampusSettingsModal/Footer | 保留 |

结论：删 Table + icons 后 antd 仍被全部 8 个 JSX 组件文件使用，antd 本体不可删（A 档定义相符）。

版本核实（npm registry，2026-07-27）：
- antd `latest` = **6.5.2**（v6 已发布！）；`latest-5` dist-tag = **5.29.3**。R1 升级必须用 `antd@^5` 或 `antd@latest-5`，裸 `pnpm update antd --latest` 会跳大版本。
- antd 5.29.3 自身 dependencies 含 `@ant-design/icons: ^5.6.1` 与 `rc-table: ~7.54.0` —— 即使删掉我们的直接依赖并移除 Table 用法，这两个包仍留在 lockfile（作为 antd 传递依赖）；靠 tree-shaking 不进 bundle，配合 R8 visualizer 验证。

---

## 2. R6：TodayClassroomTable 主表 antd Table → 原生 `<table>`

### 2.1 现有 antd Table 用法全解（`TodayClassroomTable.jsx:151-163`）

```jsx
<Table
  dataSource={emptyClassrooms}      // :152 来自 memo（:29-59），已排序好的行数组
  columns={columns}                 // :153 见下
  pagination={false}                // :154 无分页
  bordered={false}                  // :155 无边框
  tableLayout="auto"                // :156
  size="small"                      // :157 紧凑 padding
  rowKey={(record) => `${record.building}-${record.display_name}`}  // :158
  style={{ width: "100%" }}         // :159-161
  scroll={{ x: true }}              // :162 横向滚动
/>
```

columns（`:97-139`）3 列，全部 `align: "center"`：
1. **教室**（display_name）：render 输出 `<button class="room-name">`（:103-113），点击调 `showClassroomInfo(record)` —— 原生化后原样保留此 button。
2. **座位数**（capacity）：render `capacity || "未知"`（:120）—— capacity=0 显示"未知"的语义问题审计已提，本次保持不变。
3. **空闲节次**（free_nodes）：render 过滤 `selectedClassTimes.includes(node)` 后逐个 `<Tag bordered={false}>`，`padStart(2,"0")`（:127-137）。

**未使用的 Table 特性**：排序、筛选、locale（空态早退保证 Table 永远不会渲染空数据——`:65-86` 在 `emptyClassrooms.length===0` 时直接 return Empty Card，locale.emptyText 不需要）、sticky、rowSelection、expandable、onRow。确认零特性依赖，可无损原生化。

外层 Card：`:143-150` 用 `styles={{ body: { padding: 0 } }}`（antd 5 新 API，5.29 兼容）。原生化后 Card 保留。

### 2.2 `.ant-table-*` 覆盖与 `!important` 清单

全部在 `frontend/src/components/TodayClassroomTable.css`（176 行）：

| 行号 | 选择器 | 作用 | 处置 |
|---|---|---|---|
| :5-7 | `.today-classroom-table .ant-table-wrapper` | `overflow-x: auto` | 删，由原生 wrap div 承担 |
| :9-12 | `.ant-table-thead > tr > th` | 表头 `font-weight:500; bg rgba(0,0,0,0.02)` | 删，等价规则已存在于 `.room-info__table th`（:111-116） |
| :14-16 | `body.dark ... .ant-table-thead > tr > th` | 暗色表头 bg | 删，等价 :118-121 |
| :18-20 | `.ant-table-tbody > tr` | bg transition | 删（可选：原生 hover 补回，见 2.4） |
| :153-155 | `@media ≤767px .ant-table` | `font-size: 0.9em` | 删，改写为原生表类名 |
| :157-159 | `@media ≤767px .ant-table-cell` | `padding: 10px 6px` | 同上 |
| :163-165 | `@media ≤479px .ant-table` | `font-size: 0.85em` | 同上 |
| :167-169 | `@media ≤479px .ant-table-cell` | `padding: 8px 4px` | 同上 |
| :171-175 | `@media ≤479px .ant-tag` | Tag 缩小 | **保留**（Tag 不在 A 档删除范围） |

`!important` 现状（本文件仅 2 处，**都不在 .ant-table 规则上**，而在 room-info 体系内）：
- `:129` `.room-info__col-node { text-align: center !important; }` —— 完全冗余：基础规则 `.room-info__table th, .room-info__table td`（:99-104）已设 `text-align: center`，可直接删掉整条声明。
- `:145` `.room-info__empty { padding: 24px 14px !important; }` —— 存在原因是特异性不足：`.room-info__empty`(0,1,0) 打不过 `.room-info__table td`(0,1,1)。改写为 `.room-info__table td.room-info__empty` 即可去掉 `!important`。

其余全项目 `!important`（14 处总数）分布：ClassTimePicker.css 8 处、App.css 3 处、TodayClassroomTable.css 2 处（上述）——PRD R6 只要求删 `.ant-table-*` 相关，实际收益是上述 2 处顺带清理；ClassTimePicker 的 8 处属 R12 范围。

### 2.3 `.room-info__table` 现有样式体系（可复用性）

位于 TodayClassroomTable.css:83-150，服务于 Modal 内已有的手写 `<table>`（JSX :180-205）。构成：

- `.room-info__table-wrap`（:83-91）：`overflow-x:auto` + 1px 边框 + `--bupt-ec-radius-sm` 圆角，暗色边框变体。→ 直接可作为主表横向滚动容器模式。
- `.room-info__table`（:93-97）：`width:100%; border-collapse:collapse; font-variant-numeric:tabular-nums`。
- th/td（:99-109）：`padding:10px 14px; text-align:center;` 行底边框 + 暗色变体。
- th（:111-121）：`font-size:12px; font-weight:500;` 灰字 + 浅灰底 —— 与被删的 `.ant-table-thead` 覆盖（:9-16）视觉一致，证明复用可行。
- `tr:last-child td` 去底边框（:123-125）。
- 列修饰类 `.room-info__col-node`（width:28%）/`.room-info__col-time`/`.room-info__empty`。

**复用方案建议**：把 :83-125 的"表核心"改名为通用块（如 `.ec-table-wrap` / `.ec-table`），modal 表和主表同用；modal 专属的列宽类（`__col-node` width:28%）留在 room-info 命名下。主表新增 3 个列类或不加类（3 列均居中，无需列级样式，仅"教室"列按钮已有 `.room-name` 样式 :22-41）。若不想动 modal 命名，也可让主表直接挂 `.room-info__table` 类——语义略怪但零 CSS 新增；推荐前者（一次 rename，两处受益）。

### 2.4 原生 `<table>` 改写的语义/行为要求

- **结构**：`<div class="…-wrap（overflow-x:auto）"><table><thead><tr><th scope="col">×3</thead><tbody>…`。可选 visually-hidden `<caption>`（如"今日空教室列表"）。
- **表头 sticky**：当前 antd Table 未启用 sticky，**不需要**引入（保持行为等价）。
- **响应式**：横向滚动由 wrap div 承担（对应删除的 :5-7）；≤767px/≤479px 的 font-size 与 cell padding 两档 media query 用新类名等价重写（对应 :153-169）。
- **空态**：维持现状——组件在数据为空时早退渲染 Empty Card（:65-86），`<table>` 分支永远有行，无需表内空态。
- **rowKey 等价**：React key 用 `${record.building}-${record.display_name}`（同 :158；display_name 跨楼可能重名，必须带 building 前缀——这也是 R9 activeRoomKey 的现成格式）。
- **行 hover**：antd Table 默认行 hover 高亮会随之消失（:18-20 只是 transition）。如需保留观感补一条 `tbody tr:hover td { background: rgba(0,0,0,0.02) }`（+暗色变体）；属可选打磨。
- **size="small" 等价**：th/td padding 直接写死（复用体系已是 10px 14px，接近 small 档），无需换算 antd token。

---

## 3. R7：内联 SVG 图标，删除 @ant-design/icons

### 3.1 使用点（仅 2 个图标、3 个文件、4 个渲染位）

| 图标 | 文件:行 | 上下文 |
|---|---|---|
| SettingOutlined | `CampusButtonGroup.jsx:4`(import), `:22` 与 `:32`（`icon={<SettingOutlined />}`，Button，空列表/列表中缝两个分支） | 有 `aria-label="设置"`（:24,:34） |
| GithubOutlined | `CampusSettingsModal.jsx:3`(import), `:77`（Button `icon` prop, size=small type=link） | 按钮有文字 "Github" |
| GithubOutlined | `Footer.jsx:2`(import), `:19`（Button `icon` prop, type=text size=small） | 有 `aria-label="GitHub"`（:20） |

### 3.2 内联 SVG 方案（path 数据已从本地 `@ant-design/icons-svg@4.3.1` 提取）

两个图标同属 outlined 主题，SVG attrs 一致：`viewBox="64 64 896 896" focusable="false"`。antd 图标 wrapper 的等价渲染是 `<span role="img" class="anticon"><svg fill="currentColor" width="1em" height="1em" …/></span>`；自绘时保留 `fill="currentColor" width="1em" height="1em" aria-hidden="true"` 即可继承按钮色与字号，无需 anticon class（项目 CSS 无任何 `anticon` 选择器，已 grep 确认）。

**SettingOutlined path**（单 path，d 值完整）：

```
M924.8 625.7l-65.5-56c3.1-19 4.7-38.4 4.7-57.8s-1.6-38.8-4.7-57.8l65.5-56a32.03 32.03 0 009.3-35.2l-.9-2.6a443.74 443.74 0 00-79.7-137.9l-1.8-2.1a32.12 32.12 0 00-35.1-9.5l-81.3 28.9c-30-24.6-63.5-44-99.7-57.6l-15.7-85a32.05 32.05 0 00-25.8-25.7l-2.7-.5c-52.1-9.4-106.9-9.4-159 0l-2.7.5a32.05 32.05 0 00-25.8 25.7l-15.8 85.4a351.86 351.86 0 00-99 57.4l-81.9-29.1a32 32 0 00-35.1 9.5l-1.8 2.1a446.02 446.02 0 00-79.7 137.9l-.9 2.6c-4.5 12.5-.8 26.5 9.3 35.2l66.3 56.6c-3.1 18.8-4.6 38-4.6 57.1 0 19.2 1.5 38.4 4.6 57.1L99 625.5a32.03 32.03 0 00-9.3 35.2l.9 2.6c18.1 50.4 44.9 96.9 79.7 137.9l1.8 2.1a32.12 32.12 0 0035.1 9.5l81.9-29.1c29.8 24.5 63.1 43.9 99 57.4l15.8 85.4a32.05 32.05 0 0025.8 25.7l2.7.5a449.4 449.4 0 00159 0l2.7-.5a32.05 32.05 0 0025.8-25.7l15.7-85a350 350 0 0099.7-57.6l81.3 28.9a32 32 0 0035.1-9.5l1.8-2.1c34.8-41.1 61.6-87.5 79.7-137.9l.9-2.6c4.5-12.3.8-26.3-9.3-35zM788.3 465.9c2.5 15.1 3.8 30.6 3.8 46.1s-1.3 31-3.8 46.1l-6.6 40.1 74.7 63.9a370.03 370.03 0 01-42.6 73.6L721 702.8l-31.4 25.8c-23.9 19.6-50.5 35-79.3 45.8l-38.1 14.3-17.9 97a377.5 377.5 0 01-85 0l-17.9-97.2-37.8-14.5c-28.5-10.8-55-26.2-78.7-45.7l-31.4-25.9-93.4 33.2c-17-22.9-31.2-47.6-42.6-73.6l75.5-64.5-6.5-40c-2.4-14.9-3.7-30.3-3.7-45.5 0-15.3 1.2-30.6 3.7-45.5l6.5-40-75.5-64.5c11.3-26.1 25.6-50.7 42.6-73.6l93.4 33.2 31.4-25.9c23.7-19.5 50.2-34.9 78.7-45.7l37.9-14.3 17.9-97.2c28.1-3.2 56.8-3.2 85 0l17.9 97 38.1 14.3c28.7 10.8 55.4 26.2 79.3 45.8l31.4 25.8 92.8-32.9c17 22.9 31.2 47.6 42.6 73.6L781.8 426l6.5 39.9zM512 326c-97.2 0-176 78.8-176 176s78.8 176 176 176 176-78.8 176-176-78.8-176-176-176zm79.2 255.2A111.6 111.6 0 01512 614c-29.9 0-58-11.7-79.2-32.8A111.6 111.6 0 01400 502c0-29.9 11.7-58 32.8-79.2C454 401.6 482.1 390 512 390c29.9 0 58 11.6 79.2 32.8A111.6 111.6 0 01624 502c0 29.9-11.7 58-32.8 79.2z
```

**GithubOutlined path**（单 path）：

```
M511.6 76.3C264.3 76.2 64 276.4 64 523.5 64 718.9 189.3 885 363.8 946c23.5 5.9 19.9-10.8 19.9-22.2v-77.5c-135.7 15.9-141.2-73.9-150.3-88.9C215 726 171.5 718 184.5 703c30.9-15.9 62.4 4 98.9 57.9 26.4 39.1 77.9 32.5 104 26 5.7-23.5 17.9-44.5 34.7-60.8-140.6-25.2-199.2-111-199.2-213 0-49.5 16.3-95 48.3-131.7-20.4-60.5 1.9-112.3 4.9-120 58.1-5.2 118.5 41.6 123.2 45.3 33-8.9 70.7-13.6 112.9-13.6 42.4 0 80.2 4.9 113.5 13.9 11.3-8.6 67.3-48.8 121.3-43.9 2.9 7.7 24.7 58.3 5.5 118 32.4 36.8 48.9 82.7 48.9 132.3 0 102.2-59 188.1-200 212.9a127.5 127.5 0 0138.1 91v112.5c.8 9 0 17.9 15 17.9 177.1-59.7 304.6-227 304.6-424.1 0-247.2-200.4-447.3-447.5-447.3z
```

建议新建 `frontend/src/components/icons.jsx` 导出 `SettingIcon`/`GithubIcon` 两个无 props（或仅透传 className）函数组件；antd Button 的 `icon` prop 接受任意 ReactNode（5.x 会包进 `.ant-btn-icon` span，间距自动正确），三处调用点只改 import 与标签名。

### 3.3 package.json 变化

- `frontend/package.json:17` 删除 `"@ant-design/icons": "^5.2.6"`（dependencies 剩 antd/prop-types/react/react-dom）。
- **lockfile 不会移除该包**：antd 5.29.3 传递依赖 `@ant-design/icons ^5.6.1`（本地 5.12.6 亦依赖 ^5.2.6，已验证 antd/package.json）。收益体现在 bundle（两个 icon 模块 + 直接依赖声明消失），而非 node_modules。审计"省 20-40KB min"是按 tree-shaking 失效的悲观值，实际需 R8 visualizer 实测确认。

---

## 4. R9：Modal 过期数据 bug

### 4.1 现场（4 个 state + 快照复制）

- 4 个 state：`TodayClassroomTable.jsx:7-10` —— `modalTitle` / `modalCapacity` / `modalFreeTimes` / `openModal`。
- 快照写入：`showClassroomInfo(room)`（:88-95）在点击瞬间把 `room.display_name`、`room.capacity || "未知"`、`room.free_times` 复制进 state；此后 props 更新不再触达 Modal 内容。
- Modal 消费快照：`:165-208`（title=:166，capacity=:176，free_times 表=:188-203）。

### 4.2 复现路径

1. 页面加载，点击任一教室名（`.room-name` 按钮，:104-113）→ Modal 打开，显示当时的 free_times。
2. 保持 Modal 打开，等待后台轮询（`useTodayClassrooms.js:205-225` 按 `nextReloadDelay` 定时 → background reload → `resp` 更新 → App.jsx:52-55 `selectedCampusData` 新引用 → 本组件 props 变化，`emptyClassrooms` memo 重算，**表格行已更新**）。
3. Modal 仍显示打开瞬间的快照：已结束/已被占用的节次仍列为"空闲"；跨节次边界（如 12:00）时最易观察。
4. 边缘恶化形态：若刷新后 `emptyClassrooms` 变空，组件走 :65-86 早退分支——**Modal 连同 JSX 一起消失**（无关闭动画、state 残留 openModal=true，下次有数据时 Modal 会凭空弹回）。

### 4.3 activeRoomKey 派生方案数据流

```jsx
const [activeRoomKey, setActiveRoomKey] = useState(null);   // 唯一 state
const roomKey = (room) => `${room.building}-${room.display_name}`;  // 与现 rowKey(:158) 同格式
// 派生源建议用"全部房间"而非 emptyClassrooms，Modal 不因筛选波动而丢内容：
const activeRoom = useMemo(() => {
  if (activeRoomKey == null) return null;
  return buildings                     // :21-27 已有的 buildings memo（来自 props.selectedCampusData）
    .flatMap((b) => (Array.isArray(b.rooms) ? b.rooms : []).map((r) => ({ ...r, building: b.name })))
    .find((r) => roomKey(r) === activeRoomKey) || null;
} [buildings, activeRoomKey]);
// 打开：onClick={() => setActiveRoomKey(roomKey(record))}
// 关闭：onCancel={() => setActiveRoomKey(null)}
// 渲染：<Modal open={activeRoomKey != null} title={activeRoom?.display_name ?? ""}>
//   capacity 显示 activeRoom ? (activeRoom.capacity || "未知") : "—"
//   free_times = Array.isArray(activeRoom?.free_times) ? activeRoom.free_times : []
//   （空数组时现有 "暂无空闲节次" 行 :188-193 自然兜底 room 消失的场景）
```

- 数据流：`resp`（useTodayClassrooms）→ App `selectedCampusData` memo（App.jsx:52-55）→ props → `buildings` memo（:21-27）→ **render 期 find** → Modal 内容。后台刷新后一切自动跟随，无副本。
- 需要决策的点：房间从数据里彻底消失时（校区数据缺失/room 被移除），`activeRoom=null`——推荐保持 Modal 打开显示"暂无空闲节次"兜底（上面草案行为），而非强制关闭；同时把 Modal JSX 移出 :65-86 的早退分支（Modal 与空态 Card 并存渲染），修掉 4.2-4 的消失 bug。
- `capacity || "未知"` 的转换从写入时移到渲染时（消除 modalCapacity 存"已转换值"的暗坑）。

---

## 5. R10：两处 effect 派生状态

### 5.1 App.jsx:74-83 campus 选择 effect 逐行解读

```jsx
74  useEffect(() => {
75    const nextCampusId = chooseCampusId({        // 纯函数，campusSelection.js:45-78
76      campuses,                                  // resp 派生 memo（:45-51），resp.code!==0 时为 []
77      partialCampusIds: resp.data?.partial_campuses,
78      selectedCampusId: selectedCampus,          // context state（selectionContext.js 初始 ""）
79    });
80    if (nextCampusId !== selectedCampus) {
81      dispatch({ type: "SET_CAMPUS", id: nextCampusId });  // reducer 顺带清空 buildings/classTimes（selectionContext.js:37-43）
82    }
83  }, [campuses, resp.data?.partial_campuses, selectedCampus, dispatch]);
```

闪烁机制：首次数据到达 → 渲染一帧（selectedCampus=""，`selectedCampusData=null` → BuildingPicker/ClassTimePicker/Table 全 return null，campus 按钮全灰）→ effect 提交后 dispatch → 第二帧才出现选中态与下方卡片。

**渲染期派生改法草案**（推荐"派生渲染 + 保留极薄 reconcile effect"）：

```jsx
const activeCampusId = chooseCampusId({ campuses, partialCampusIds: resp.data?.partial_campuses, selectedCampusId: selectedCampus });
const selectedCampusData = useMemo(
  () => campuses.find((c) => c.id === activeCampusId) || null, [campuses, activeCampusId]);
// CampusButtonGroup 增加 activeCampusId prop（或继续读 context 但以 prop 判断选中态）
useEffect(() => {   // 仅同步 store（触发 reducer 的 buildings/times 重置），UI 已无闪烁
  if (activeCampusId !== selectedCampus) dispatch({ type: "SET_CAMPUS", id: activeCampusId });
}, [activeCampusId, selectedCampus, dispatch]);
```

- 为什么不能纯"dispatch during render"：state 在 SelectionProvider（父组件 useReducer），子组件渲染期 dispatch 触发 React "Cannot update a component while rendering a different component" 警告；React 官方 setState-during-render 模式仅限同组件。故保留 effect 同步 store，但渲染改用派生值消除可见闪烁。
- 派生首帧的过渡语义：campus 派生已就位但 store 里旧 buildings/times 尚未被 reducer 重置（晚一拍）。当前初始态 buildings/times 均为空数组，冷启动无影响；仅"校区快照失效被迫切换校区"场景下会有一帧旧 buildings 名单套新校区（匹配不到任何楼 → Table 显示"没有符合条件的空教室"一帧）。如要求绝对干净，可同时派生 `effectiveBuildings = selectedBuildings ∩ activeCampus.buildings`。
- **localStorage 交互：无**。持久化仅覆盖 `showClassTime`/`canSelectAllDay`（selectionContext.js:24-32 读、SelectionProvider.jsx:23-41 写）；campus/buildings/classTimes 均为会话内存态。R10 不触碰持久化路径。PRD 验收里的"选择与持久化"指这两个开关。

### 5.2 ClassTimePicker.jsx:47-72 裁剪 effect 逐行解读

```jsx
47  useEffect(() => {
48    if (!selectedCampusData) return;                    // 无校区数据不裁剪
51    const nodes = Array.isArray(selectedCampusData.nodes) ? ... : [];
54    const next = pruneEndedClassTimes(selectedClassTimes, nodes, {  // 纯函数，classTimeUtils.js:39-58
55      nowTime,                                          // :44 由 now state 派生（Shanghai HH:mm）
56      isToday,                                          // :45 todayDate === formatShanghaiDate(now)
57      canSelectAllDay: state.canSelectAllDay,
58    });
59    if (next.length !== selectedClassTimes.length ||    // 显式相等守卫
60        next.some((node, i) => node !== selectedClassTimes[i])) {
63      dispatch({ type: "SET_CLASS_TIMES", times: next });  // 又写回自己的依赖 → 循环风险
64    }
65  }, [selectedCampusData, selectedClassTimes, nowTime, isToday, state.canSelectAllDay, dispatch]);
```

死循环契约：依赖含 `selectedClassTimes` 且 effect 内 dispatch 改它。安全性靠双保险：(a) :59-62 的内容相等守卫；(b) `pruneEndedClassTimes` 在无可裁剪时**返回原引用**（classTimeUtils.js:45-46,54-56）。两者任一失效即死循环——审计所称"隐式契约"。

**渲染期派生改法草案**——关键约束：`state.selectedClassTimes` 有两个消费者（本组件按钮态 + App.jsx:131 传给 TodayClassroomTable 过滤行）。只在 Picker 内部派生会让 Table 看到未裁剪列表，行为退化。两个可行方向：

- 方案 A（改动最小，与 5.1 同构）：Picker 内渲染全部改用 `const prunedSelected = pruneEndedClassTimes(...)` 派生值（按钮选中态 :115、toggle 基础列表 :118-120、isAllChecked :84-90 全用它），保留一个瘦身 effect 仅做 store 收敛（守卫照旧）。消除"渲染依赖 effect 时序"的闪烁，同时 Table 因 store 收敛仍最终一致；契约风险从"防死循环"降为"多一次收敛 dispatch"。
- 方案 B（彻底派生，改动大）：时钟 `now` 上移 App（抽 `useNow(interval)` 共享 hook），App 渲染期算 `prunedClassTimes` 传给 Picker 与 Table，store 永远存原始选择，dispatch 只发生在用户交互。彻底删掉裁剪 effect，但改 3 个文件的 props 结构。
- 建议 design.md 里选 A 起步（配合 R11 的时钟仍留在 Picker），B 作为 R12 抽组件时的顺带升级选项。

---

## 6. R11：ClassTimePicker 时钟机制与 visibilitychange 重同步

现状（`ClassTimePicker.jsx:18-32` + `:161-171`）：
- `now` state 初始化 `new Date()`（:18）。
- effect（:20-32，空依赖）建 setTimeout 自链：`scheduleNextTick` 每次以 `msUntilNextFiveMinuteTick(new Date())` 对齐到下一个 5 分钟整点（:24-27），触发后 `setNow(new Date())` 并续链；卸载 clearTimeout（:31）。
- `msUntilNextFiveMinuteTick`（:161-171）：取整到分钟，`remainder===0` 时加满 5 分钟，下限钳 1000ms（:170）。

缺陷：后台标签页浏览器节流 setTimeout（Chrome 最低 1 次/分钟，intensive throttling 下 1 次/小时），切回前台后 `now` 可能滞后几十分钟——已结束节次仍可选、pruning 不触发，直到下一次 tick。对照：`useTodayClassrooms.js:109-118` 已有 visibilitychange 监听（setPageVisible），但两处各写各的。

**改法草案**（单 effect 内加监听，与现有结构最小差异）：

```jsx
useEffect(() => {
  let timeoutID;
  function schedule() {
    timeoutID = window.setTimeout(() => { setNow(new Date()); schedule(); },
      msUntilNextFiveMinuteTick(new Date()));
  }
  function onVisibility() {
    if (document.visibilityState !== "hidden") {
      window.clearTimeout(timeoutID);   // 丢弃被节流的旧闹钟
      setNow(new Date());               // 立即重同步
      schedule();                       // 以新时刻重新对齐 5 分钟栅格
    }
  }
  schedule();
  document.addEventListener("visibilitychange", onVisibility);
  return () => {
    window.clearTimeout(timeoutID);
    document.removeEventListener("visibilitychange", onVisibility);
  };
}, []);
```

备注：`setNow(new Date())` 每次都是新对象必然重渲一次（即使分钟未变），频率低可接受；若 R10 选方案 B（时钟上移 App），此改法整体移入共享 `useNow` hook。R3 的 SWR `revalidateOnFocus` 只管数据层，不覆盖此 UI 时钟，两者独立。

---

## 7. R12/R13：ToggleButtonGroup 收益评估与 ErrorBoundary

### 7.1 三个 Picker 的 JSX 重复度

| 维度 | BuildingPicker.jsx | ClassTimePicker.jsx | CampusButtonGroup.jsx |
|---|---|---|---|
| 模式 | 多选 | 多选 + 全选按钮 | 单选 |
| 按钮循环 | :31-44 | :112-149 | :27-44 |
| 选中态表达 | `type={included ? "primary" : "default"}`（:34） | 同（:115） | 同（:38） |
| toggle 逻辑 | :36-38 `filter/concat` | :118-120 **逐字符相同** | 单选 dispatch（:39） |
| 特殊点 | displayBuildingName 别名（:6-15） | 双行时间标签（:125-147）、disabled（:78-82,:123）、全选按钮（:150-156） | settings 按钮插在中缝 `settingsSplitIndex`（:14,:29-36）、空列表分支（:19-25） |
| 容器 | Card | Card | 裸 div（.campus-buttons） |

CSS 重复度（实测行数）：BuildingPicker.css 48 行 + CampusButtonGroup.css 65 行 + ClassTimePicker.css 80 行 = **193 行**。共有模式：flex wrap center + gap 容器（BP:5-11 / CTP:5-11 / CBG:5-11）、`.ant-btn` 圆角 `--bupt-ec-radius-sm` + `transform: translateY(-1px)` hover + transition（BP:13-22 / CTP:13-29 / CBG:13-22）、767/479 两档 media（三文件各两段）。差异化：CTP 固定宽 48/42/38px + show-time 高度变体 + disabled/select-all 共 8 处 `!important`；CBG settings-trigger；BP min-width em 制。合并后共享部分约 60-70 行，专有修饰各 10-30 行，净删约 90-110 行 CSS。

**抽象接口草案**：

```jsx
<ToggleButtonGroup
  mode="multiple"            // "multiple" | "single"
  options={[{ value, label, disabled }]}
  value={selectedArray | singleValue}
  onChange={(next) => dispatch(...)}
  renderLabel={(option) => ReactNode}   // CTP 双行时间、BP 别名走这里
  className / buttonClassName           // 差异化样式挂点
/>
```

- 每个按钮输出 `aria-pressed={selected}`（单选模式可考虑 role="radiogroup"/role="radio"+aria-checked，MVP 先统一 aria-pressed 亦可接受）。
- CampusButtonGroup 的 settings 按钮**留在组外**：把 options 切成两片渲染两个 group，或 group 接受 children 插槽——建议前者，避免抽象吃进特例。
- CTP 的"全选"按钮不进 group（它不是 toggle 语义），保持独立按钮尾随。
- 收益评估：JSX 去重中等（toggle 逻辑 ~6 行×2），CSS 去重显著（~100 行），**最大收益是 aria-pressed 一次性补齐**。风险：renderLabel + 三套差异样式让组件 props 偏胖；PRD 已标注 R12 可裁剪，若时间紧可只做"补 aria-pressed 到三处现有按钮"（3 行改动）保住无障碍收益。

aria 现状（全项目 grep）：仅 3 处 `aria-label`（CampusButtonGroup.jsx:24,34；Footer.jsx:20），**零 aria-pressed**；eslint 无 jsx-a11y 插件（eslint.config.js 只有 react/react-hooks/react-refresh），无 lint 兜底。

### 7.2 R13：ErrorBoundary 现状与最小改法

现状：`components/ErrorBoundary.jsx:4-17` 已存在——`getDerivedStateFromError` + fallback 渲染，**无 componentDidCatch**（异常被静默吞掉，无任何 console 输出）。唯一使用点 App.jsx:118-134，只包 lazy TodayClassroomTable。裸露面：CampusButtonGroup 里的 lazy CampusSettingsModal（CampusButtonGroup.jsx:8,47-55）只有 Suspense 没有 boundary，chunk 加载失败会直接炸穿整个应用（main.jsx 无兜底）。

最小实现草案（改 2 个文件）：

```jsx
// ErrorBoundary.jsx 增加：
componentDidCatch(error, info) {
  console.error("[ErrorBoundary]", error, info?.componentStack);
}

// main.jsx 顶层包裹（fallback 用纯 HTML，不依赖 antd/ConfigProvider——崩溃可能发生在其内部）：
<React.StrictMode>
  <ErrorBoundary fallback={<div style={{ padding: 48, textAlign: "center" }}>页面出错，请刷新重试</div>}>
    <App />
  </ErrorBoundary>
</React.StrictMode>
```

保留 App.jsx:118 的内层 boundary（局部降级优于全页兜底）。React 18 语义提醒：boundary 不捕获事件回调/异步错误；本项目主要风险面（渲染期数组防御缺口、lazy chunk 失败）都在捕获范围内。

---

## 8. 各项测试影响面

现状盘点：**组件层 0 测试**。现有 9 个测试文件全部是逻辑层/hook 层：campusSelection、classTimeUtils、classroomDataValidity、darkMode、reloadSchedule、selectionContext、todayClassroomsResponse、useTodayClassrooms（×2）。vitest 默认 environment 为 node（vite.config.js test 段），jsdom 用例需文件级 `// @vitest-environment jsdom` 指令（useTodayClassrooms.lifecycle.test.jsx 已示范此模式）。

| 需求 | 现有测试改动 | 建议新增 |
|---|---|---|
| R6 原生表 | 无（无组件测试可改） | `TodayClassroomTable.test.jsx`：渲染 thead 3 列/行数/排序、free_nodes 过滤与 padStart、空态三分支文案（:75-81） |
| R7 图标 | 无 | 可并入相关组件测试断言 svg 存在；单测价值低，可不做 |
| R9 Modal | 无 | **重点回归测试**：打开 Modal → rerender 换新 props（free_times 变化）→ 断言 Modal 内容更新；room 消失 → 断言兜底文案且不崩 |
| R10 campus/裁剪派生 | `campusSelection.test.js`、`classTimeUtils.test.js`（pruneEndedClassTimes 4 用例）**保持不变**——纯函数继续复用 | App 级：数据首帧即渲染选中校区（可测 AppContent + mock hook，成本较高，允许降级为人工回归项） |
| R11 时钟 | 无 | ClassTimePicker：vi.useFakeTimers + 派发 `visibilitychange`，断言 setNow 后 disabled 状态刷新 |
| R12 ToggleButtonGroup | `selectionContext.test.js` 不受影响（reducer 不变） | `ToggleButtonGroup.test.jsx`：multiple/single toggle、disabled 不可点、**aria-pressed 值断言** |
| R13 ErrorBoundary | 无 | `ErrorBoundary.test.jsx`：子组件 throw → fallback 渲染 + `console.error` 被调（spy） |

跨项注意：组件测试渲染 antd 组件（Card/Modal/Tag）在 jsdom 下可能需要 `window.matchMedia` polyfill（antd responsiveObserver/Grid 使用；Modal/rc-dialog 需要 `document` 动画相关 stub 一般 jsdom 自带）。项目当前**无 setupFiles**（审计已指出），首个组件测试落地时建议顺手加 `test.setupFiles` 配 matchMedia stub——属于 R2/R5 执行时的基础设施铺垫。

## Caveats / Not Found

- antd **v6 已发布**（latest=6.5.2），R1 升级命令必须锁 `^5`（latest-5=5.29.3），否则 pnpm update --latest 直接跳大版本。
- 删除 `@ant-design/icons` 直接依赖后包仍在 lockfile（antd 5.29.3 传递依赖 ^5.6.1）；"无 @ant-design/icons"验收标准应解释为 package.json dependencies 层面，AC 原文如此可满足。
- TodayClassroomTable.css 的 2 处 `!important`（:129,:145）不在 `.ant-table-*` 规则上而在 room-info 体系内：:129 冗余可直接删，:145 需改选择器特异性（`td.room-info__empty`）后才能去掉——PRD R6 "相关 !important" 按此理解执行。
- `.ant-tag` 覆盖（TodayClassroomTable.css:171-175）与 Tag 组件在 A 档保留，勿随 .ant-table 清理误删。
- R10 两处若走"渲染派生 + reconcile effect"路线，effect 并未消失只是不再影响首帧视觉；若 PRD 验收严格要求"消除 effect 派生状态"字面义，需在 design.md 明确采用方案 A（保留收敛 effect）的理由或升级到方案 B（时钟上移 App）。
- Modal 在空态早退分支会连 JSX 一起卸载（4.2-4），R9 实施时需把 Modal 移出早退分支，否则 activeRoomKey 方案也会残留"凭空弹回"问题。
