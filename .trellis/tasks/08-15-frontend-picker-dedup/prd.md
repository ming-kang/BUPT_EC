# PRD: ToggleButtonGroup 统一 Picker（frontend-picker-dedup）

来源：`08-07-project-audit-optimize` backlog 优先级 3（F-03 + 夹带 F-06）。

## 问题陈述

1. **F-03 三处重复的按钮组实现**：
   - `BuildingPicker.jsx:31-44`（多选，`aria-pressed` + `type` 切换 + dispatch 模式）
   - `ClassTimePicker.jsx:112-149`（多选 + disabled + 自定义内容 + 全选按钮）
   - `CampusButtonGroup.jsx:27-44`（单选 + 插入设置按钮）
   三者共享同一套「antd Button + primary/default + aria-pressed + 点击切换 dispatch」
   模式与高度相似的 CSS（flex-wrap 居中、gap、圆角、hover 上浮、响应式字号/间距），
   改样式要改三处，a11y 约定靠复制粘贴维持。
2. **F-06 设置页更新时间用浏览器时区**：`CampusSettingsModal.jsx:57` 的
   `toLocaleString()` 随访客时区漂移，与全应用业务时区 `Asia/Shanghai`
   （`classTimeUtils.js` SHANGHAI_TZ）不一致，跨时区用户会误读。

## 目标

- 抽取共享 `ToggleButtonGroup`（多选）与 `ToggleButton`（单按钮原语）组件：
  统一 `aria-pressed`/`type` 切换语义与共享 CSS；各 Picker 保留领域 props 与逻辑
  （别名映射、pruning、全选、设置按钮插入位置）。
- 设置页更新时间按 `Asia/Shanghai` 展示（复用/扩展 `classTimeUtils` 格式化）。

## 非目标

- 不引入 radiogroup 等 a11y 重写（设计 D10 已锁定 aria-pressed 方案）。
- 不改 selectionContext 的 store/action 结构。
- 不动 TodayClassroomTable / GlobalEmpty 等其余组件。
- 不处理 F-05/F-07（capacity 语义、楼栋别名后端化——需产品确认）。

## 验收标准

1. 新组件 `frontend/src/components/ToggleButtonGroup.jsx` 导出
   `ToggleButtonGroup`（多选容器）与 `ToggleButton`（单按钮原语），配套单一
   `ToggleButtonGroup.css`；组件文件只导出组件（AGENTS.md 规则）。
2. `BuildingPicker`、`ClassTimePicker`（时段按钮）、`CampusButtonGroup`（校区按钮）
   均改用共享组件；三处不再各自维护重复的按钮切换/CSS 样板。
3. 现有测试行为零回退：三组测试的 `aria-pressed` 断言、选择 dispatch、
   disabled、全选/取消、设置按钮（无 aria-pressed）全部原样通过。
4. `ClassTimePicker` 特有样式（固定宽 48px、tabular-nums、time-slot-show-time、
   select-all 虚线样式）与 `CampusButtonGroup` 特有样式（settings-trigger、
   1024px 断点）保留在其原 CSS 文件或迁入共享 CSS 的 scoped 变体，视觉不变。
5. `CampusSettingsModal` 更新时间以 `Asia/Shanghai` 展示；为其加/改一条测试
   锁定时区行为（fake timers 或注入固定 Date）。
6. `pnpm lint` / `pnpm test` 全绿；`pnpm build` 成功（体积不回退——共享 CSS 应
   净减或持平）。
7. CHANGELOG.md Unreleased 记录设置页时区修复（用户可见）；无 API 契约变化。

## 约束

- 遵循前端规范：PascalCase 文件名、CSS 与组件同目录、两空格缩进、组件文件只导出组件。
- antd 版本与现有 import 不变；不新增依赖。
- 测试用 Vitest + Testing Library 现有风格，行为断言优先于快照。
