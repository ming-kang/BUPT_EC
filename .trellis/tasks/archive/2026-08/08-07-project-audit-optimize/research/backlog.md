# 优先级 Backlog（2026-08-07）

跨 `audit-backend.md` / `audit-frontend.md` / `audit-engineering.md` 汇总。  
只列**仍值得开实现任务**的项；已落地/关闭项见各审计对照表，不重复建任务。

**优先级启发式**：高收益低风险未落地 > 批次④「继续做」> 性能（JW/序列化）> 新工具链（TS/goreleaser）> 纯洁癖。

## 批次④总判定（必须项）

| 遗留项 | 判定 | 一句话依据 |
|---|---|---|
| 缓存新鲜度模型收敛 | **降级** | 对外字段即契约；仅允许内部 `freshness` 辅助，不删 JSON |
| refresh_coordinator → singleflight | **降级** | 手写协调器已测满；换库回归大、省行少 |
| Lifecycle Run/Shutdown + cancel | **继续做** | `WithoutCancel` + `StartWarmup` 侧门仍在 |
| TypeScript | **继续做** | 分阶段；契约对齐收益高 |
| React 19 | **降级**（跟 TS） | 单独升版收益低 |
| goreleaser | **降级** | 现发布链够用 |
| Dockerfile | **关闭** | 主路径非容器 |
| install.sh 大改 / bats | **降级** | 可小步抽模板；禁止为 bats 而 bats |

## 排序表

| Pri | ID | 标题 | 类别 | 工作量 | 契约 | 建议后续任务粒度 | 依赖/备注 |
|---:|---|---|---|---|---|---|---|
| 1 | B-01+B-02 | 生命周期 Run/Shutdown + 取消传播 | 可靠性 | M | 无 | `lifecycle-run-shutdown`（单一复杂任务） | 批次④唯一强继续；可顺带评估 B-06 |
| 2 | E-04 | nightly 发布覆盖代替删建 | 可靠性 | S | 无 | `nightly-release-clobber`（轻量） | 可与 E-05 同 PR |
| 3 | E-05 | release.sh 可移植替换 sed | DX | S | 无 | 并入上项或 `release-sh-portable` | macOS 静默损坏风险 |
| 4 | F-06 | 设置页时间 Asia/Shanghai | 可维护性 | S | 无 | 夹带任意前端小 PR | 无依赖 |
| 5 | F-03 | ToggleButtonGroup 统一 Picker | 可维护性 | M | 无 | `toggle-button-group` | 视觉回归 |
| 6 | B-07+B-11 | readyz 公网收敛 + 成功信封 log_id | 可靠性/DX | S | CHANGELOG | `api-surface-hygiene` | 运维若依赖 readyz 字段需先盘点 |
| 7 | B-08 | utils HTTP 并入 service | 可维护性 | S | 无 | `utils-into-service` | 低冲突 |
| 8 | F-01 | TypeScript 分阶段迁移 | 可维护性 | L | 无 | 父+子：`frontend-typescript` | 批次④主前端；可含 F-09 整理 |
| 9 | F-05+F-07 | capacity 语义 + 楼栋 display_name | 可维护性 | S–M | CHANGELOG | `classroom-display-contract` | 需确认 JW/产品语义 |
| 10 | B-04 | get_data 预序列化 + ETag | 性能 | M | CHANGELOG | `api-etag-preserialize` | 流量驱动；注意 stale/error 组合键 |
| 11 | B-03 | 冷路径有界等待 + 503 | 性能/可靠性 | M | CHANGELOG | `cold-path-bounded-wait` | **需产品批准**体验变化 |
| 12 | F-02 | React 19 | DX | M | 无 | `react-19` | **依赖** F-01 中后期 |
| 13 | E-03′ | install 模板小步抽取 | 可维护性 | S–M | 无 | `install-templates-extract` | 不做 bats；大改关闭 |
| 14 | E-06 | golangci-lint 最小集 | DX | M | 无 | `golangci-minimal` | 意愿驱动；同步 Task+CI |
| 15 | F-04 | antd B 档 / 首屏预算 | 性能 | M–L | 无 | `antd-b-tier` | 预算吃紧再开 |
| 16 | B-05 | 内部 freshness 辅助函数 | 可维护性 | S | 无 | 夹带 | 禁止改 wire format |
| — | B-06 | singleflight 迁移 | 可维护性 | M | 无 | **不建** | 降级；仅生命周期重写时再评 |
| — | E-01 | goreleaser | DX | L | 无 | **不建** | 降级 |
| — | E-02 | Dockerfile | DX | S–M | 无 | **关闭** | 除非明确容器需求 |
| — | E-08/E-09 | agent 去重 / module 改名 | 可维护性 | M–L | 破坏性(E-09) | **关闭** | 需单独批准 |
| — | B-09/B-10/B-12 | httpapi 搬家 / 测试再拆 / 多锁 | — | — | 无 | **关闭或降级** | 见 backend 报告 |
| — | F-08/F-10 | ClassTime effect / jsx-a11y | — | — | 无 | **关闭/降级** | 见 frontend 报告 |
| — | E-10 | 预压缩静态资源 | 性能 | M | 无 | **降级** | 与 gzhttp/Nginx 叠加风险 |

## 建议的实现任务切分（可直接 `task.py create`）

按推荐开单顺序（用户批准后另开，**本审计任务不创建实现子任务**）：

1. **`lifecycle-run-shutdown`** — B-01, B-02（复杂；需 design/implement）
2. **`release-hygiene`** — E-04, E-05（轻量 PRD-only）
3. **`frontend-picker-dedup`** — F-03（+可选 F-06）
4. **`api-surface-hygiene`** — B-07, B-11（+可选 B-08）
5. **`frontend-typescript`** — F-01（复杂；末期挂 F-02）
6. **`classroom-display-contract`** — F-05, F-07（跨层轻量）
7. 按需：**`api-etag-preserialize`**、**`cold-path-bounded-wait`**（后者要产品点头）

## 明确不做（本轮 backlog 剔除）

- 本地课表 DB / 多实例共享缓存（违反 spec 非协商规则）
- 回退 SWR / 回退 gzhttp / 回退 embed 标签方案
- 为「看起来整齐」而搬 `internal/httpapi`、全量 bats、全量去 antd（C 档）

## 证据命令摘要（审计期可选）

```text
go list -m all          → 约 40 个模块（相对 07-26 审计前 77）
go.mod 直接 require     → 7 项
frontend/package.json   → react 18.3 / antd 5.29 / swr 2.4 / vite 7 / 无 dayjs、无 @ant-design/icons
scripts/install.sh      → ~1079 行；install_test.sh ~1000 行
Dockerfile / .goreleaser.yml / .golangci.yml / tsconfig → 均不存在
```

未强制要求本任务 `task check` 全绿；上表不依赖新基准体积实测（预算文件已记载 209898 B / budget 230888 B）。
