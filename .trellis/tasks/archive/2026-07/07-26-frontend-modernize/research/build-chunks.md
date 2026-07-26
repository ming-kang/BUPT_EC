# Research: 构建与体积（R8 — manualChunks / visualizer / CI 体积预算 / web-dist 联动）

- **Query**: vite.config.js 现状与产物基线；antd-vendor 分组在 pnpm 下的正确写法；rollup-plugin-visualizer 接入；CI gzip 体积预算方案；Vite 7 / Rollup 4 manualChunks 注意事项；web/dist 联动核实
- **Scope**: mixed（内部代码 + 实测构建 + npm registry / 官方文档）
- **Date**: 2026-07-27

## 1. vite.config.js 现状 + 产物基线（实测）

### 配置现状（frontend/vite.config.js，全文 35 行）

| 位置 | 内容 |
|---|---|
| vite.config.js:6 | `plugins: [react()]`，仅 @vitejs/plugin-react |
| vite.config.js:10-18 | `manualChunks(id)` 函数形式，仅拆 `react-vendor`（匹配 `/node_modules/react/`、`/node_modules/react-dom/`、`/node_modules/scheduler/` 三个包段） |
| vite.config.js:22-26 | vitest `environment: 'node'`（jsdom 由测试文件级指令 opt-in） |
| vite.config.js:27-34 | dev proxy `/api` → localhost:8080 |
| （缺省） | 无 `build.target`（Vite 6 默认 'modules'）、无 `chunkSizeWarningLimit`、无 visualizer、无体积断言 |

### 产物基线（2026-07-27，antd 5.12.6 / react 18.2.0 / vite 6.4.3，`pnpm -C frontend build` 实跑，dist 已保留）

| 文件 | raw (B) | gzip -9 (B) | 加载时机 / 内容 |
|---|---:|---:|---|
| `index.html` | 797 | 461 | 首屏 |
| `assets/index-mabor-gJ.js`（entry） | 414,808 | 135,910 | 首屏。业务代码 + **antd 核心（cssinjs/Button/Card/Alert/Spin/Typography/Empty/Tag/theme）+ @ant-design/icons**（含 12 处 `anticon` 标记） |
| `assets/react-vendor-Dv6xG0fm.js` | 143,457 | 45,847 | 首屏（modulepreload） |
| `assets/index-WFBypEFs.css` | 4,853 | 1,381 | 首屏 |
| `assets/TodayClassroomTable-BHNvPZQi.js` | 310,020 | 95,061 | 懒加载。**rc-table + rc-virtual-list 全链路在此**（grep 证实） |
| `assets/index-DPVRhY_J.js` | 29,120 | 9,489 | 懒加载（两个 lazy 组件的共享 chunk，含 **rc-dialog/Modal**） |
| `assets/CampusSettingsModal-WM2fax4v.js` | 13,242 | 4,468 | 懒加载 |
| `assets/TodayClassroomTable-DQdjHLRD.css` | 2,584 | 790 | 懒加载 |
| **合计** | 918,881 | **293,407** | |

- 首屏 gzip ≈ 183.6 KB（entry + react-vendor + css + html）；懒加载 gzip ≈ 109.8 KB。
- vite 自带 gzip 报告（zlib 默认级别）与 `gzip -9` 差 ~0.5%（如 entry 136.90 vs 135.91 KB），做预算断言时须固定用同一种算法。
- 懒加载点：App.jsx:21（`lazy(TodayClassroomTable)`）、CampusButtonGroup.jsx:8（`lazy(CampusSettingsModal)`）。两者都用 Modal → Rollup 自动生成了共享懒 chunk `index-DPVRhY_J.js`。

## 2. antd-vendor 分组写法（pnpm 严格结构）

### pnpm 路径事实（实测本仓库）

Vite 默认 `resolve.preserveSymlinks: false`，模块 id 是 **解析过符号链接的真实路径**，形如：

```
C:/Users/.../frontend/node_modules/.pnpm/antd@5.12.6_react-dom@18.2.0_react@18.2.0__react@18.2.0/node_modules/antd/es/button/index.js
C:/.../.pnpm/@ant-design+icons@5.2.6_.../node_modules/@ant-design/icons/es/...
C:/.../.pnpm/rc-table@7.36.1_.../node_modules/rc-table/es/...
```

关键点：
- `.pnpm/` 目录名里 scope 的 `/` 被折成 `+`（`@ant-design+icons@...`），且带 peer 后缀 —— **不要匹配 `.pnpm/` 段**；
- 但末尾恒有干净的 `/node_modules/<真实包名>/` 段 —— **锚定这一段匹配即可**，peer 后缀免疫。现有 react-vendor（vite.config.js:11-15）正是这么写的且已验证有效（Windows 下 id 已被 Vite 规范化为 `/` 分隔）。
- 切勿 `id.includes('antd')` 裸子串匹配：会误伤含 "antd" 的业务路径与 `.pnpm` 目录 peer 后缀（如 `_antd@...`）。

### 推荐写法

```js
// 依赖包家族（antd 5.29.3 仍是 rc-* + @rc-component/* 双家族并存，两者都要匹配）
const ANTD_ID = /\/node_modules\/(antd|@ant-design|rc-[a-z-]+|@rc-component)\//;
const REACT_ID = /\/node_modules\/(react|react-dom|scheduler)\//;

manualChunks(id) {
  if (REACT_ID.test(id)) return 'react-vendor';
  if (ANTD_ID.test(id)) return 'antd-vendor';
}
```

- **先后关系**：react-vendor 与 antd-vendor 的包集合不相交（antd 的模块 id 不会落进 react 匹配，反之亦然），顺序无关；保留 react 在前只为可读性。函数形式对每个 id 独立求值，无“先匹配者赢”的去重问题。
- **依赖归并**：Rollup 4 函数形式默认把被命中模块的依赖一并归并进该 manual chunk（除非依赖被其他 chunk 需要或已属于另一 manual chunk）。所以 antd 的传递依赖 —— 本仓库实测有 `dayjs@1.11.10`（注意：删掉直接依赖后它仍作为 antd 传递依赖存在）、`@babel/runtime`、`classnames`、`copy-to-clipboard`、`scroll-into-view-if-needed`、`throttle-debounce` 等 —— 会自动进 antd-vendor，无需逐一列举。业务代码不使用这些包，不会产生跨 chunk 撕扯。
- 本仓库 rc-* 家族共 34 个包（rc-util/rc-motion/rc-table/rc-dialog/...），@rc-component 家族 7 个 —— 正则前缀匹配全覆盖。

### 与懒加载 chunk 的交互（重要）

当前 rc-table + rc-virtual-list（约 95 KB gzip）**只**被懒加载的 TodayClassroomTable 引用，rc-dialog/Modal（约 10-14 KB gzip）只被两个懒组件引用。而 entry 静态 import 了 antd（Button/Card/Alert/...），所以 antd-vendor chunk 会被 entry 静态引用 = 首屏下载。因此：

- **若在 R6（删 antd Table）之前套用全量 antd-vendor 分组**：rc-table 全链路会被拽进首屏 vendor，首屏 gzip 暴涨约 +95 KB。**顺序上必须 R6/R7 先行（或同一 PR），R8 分组后再定基线**。
- R6/R7 完成后，懒加载侧剩下的 antd 部件只有 Modal/Switch/Divider（≈10 KB gzip），被并入首屏 antd-vendor 的代价小，换来单一稳定 hash 的 vendor（用户日常回访命中长缓存），可接受；acceptance 里“总 gzip 不增加”不受影响（归并不产生重复代码）。
- 若确要极致首屏，可用 `getModuleInfo` 反向遍历 importers、只把“从 entry 静态可达”的模块归入 antd-vendor（Rollup 官方文档有该模式的完整示例），懒加载独享的部分留给 Rollup 默认切分。**A 档下收益 ~10 KB，不建议做**，仅备案。

### hash 稳定性（acceptance：主 chunk 变、antd-vendor hash 不变）

Rollup 4 chunk hash = 自身内容（含它 import 的其他 chunk 的带 hash 文件名）。依赖方向是 entry → antd-vendor → react-vendor，vendor 不反向引用业务 chunk，所以业务改动只改 entry hash。验证方法：构建两次，第二次在 App.jsx 加一行注释，比对 `dist/assets/antd-vendor-*.js` 文件名不变（react-vendor 现状已具备同性质，可对照）。

## 3. rollup-plugin-visualizer

### 版本与兼容（npm registry 实查，2026-07-27）

| 版本 | 发布 | engines | peer |
|---|---|---|---|
| **7.0.1（latest）** | 2026-03-03 | **node >= 22** | rollup 2.x‖3.x‖4.x（optional）、rolldown |
| 6.0.11（prev） | — | node >= 18 | 同上 |

- Vite 7.3.6（7.x 最新，内置 rollup ^4.43.0）在 peer 范围内，兼容无虞。
- CI node 22（quality.yml:28）、本机 node 24.13 —— 用 **7.0.1**。若担心低版本 node 开发者，退 6.0.11（pnpm 默认不强制 engines，非硬约束）。

### 接法（环境变量开关，仅报告模式启用）

```js
import { visualizer } from 'rollup-plugin-visualizer';

export default defineConfig({
  plugins: [
    react(),
    // BUNDLE_REPORT=1 pnpm build 时生成报告；README 建议 visualizer 放插件数组最后
    process.env.BUNDLE_REPORT === '1' &&
      visualizer({
        filename: 'bundle-stats.local.html', // 见下：绝不能落在 dist/ 内
        gzipSize: true,
        brotliSize: true,
        template: 'treemap',
        open: false,
      }),
  ].filter(Boolean),
  ...
});
```

- Windows 下本项目开发用 Git Bash，`BUNDLE_REPORT=1 pnpm build` 直接可用；cmd/PowerShell 用户需 `set`/`$env:`（可在 README 注一句，不必引入 cross-env）。
- CI 若要留档：给 quality.yml 的 “Build frontend” 步骤加 `env: { BUNDLE_REPORT: "1" }` + 一个 upload-artifact 步骤传 `frontend/bundle-stats.local.html`（retention 3 天，与 frontend-dist 一致）。开销约 1-2s。

### 产物路径与 .gitignore（有陷阱）

- **报告文件绝不能写进 `frontend/dist/`**：Taskfile.yml:39-41 `cp -r frontend/dist web/dist` + web/embed_enabled.go:10 `//go:embed all:dist` 会把它嵌进发布二进制，且 router.go:198-206 会把 dist 根部任意真实文件当静态资源公开服务（`/bundle-stats.html` 可被下载）。插件默认 filename 是相对 frontend/ 根的 `stats.html`，本身安全，但要防手滑配成 `dist/stats.html`。
- gitignore：frontend/.gitignore 已有 `*.local` 规则 —— 报告命名为 `bundle-stats.local.html` 可**零改动**被忽略；若偏好显式，往 frontend/.gitignore 加一行 `stats.html`。

## 4. CI gzip 体积预算断言

### 方案对比

| 方案 | 现状 | 评价 |
|---|---|---|
| **自写 node 脚本**（推荐） | 零依赖，node:zlib + node:fs，~40 行 | CI 已装 node 22；无新 devDep、无 pnpm audit 面积、算法/阈值完全自控、报错信息可定制 |
| size-limit | 13.0.1（活跃，2026-07-24 发布） | 需 size-limit + @size-limit/file 两个 devDep（连带几十个传递依赖），配置载体另起；对本仓库“一个 dist 总量”场景是杀鸡牛刀 |
| bundlesize | 0.18.2（最后发布 2024-03） | 事实弃维护，历史上依赖 CI status token，排除 |

### 脚本草案（frontend/scripts/check-bundle-size.mjs）

```js
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

// 预算：升级 + A 档 + 拆 chunk 落地后用本脚本实测总量,加 ~10% 余量后回填。
// 过渡期先用今日基线(2026-07-27, antd5.12/vite6): 293,407 B,防倒退。
const BUDGET_BYTES = 294_000;

const dist = join(fileURLToPath(new URL('..', import.meta.url)), 'dist');
const files = [];
(function walk(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    e.isDirectory() ? walk(p) : /\.(js|css|html|svg)$/.test(e.name) && files.push(p);
  }
})(dist);

let total = 0;
for (const f of files.sort()) {
  const gz = gzipSync(readFileSync(f), { level: 9 }).length;
  total += gz;
  console.log(`${String(gz).padStart(9)} B gzip  ${f.slice(dist.length + 1)}`);
}
console.log(`total ${total} B gzip (budget ${BUDGET_BYTES} B)`);
if (total > BUDGET_BYTES) {
  console.error(`Bundle size budget exceeded by ${total - BUDGET_BYTES} B. ` +
    'If intentional, update BUDGET_BYTES with justification.');
  process.exit(1);
}
```

- 统计口径：dist 下 js/css/html/svg 的 gzip(level 9) 总和，排除 favicon.ico 等二进制。**阈值必须用同一脚本实测得出**（vite 控制台 gzip 是 zlib 默认级别，与 -9 差 ~0.5%，不可混用）。
- package.json 加 `"size": "node scripts/check-bundle-size.mjs"`。

### quality.yml 接线草案

在 “Build frontend”（quality.yml:52-54）之后、“Upload frontend dist”（:56）之前插入：

```yaml
      - name: Check bundle size budget
        working-directory: frontend
        run: node scripts/check-bundle-size.mjs
```

同步在 Taskfile.yml `check` 任务尾部加 `- pnpm -C frontend build` + `- node frontend/scripts/check-bundle-size.mjs`（当前 `check` 不含 build，若嫌重可只进 CI；quality.yml:1-3 的注释要求两处口径同步，改动时记得同步 docs/development.md）。

### 阈值怎么定

1. 依赖升级 + A 档 + 分组全部落地后，跑 `pnpm build && pnpm size` 记录实测总量 T；
2. `BUDGET_BYTES = ceil(T × 1.10)`（10% 余量吸收依赖 patch 升级抖动）；
3. 过渡期（R8 先行提交时）可先设 294,000 B（今日基线）当“防倒退”闸，A 档落地后收紧；
4. 可选二级断言：首屏子集（index.html + entry + react-vendor + antd-vendor + 首屏 css）单独预算，防止“总量不变但懒加载被拽进首屏”回归 —— 与 §2 的交互陷阱互为保险。

## 5. Vite 7 / Rollup 4 下 manualChunks 注意事项

- **函数签名没有变化**：Rollup 4 官方文档确认仍是 `(id, { getModuleInfo, getModuleIds }) => string | void`（Rollup 3 起如此）。现有函数写法平移即可。
- **新增选项 `output.onlyExplicitManualChunks`**（Rollup 4 后期新增，**Rollup 5 将改为默认**）：开启后函数形式不再自动归并被命中模块的依赖。本任务**依赖默认归并行为**（让 dayjs/@babel/runtime 等自动进 antd-vendor），升 Rollup 5 / Vite 8 时需回头检查此项。
- 硬冲突：manualChunks 与 `inlineDynamicImports` / `preserveModules` 互斥（报错）；本仓库未用。
- 官方警示原文：manual chunk 可能改变应用行为 —— 若被提前加载的模块有顶层副作用会提前执行。antd/es 是 sideEffects:false 的 ESM,风险低,但这是 §2 中“懒加载依赖被拽进首屏”除体积外的第二重代价。
- `splitVendorChunkPlugin` 在 Vite 7 已删除（5.2.7 起废弃）——本仓库未用，无迁移点。
- 若未来切 rolldown-vite（Vite 7+ 可选包）：manualChunks 被废弃，需迁移到 `output.advancedChunks`；本次留在 Rollup 即可，仅备案。
- Vite 7 默认 `build.target` 从 'modules' 变为 'baseline-widely-available'（Chrome 107/Edge 107/FF 104/Safari 16），产物只会更小或持平；audit 提到的“缺 build.target”在 Vite 7 下可继续留空。Node 要求 20.19+/22.12+（CI node 22 ✓，quality.yml:28）。
- chunk 超 500 KB raw 会有告警（`build.chunkSizeWarningLimit`）：若 R8 在 R6 之前落地，全量 antd-vendor 可能瞬时超限触发告警；按 §2 的顺序做则不会。
- 版本参考（npm 实查 2026-07-27）：vite 7.x 最新 = **7.3.6**（2026-06-25，内置 rollup ^4.43.0）；vite latest 已是 8.1.5，但 PRD 锁 7。@vitejs/plugin-react 与 vite 7 兼容的版本：**4.7.0**（现版本，peer 含 ^7.0.0，node 门槛宽）或 5.x（peer 含 ^7，engines node ^20.19‖>=22.12）；latest 6.0.4 **只支持 vite 8，不可用**。vitest 3.2.7（现版本）peer 已含 vite ^7.0.0-0 ✓。antd 5.x 最新 = 5.29.3（antd 6.5.2 已发布但超出 PRD 范围）。

## 6. web/dist 联动核实（结论：Taskfile / quality.yml / release.yml 均无需改）

| 环节 | 证据 | 对 chunk 结构的依赖 |
|---|---|---|
| Go 嵌入 | web/embed_enabled.go:10 `//go:embed all:dist` | 整目录嵌入，无文件名依赖 |
| Go 服务 | router.go:165-167 仅硬依赖 `index.html` 存在（缺失即 panic）；router.go:216 `GET /assets/` 前缀挂 immutableCache；router.go:189-209 其余路径先查真实文件否则回落 index.html | 仅依赖 ①`index.html` 在根 ②哈希资源在 `assets/` 下（Vite 默认 `build.assetsDir`，勿改）。chunk 数量/名字随意 |
| 占位构建 | web/embed_disabled.go:11-13 placeholder 只有 index.html；web/web_test.go:17 只读 index.html | 与真实 dist 结构无关 |
| Taskfile | Taskfile.yml:39-41 `rm -rf web/dist && cp -r frontend/dist web/dist` | 目录整拷，无文件名依赖 |
| quality.yml | :52-54 build；:56-61 上传整个 `frontend/dist` artifact；:97-104 整拷后 `go build -tags embed_assets` | 无文件名依赖 |
| release.yml | :45-49 下载 artifact 到 `web/dist`；:62 带 tag 构建 | 无文件名依赖 |

需要动 quality.yml 的唯一原因是**新增**体积预算步骤（§4）和可选的 visualizer artifact 上传（§3），不是 chunk 结构逼出来的被动改动。

## Caveats / Not Found

- **顺序硬约束**：全量 antd-vendor 分组必须在删 antd Table（R6）之后/同 PR 落地，否则 rc-table ~95 KB gzip 被拽进首屏（§2）。
- **visualizer 报告禁止写入 dist/**：会被 go:embed 进发布二进制并被公网访问到（§3）。
- visualizer 7.0.1 要求 node >= 22（CI ✓ 本机 ✓）；pnpm 默认不强制 engines，其他开发者低版本 node 只会静默无感，不是硬闸。
- @vitejs/plugin-react 不能追 latest（6.x 仅 vite 8）；留 4.7.0 或升 5.x。
- 体积预算脚本与 vite 控制台 gzip 数值口径不同（level 9 vs 默认），阈值只能由脚本自身实测回填。
- 未实测“升级到 vite 7 + antd 5.29 后”的产物尺寸（依赖尚未升级，属实现阶段动作）；本文基线为升级前快照，acceptance 的“总 gzip 不增加”以 293,407 B（gzip -9 口径）为对照点。
- frontend/dist 已按要求保留（本次构建产物即 §1 基线）。
