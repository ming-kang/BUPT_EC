# Design: ToggleButtonGroup 统一 Picker

## 组件 API

```jsx
// ToggleButtonGroup.jsx — 只导出组件
<ToggleButtonGroup
  options={[{ value, label, disabled?, content? }]}  // content: 自定义节点（时段按钮上下行时间）
  selectedValues={string[]}          // 多选选中集
  onToggle={(value) => void}         // 单值切换回调，dispatch 逻辑留在 Picker
  className?                         // 领域类名（time-slot 等）挂到容器
/>
<ToggleButton
  pressed={boolean}
  onClick={fn}
  className?
  disabled?
  ariaLabel?                          // settings-trigger 这类非选项按钮
>{children}</ToggleButton>
```

- `type={pressed ? "primary" : "default"}` 与 `aria-pressed={pressed}` 只写一次。
- 单选（CampusButtonGroup）：`pressed={activeCampusId === campus.id}` + `ToggleButton`；
  保持 aria-pressed 语义（D10），不写 radiogroup。

## 消费方改造

| Picker | 变化 | 保留 |
|---|---|---|
| BuildingPicker | 按钮组 → `ToggleButtonGroup`（options=buildings.map, label=displayBuildingName） | Card 壳、别名映射、dispatch SET_BUILDINGS |
| ClassTimePicker | 时段按钮 → `ToggleButtonGroup`（options 带 disabled+content）；全选按钮 → `ToggleButton`（无 aria-pressed 语义变化：全选按钮现状无 aria-pressed，Toggle 默认不设或显式跳过） | pruning、visibility 时钟、select-all 逻辑与虚线样式 |
| CampusButtonGroup | 校区按钮 → `ToggleButton`（单选） | Fragment 插入 settings-trigger、modal 懒加载、settings-trigger 样式 |

注意：`ToggleButton` 的 `aria-pressed` 仅在明确传 pressed 语义时输出；
select-all 与 settings-trigger 现状无 aria-pressed（测试锁定），API 上用
「不传 pressed 则不输出」或独立 prop 控制，保证零回退。

## CSS 合并

`ToggleButtonGroup.css` 收编三份 CSS 的公共部分：
- `.toggle-button-group`（flex/wrap/center/gap）
- `.toggle-button-group .ant-btn`（min-width 6em、radius、hover translateY、transition）
- 响应式 767/479 断点的 gap/padding/font-size 通用档

保留在原文件：`.class-time-picker`（固定宽/高、tabular-nums、disabled 色、
select-all 虚线、time-slot-show-time）、`.campus-button-group`（settings-trigger、
1024 断点）、`.building-picker` 若无剩余可整文件删除。
ClassTimePicker 容器 gap=6px 与默认 8px 不同 → 用领域 className 覆盖，不塞进公共 CSS。

## F-06 时区

`classTimeUtils.js` 新增导出 `formatShanghaiDateTime(date)`（`Intl.DateTimeFormat`
`zh-CN` + `timeZone: SHANGHAI_TZ`，`YYYY-MM-DD HH:mm` 风格）；
`CampusSettingsModal` 替换裸 `toLocaleString()`。测试注入固定 UTC 时间串断言输出。

## 测试策略

- 三组现有测试不改断言（回归保护）。
- 新增 `ToggleButtonGroup.test.jsx`：多选 toggle 回调、disabled 不触发、
  aria-pressed 输出/省略、content 渲染。
- CampusSettingsModal：现有测试文件存在则补一条时区用例。

## 风险

| 风险 | 缓解 |
|---|---|
| CSS 合并后视觉偏移 | 保留领域覆盖类；构建后人工核对；体积预算守门 |
| select-all/settings 误加 aria-pressed | API 设计上「未传 pressed 不输出」+ 现有测试锁定 |
| ClassTimePicker content 双行布局回归 | content 通道透传现有 JSX，不重写结构 |
