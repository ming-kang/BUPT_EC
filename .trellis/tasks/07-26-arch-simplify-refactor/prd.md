# PRD：架构简化重构：依赖瘦身与技术栈现代化（父任务）

## 角色

父任务：持有需求来源（三份审计）、子任务地图、跨子任务验收标准与最终集成审查。无直接实现工作，不进入 in_progress。

## 目标与用户价值

基于 2026-07-26 的三份架构审计（`research/audit-backend.md`、`research/audit-frontend.md`、`research/audit-engineering.md`），做一次以"减法"为主的重构：删除过度抽象与无关依赖、用标准库/成熟库替代手写基础设施、升级陈旧的前端依赖。对外行为保持不变或仅有显式批准的增量改进。

价值：go.mod 声明依赖 43 → 15 行（9 直接 + 34 indirect → 7 直接 + 8 indirect），完整 module graph（`go list -m all`）77 → 40（被动 CVE 面大幅收缩）；净删千行级手写基础设施；裸克隆可开发（当前无前端产物时 `go vet` 直接失败）；修复 gzip 协议缺陷、Modal 过期数据等真实 bug。

## 已确认决策（2026-07-26，用户拍板）

| 决策点 | 结论 |
|---|---|
| 范围 | 批次①②③；批次④（过期模型收敛、生命周期 Run/Shutdown、TypeScript、goreleaser、Dockerfile）排除，另开任务 |
| antd 档位 | A 档：只删 Table + @ant-design/icons + 拆 antd-vendor chunk |
| gzip 策略 | Go 侧换 klauspost/compress/gzhttp；不下放 Nginx，install.sh 不动 |
| agent 目录 | `.cursor/` 与 `.pi/` 都保留入库，不去重 |
| React 版本 | 本次升 18.3.1；React 19 与 TS 迁移一起留批次④（prop-types 在 19 失效）——技术推导，非用户显式拍板 |

## 子任务地图（执行顺序）

| # | 任务目录 | 内容 | 复杂度 |
|---|---|---|---|
| 1 | `07-26-hygiene-quick-wins` | 删 dayjs、.gitignore、CI 缓存/timeout/去重、版本注入、文档矛盾 | 轻量（PRD-only） |
| 2 | `07-26-backend-subtraction` | 删 cache 包/go-cache、Noop 指标默认化、死代码、退避合并、测试收敛 | 复杂 |
| 3 | `07-26-degin-http-rewrite` | 去 Gin→ServeMux、gzhttp、embed 构建标签解耦、Taskfile、logs 去 gin | 复杂 |
| 4 | `07-26-frontend-modernize` | 依赖升级、SWR、原生表格、chunk 拆分、Modal/effect 修复 | 复杂 |

顺序依赖（写入各子任务 PRD）：1 → 2 → 3；4 与 2/3 无代码冲突但建议最后（视觉回归成本）。每个子任务独立提交、独立可回滚。

## 跨子任务验收标准

- 全程 `go test -race ./...`、`pnpm test`、`pnpm lint` 保持绿。
- API 契约不变：`/api/get_data` 信封（code/msg/data/log_id 语义）、`/healthz`、`/readyz`（允许新增 version 字段）、`/metrics` 指标名与标签。显式批准的例外：响应头 `LogID` → `X-Log-Id`、`GIN_MODE` 配置项移除（均记 CHANGELOG）。
- 最终态：go.mod 直接依赖不含 gin、gin-contrib/static、patrickmn/go-cache；前端无 dayjs、@ant-design/icons。
- 干净检出无前端产物时 `go vet ./...`、`go test ./...` 可运行。
- 每个子任务归档前完成全量 2.2 检查（backend spec Quality Check）并按 Conventional Commit 分批提交。
- 集成审查（父任务收尾）：四个子任务全部归档后，跑一次完整构建 + 手动冒烟（本地起服务、前端查询流程、暗色模式），核对 CHANGELOG 与 docs 同步。

## 范围外

批次④全部；install.sh 大改与 bats 迁移；模块路径改名；antd B/C 档；楼栋 display_name 下沉；`.cursor`/`.pi` 去重。

## 开放问题

无（规划期决策已全部收敛；子任务 2/3/4 的技术细节在各自 design.md 中展开）。
