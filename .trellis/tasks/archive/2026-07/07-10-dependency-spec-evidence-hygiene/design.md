# 依赖、规范与验收证据同步设计

## Position in Task Graph

该任务是最终 integration hygiene child：

~~~text
five behavior/security children complete
  → code contracts stable
  → dependency metadata tidy
  → CI prevention
  → docs/spec/changelog consistency audit
  → evidence audit
  → parent full quality gate
~~~

如果本任务发现 production behavior 与 child PRD 不一致，停止文档修补并把问题退回对应
child；不得用文档描述掩盖代码缺陷。

## Source-of-Truth Matrix

| Contract | Primary source | Required consumers |
| --- | --- | --- |
| cache and Clock signatures | cache/cache.go, service/classroom_service.go | AGENTS, directory/runtime/quality specs, development docs |
| refresh backoff | refresh_coordinator.go + tests | AGENTS, operations, runtime/error specs, changelog |
| metrics encoding/login labels | main/router/token/metrics tests | operations, logging/quality specs, changelog |
| frontend reload policy | reloadSchedule tests + lifecycle tests | operations, API spec, AGENTS, changelog |
| mirror URL safety | install tests | deployment/upgrading/quality spec, AGENTS, changelog |
| Unicode sanitizer | jw_error tests | error/logging specs, AGENTS, changelog |
| release quality gate | quality.yml + actionlint | development/release docs, AGENTS |

文档更新以测试锁定的最终行为为准，不能从旧 changelog 或旧 task PRD 反向推断代码。

## Module Hygiene

使用受支持 Go toolchain 执行 tidy：

~~~text
go.mod direct/indirect classification
  + complete go.sum entries
  → go mod tidy -diff clean
  → go mod verify
~~~

只接受 tidy 机械结果；若 tidy 改变依赖版本，先调查 import graph 或 toolchain，不能在
hygiene commit 隐式升级。

## CI Placement

在 quality.yml 的 Set up Go 之后、gofmt/vet/test 之前增加：

~~~text
Check Go module tidiness: go mod tidy -diff
Verify Go modules: go mod verify
~~~

quality.yml 是 PR 和 release 的唯一 gate source，因此无需复制到 ci.yml/release.yml。
action refs、permissions 和 job dependency 不变。

## Documentation Convergence

按 matrix 逐项更新，然后使用 rg 做 negative audit。建议搜索：

~~~text
CacheStore
cache.New() *gocache.Cache
total or partial outcomes set 30 seconds
5s/10s/20s/30s
stale payload polls after 5 seconds
full mirror URL logging
ASCII-only control handling
~~~

有历史语境的文本可以保留，但必须明确标注为旧行为；当前指南和操作说明不得产生歧义。

## Changelog Convergence

- owning child 先在自身 commit 记录变化。
- 本任务只整理 Unreleased 分类和消除同一行为的冲突 bullet。
- 保留所有独立 Added/Changed/Fixed/Security/Dependencies 信息。
- 运行 extract-changelog.sh Unreleased，并检查输出非空、顺序稳定、无 generated notes
  配置变化。

## Evidence Policy

旧 archive 是历史事实，不修改。新任务以以下字段形成证据：

~~~text
implement checklist checked as work happens
validation section with command + result
task.json commit populated before archive
parent notes map child → commit
~~~

如 task.py archive 不自动写 commit，在 archive 前显式更新 metadata；不得在 commit 尚未
创建时预填 hash。

## Compatibility and Rollback

- go.mod/go.sum 与 CI check 应在同一 commit，避免短暂 untidy main。
- docs/spec/changelog 可与该 hygiene child 同 commit，但前置 child 的必要 changelog/spec
  仍必须随其行为 commit。
- 若 CI tidy check 与 Go 1.25.12 不兼容，先修 workflow 命令，不回滚已正确 tidy metadata。
