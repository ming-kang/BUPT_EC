# 全面审计：性能与可维护性优化机会

## Goal

对当前（2026-07 减法重构之后）的 BUPT_EC 做一次全面审计，找出仍值得做的性能与可维护性优化点，按收益/风险排序，形成可执行后续 backlog。本任务**只产出审计报告与 backlog**，不改产品代码。

## Background

- 2026-07-26 已完成架构审计 + 批次①②③（`archive/2026-07/07-26-arch-simplify-refactor` 及子任务）：去 Gin、去 go-cache、SWR、依赖瘦身、embed 标签解耦、CI/卫生等。
- 当时排除的**批次④**仍未做：过期模型收敛、刷新协调器改 singleflight、生命周期 Run/Shutdown、TypeScript、React 19、goreleaser、Dockerfile、install.sh 大改等。
- 当前形态：Go 1.25.12 + stdlib `http.ServeMux`；`go.mod` 直接依赖约 7 项；前端 React 18.3 + antd 5.29 + SWR 2.4；单进程内存缓存、无 DB。
- 用户拍板交付形态：**A** — 本任务止于审计报告 + 优先级 backlog；实现另开任务。

## Requirements

- R1 以当前 `main` 源码、测试、CI、docs、`.trellis/spec` 为证据，覆盖 backend / frontend / engineering（构建·CI·发布·运维脚本），产出带 file:line 锚点的发现清单。
- R2 对照 07-26 三份 `research/audit-*.md` 与批次④遗留项：标出「已落地 / 仍有效 / 已过时 / 新发现」。
- R3 每条建议标注：类别（性能 / 可维护性 / 可靠性 / DX）、预估收益、风险、工作量、是否破坏对外契约。
- R4 给出优先级排序与建议执行切分（可独立验收的后续任务粒度），不默认扩大产品功能范围。
- R5 审计产物写入本任务 `research/`（建议拆 `audit-backend.md` / `audit-frontend.md` / `audit-engineering.md`，可另附 `backlog.md` 汇总排序）；报告用中文，风格对齐 07-26 审计。

## Out of Scope

- 本任务不修改应用/前端/脚本等产品代码（含「顺手修一下」）。
- 不引入本地课表数据库；不设计多实例共享缓存。
- 不重复论证批次①②③中已完成且仍成立的结论（只做状态核对）。
- 不在本任务内创建或启动实现向子任务（backlog 可建议后续任务标题/切分）。

## Acceptance Criteria

- [ ] `research/` 下有 backend / frontend / engineering 审计产物（或等价合并报告），条目含证据锚点与优先级元数据（R3 字段）。
- [ ] 批次④遗留项每项有「继续做 / 降级 / 关闭」判定，并说明依据。
- [ ] 有一份按优先级排序的后续 backlog（可映射为独立 Trellis 任务）。
- [ ] 对照表覆盖 07-26 三份审计中的主要条目（已落地/仍有效/过时/新发现）。
- [ ] 产品代码与本任务无关的工作树保持干净（仅允许任务目录与必要的 Trellis 元数据变更）。

## Key Decisions

| 决策 | 结论 |
|------|------|
| 交付形态 | A：报告 + backlog only |
| 任务结构 | 单任务（审计一体交付）；不建实现向子任务树 |
| 对照基线 | 07-26 `audit-*.md` + 批次④清单 |
| 报告语言 | 中文 |
