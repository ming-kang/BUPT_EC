# Research: 依赖升级面（R1/R2）

- **Query**: 现状盘点、目标版本矩阵、破坏性变更、pnpm.overrides 考证、CI 联动、升级策略
- **Scope**: mixed（内部代码/git 考古 + npm registry / Vite 官方迁移指南联网核实）
- **Date**: 2026-07-27（registry dist-tags 均为当日实查）

## Findings

### 1. 现状盘点

#### package.json（`frontend/package.json`）

| 类别 | 包 | 声明范围 | lockfile 实锁 | 备注 |
|---|---|---|---|---|
| deps | @ant-design/icons | ^5.2.6 | 5.2.6 | R7 将删除 |
| deps | antd | ^5.12.6 | 5.12.6 | 2023-12 版本 |
| deps | prop-types | ^15.8.1 | 15.8.1 | 批次④才删 |
| deps | react / react-dom | ^18.2.0 | 18.2.0 | 2022-06 版本 |
| dev | @eslint/js | ^9.39.4 | 9.39.4 | |
| dev | @testing-library/dom | ^10.4.0 | 10.4.0 | |
| dev | @testing-library/react | ^16.3.0 | 16.3.0 | |
| dev | @types/react / @types/react-dom | ^18.2.43 / ^18.2.17 | 18.2.x | |
| dev | @vitejs/plugin-react | ^4.7.0 | 4.7.0 | peer 已含 vite ^7 |
| dev | eslint | ^9.39.4 | 9.39.4 | flat config |
| dev | eslint-plugin-react / -hooks / -refresh | ^7.37.5 / ^7.1.1 / ^0.5.3 | 同 | 均为各自 latest |
| dev | globals | ^17.7.0 | 17.7.0 | |
| dev | jsdom | ^26.1.0 | 26.1.0 | latest 已到 29.1.1 |
| dev | **vite** | **6.4.3（无 ^，全文件唯一精确钉死，无注释）** | 6.4.3 | frontend/package.json:36 |
| dev | vitest | ^3.2.7 | 3.2.7 | dist-tag V3 = 3.2.7，即 v3 线终点 |

- `packageManager: pnpm@9.15.0`（frontend/package.json:6）；`pnpm-lock.yaml` `lockfileVersion: '9.0'`（frontend/pnpm-lock.yaml:1）。
- **dayjs 已删**：commit `9ad81ec`（"chore: drop unused dayjs dependency"）已从直接依赖移除——R1 该项已完成。dayjs 1.11.10 仍作为 antd→rc-picker 传递依赖留在 lockfile（frontend/pnpm-lock.yaml:943,3042），属正常，验收标准"package.json 无 dayjs"已满足。
- **测试框架**：vitest 3.2.7 + jsdom 26.1.0 + @testing-library/react 16.3.0。`vite.config.js:22-26` 设 `test.environment: 'node'` 为默认，仅 `src/useTodayClassrooms.lifecycle.test.jsx:2` 通过 `@vitest-environment jsdom` 文件级指令切换。9 个测试文件，无 setupFiles、无 coverage 配置。
- **eslint**：v9 flat config（`frontend/eslint.config.js`），@eslint/js recommended + react + react-hooks + react-refresh。注意 `eslint.config.js:27` 硬编码 `react: { version: "18.2" }`——升级时顺带改 `"detect"`（或 "18.3"）。
- 历史脉络：devDeps 在 commit `3c98705`（2026-07-10 "chore(frontend): refresh secure toolchain"）已整体刷新到当时最新（eslint 8→9 flat config、vite 5.0.8→6.4.3、加 audit 脚本、加 4 条 overrides）；**生产依赖（antd/react）从未动过**。所以本次真实迁移面是 antd 5.12→5.29、react 18.2→18.3、vite 6→7（不是 PRD 语境里的 5→7，5→6 已被 3c98705 吸收）。

### 2. 目标版本矩阵（2026-07-27 npm registry 实查）

⚠️ 生态已超前 PRD：antd latest=**6.5.2**、vite latest=**8.1.5**、eslint latest=**10.8.0**、react latest=19.2.8、vitest latest=4.1.10。本任务必须按 dist-tag 精确落在 5.x/7.x/18.x/9.x 线上，不能裸 `@latest`。

| 包 | 当前 | 目标 | 依据（dist-tag / peer / engines） |
|---|---|---|---|
| antd | 5.12.6 | **^5.29.3** | dist-tag `latest-5`=5.29.3；peer react >=16.9.0 ✓ |
| react / react-dom | 18.2.0 | **^18.3.1** | 18.x 线最后版本（registry 确认 18.3.1 为最末 18.x 正式版） |
| vite | 6.4.3 | **^7.3.6** | dist-tag `previous`=7.3.6（7.x 线终点）；engines `^20.19.0 \|\| >=22.12.0` |
| @vitejs/plugin-react | 4.7.0 | **^5.2.0**（或原地不动） | 4.7.0 peer 已含 `^7.0.0`（零改动可行）；5.2.0 是 vite≤7 的维护线终点（peer `^4.2‖^5‖^6‖^7‖^8`，node ^20.19‖>=22.12）；**6.x peer 只有 `vite ^8.0.0`，绝不可取** |
| vitest | 3.2.7 | **保持 3.2.7** | 其 vite 依赖范围 `^5.0.0 ‖ ^6.0.0 ‖ ^7.0.0-0` 覆盖 7.3.6，pnpm 会 dedupe 到工作区 vite；4.1.10（vite ^6‖^7‖^8）非必需，见 §3 |
| @testing-library/react | 16.3.0 | ^16.3.2 | **patch 而已，无主版本跳变**（16 仍是 latest 主线） |
| @testing-library/dom | 10.4.0 | ^10.4.1 | 同上 patch |
| @types/react / -dom | 18.2.x | ^18.3.x（range 内自然升） | latest tag 19.2.x 属 React 19，勿取 |
| jsdom | 26.1.0 | 可选 ^29.1.1 | vitest peer `jsdom: *`；三个 major 主要是 Node 门槛与 Web 平台行为修正，升后跑测试验证，红了回 26 |
| eslint | 9.39.4 | **^9.39.5（maintenance tag）** | eslint 10.8.0 已出但属独立迁移；@eslint/js 同取 maintenance 9.39.5；三个 react 插件已是 latest 无需动 |
| globals | 17.7.0 | ^17.8.0 | range 内 |
| prop-types | 15.8.1 | 不动 | 批次④ |
| @ant-design/icons | 5.2.6 | 不主动升（R7 删除） | 若升级提交先落地，^5.2.6 会自然解析到 5.6.x，恰满足 antd 5.29.3 内部 `^5.6.1`，可 dedupe，无冲突 |
| （新增）swr | — | ^2.4.2 | R3 用；latest=2.4.2 |
| （新增）rollup-plugin-visualizer | — | ^7.0.1 | R8 用；latest=7.0.1（实现时确认其 rollup4/vite7 peer，6.0.11 为 prev 备选） |
| pnpm | 9.15.0 | 可选 9.15.9（`latest-9`） | 同 lockfile 9.0 格式零迁移；pnpm 10/11（latest=11.17.0）有默认禁 lifecycle scripts 等行为变化，不在本次范围 |

### 3. 破坏性变更清单

#### Vite 6.4.3 → 7.3.6（官方 v7 迁移指南 https://v7.vite.dev/guide/migration.html 实查）

| 变更 | 对本项目影响 |
|---|---|
| Node 18 支持移除，要求 **20.19+ / 22.12+** | CI `node-version: 22`（quality.yml:28）解析最新 22.x ≥22.12 ✓；docs/development.md:8 已写 Node 22 LTS ✓ |
| **默认 build.target：'modules' → 'baseline-widely-available'**（Chrome 87→107、Edge 88→107、Firefox 78→104、**Safari 14→16**） | 唯一实质影响：产出语法基线抬高。老 iOS(<16) 设备将不兼容。R2 移动端回归需覆盖；若要保守可在 vite.config.js 显式 `build.target`（审计本就指出缺 build.target） |
| Sass legacy API 移除 | 不用 Sass，无影响 |
| `splitVendorChunkPlugin` 移除 | 不用；`manualChunks` 函数式写法（vite.config.js:10-18）不受影响，R8 在其上加 antd-vendor 即可 |
| `transformIndexHtml` 的 hook 级 enforce/transform 移除 | 不用，无影响 |
| Node API ESM-only（CJS 构建移除） | 项目 `"type": "module"` + ESM 配置文件，无影响 |
| Rollup 仍为 4（自 vite 5 起） | manualChunks 语义不变，无 rollup 3→4 迁移问题（早已吸收） |

（PRD/任务描述提的"5→7"里属于 5→6 的部分——CJS API deprecation、postcss-load-config 等——已在 3c98705 升 6.4.3 时吸收，无欠账。）

#### antd 5.12.6 → 5.29.3（semver-minor 线）

先 grep 了实际用的组件与 props：**Alert、Button、Card、ConfigProvider、Divider、Empty、Modal、Spin、Switch、Table、Tag、Typography、theme** 13 个 + icons 2 个（SettingOutlined、GithubOutlined）。逐项对照 5.13~5.29 弃用面：

- **零弃用 props 命中**：全源码无 `destroyOnClose`（5.25 起改名 destroyOnHidden）、无 `bodyStyle/headStyle`（Card 5.14 起弃用）、无 `visible=`（Modal 均用 `open`：CampusSettingsModal.jsx:12、TodayClassroomTable.jsx:167）、无 message/notification 静态方法、无 getPopupContainer。升级预期**无控制台弃用警告**。
- React 19 兼容 patch（@ant-design/v5-patch-for-react-19）不需要——本次留 React 18。
- 真实风险仅两处，皆由 R2 人工回归兜底：
  1. **Table**：rc-table 随 antd minor 演进，DOM/类名细节可能微调，而项目有 `.ant-table-*` + `!important` 覆盖（将被 R6 删除，但"升级先行、独立提交"的中间态里表格仍是 antd 的）→ 升级提交的视觉回归必须看主表。
  2. **暗色主题 token**：5.1x→5.2x 间 darkAlgorithm 个别 token 输出有细微调整，与 20 处 `body.dark` 手写 CSS 双轨并存时可能出现颜色不齐 → R2 暗色回归覆盖。

#### React 18.2.0 → 18.3.1

- 无运行时行为变化；只新增面向 React 19 的弃用警告（function component 的 `defaultProps`、`element.ref` 访问、legacy test-utils 等）。
- grep 证实：项目**零 `defaultProps`**；PropTypes 在 18.3 完全正常（19 才失效，批次④处理）；入口已是 `createRoot`（src/main.jsx:6）。预期**零新警告**。
- 顺带项：eslint.config.js:27 `react: { version: "18.2" }` → `"detect"`。

#### @testing-library / vitest / jsdom

- @testing-library：**不存在主版本跳变**（16.3.0→16.3.2、10.4.0→10.4.1 均 patch）。PRD 担心的迁移面为空。
- vitest 保持 3.2.7 → 零破坏。若将来上 4.1.10：workspace→projects、basic reporter 移除、mock 语义调整等，本项目用法简单（run 模式 + 文件级环境指令）实际影响≈0，但本次无收益不做。
- jsdom 26→29（可选）：Web 平台行为修正理论上可能影响 lifecycle 测试的 DOM/计时器行为——升后 `pnpm test` 验证，红了回退 26 并记录原因。

### 4. pnpm.overrides 考证（frontend/package.json:39-46）

四条全部来自 commit `3c98705`（2026-07-10 "refresh secure toolchain"，同一提交加了 audit:prod/audit:dev 门禁）。共同模式：**lockfile 陈旧导致解析停在有漏洞的旧版，用 override 强推补丁版以让 audit 过关**。逐条：

| override | 链路来源 | 原因推断 | 升级后处置 |
|---|---|---|---|
| `@babel/runtime` → 7.29.7（无范围限定，全树钉死） | antd 5.12.6 携带 2023 年的旧 7.23.x | CVE-2025-27789（RegExp 复杂度，<7.26.10，moderate）会挂 audit:prod | **可删**：antd 5.29.3 重新解析 `^7.x` 自然 ≥7.26.10。注意 @babel/runtime latest 已是 8.0.0，但 antd 依赖范围是 ^7，不会误升 |
| `brace-expansion@1.1.11` → 1.1.13 | eslint → minimatch 3.1.2 → brace-expansion | CVE-2025-5889（ReDoS） | **可删**：精确版本选择器，重装后树里不再出现 1.1.11 即成 no-op（fresh resolve 拿 maintenance-v1 = 1.1.16） |
| `minimatch@3.1.2` → 3.1.5 | eslint 9 仍依赖 `^3.1.2` | 3.1.5 是 legacy-v3 补丁线终点（dist-tag `legacy-v3`=3.1.5） | **可删**：`^3.1.2` fresh resolve 自动到 3.1.5 |
| `flatted@3.2.9` → 3.4.2 | eslint → flat-cache → flatted | 同模式（audit/陈旧） | **可删**：`^3.2.9` fresh resolve 到 3.4.3 |

验证流程：删除整个 `pnpm.overrides` 块 → `pnpm install`（全量重解析）→ 确认 lockfile 中不再出现 `brace-expansion@1.1.11` / `minimatch@3.1.2` / `flatted@3.2.9` / `@babel/runtime@7.2[0-5]` → `pnpm audit:prod && pnpm audit:dev` 全绿。若个别 audit 复红，只保留对应条目并在 PR 描述注明 CVE 编号。

### 5. CI / 工具链联动

- `.github/workflows/quality.yml:20-23`：`corepack prepare pnpm@9.15.0`；`:26-30`：`node-version: 22`（setup-node 解析最新 22.x，≥22.12，满足 vite 7 engines，**无需改**）。
- pnpm 版本引用共 3 处需保持同步：frontend/package.json:6、quality.yml:23、docs/development.md:9。若升 9.15.9 三处一起改；不升也成立（9.15.0 完全兼容本次所有目标版本）。
- **无 `.nvmrc`、package.json 无 `engines` 字段**。建议顺带加 `"engines": { "node": "^20.19.0 || >=22.12.0" }`（对齐 vite 7），防止本地旧 Node 静默出错——低成本防御，非必须。
- Taskfile.yml 仅转调 pnpm（:17,24,58-61），无版本引用；ci.yml/release.yml 无独立 Node setup（走 workflow_call 复用 quality.yml）。
- **vite 版本前缀策略**：`"vite": "6.4.3"` 是全文件唯一无 `^` 条目，3c98705 引入且 git log/blame 无任何说明——推测为当时安全刷新时的保守钉死。lockfile 本身已锁精确版本，caret 并不会造成日常漂移。建议统一为 `^7.3.6`；package.json 无法写注释，"加注释"落在 docs/development.md（Requirements 段）或提交信息里说明"deps 全部 caret + lockfile 锁定"。

### 6. 升级执行策略建议

**推荐：同任务内两个独立提交（均先于 R3+ 代码改动），而非单提交一次到位。**

- 提交 A（运行时/UI 面）：antd `^5.29.3`、react/react-dom `^18.3.1`、@types/react(-dom) 18.3 线、删除全部 4 条 pnpm.overrides、eslint/@eslint/js → 9.39.5、@testing-library patch 随动、eslint.config.js react version → "detect"。门禁：test/lint/build 全绿 + audit 双绿 + 亮/暗色与移动端视觉回归（重点：主表、Modal、暗色 token）。
- 提交 B（构建面）：vite `^7.3.6`、@vitejs/plugin-react `^5.2.0`（或维持 4.7.0，其 peer 已含 ^7）、可选 jsdom ^29。门禁：全绿 + 构建产物 diff（target 基线抬高导致的语法差异属预期）+ dev server 冒烟。

理由：
1. **回归归因隔离**——antd/react 影响视觉与 DOM，vite 影响产物与 dev server；混合提交后视觉异常或构建异常无法快速 bisect。
2. **中间态无死锁**——plugin-react 4.7.0 与 vitest 3.2.7 同时兼容 vite 6 和 7，antd 5.29 兼容 react 18.2/18.3，任意一步单独回滚都安全。
3. 一次到位技术上同样可行（兼容矩阵已全部验证无冲突），若强烈偏好最少提交可合并，代价是回归工作量叠加且不可归因。

操作方式：**直接编辑 package.json 的目标 range 后 `pnpm install`**（裸 `pnpm update` 受既有 range 限制不会跨 major/跨 minor 大步）；或 `pnpm add -D vite@^7.3.6 @vitejs/plugin-react@^5.2.0` 与 `pnpm add antd@^5.29.3 react@^18.3.1 react-dom@^18.3.1`。

明确**不做**清单：@vitejs/plugin-react 6.x（peer 仅 vite ^8）、vite 8、antd 6、eslint 10、react 19、vitest 4、pnpm 10/11。

## Related Specs

- 未发现 `.trellis/spec/` 下有前端依赖/工具链专项规范（本次为首个前端现代化任务）。

## Caveats / Not Found

- antd 5.13~5.29 逐版 changelog 未逐条通读（体量过大）；结论基于"项目零弃用 props 命中 + antd minor 线不破坏承诺"，残余风险收敛到 R2 人工视觉回归（表格与暗色）。
- rollup-plugin-visualizer 7.0.1 的 peer 范围未实查，实现 R8 时用 `pnpm add -D` 验证，不合再退 6.0.11。
- `pnpm audit` 未对升级后的假想依赖树预演（audit 只能对真实 lockfile 跑），overrides 可删结论需在提交 A 的 install 后用 audit 双门禁实证。
- jsdom 26→29 对 lifecycle 测试的具体影响未验证（列为可选项并给出回退路径）。
