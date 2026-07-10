# 依赖、规范与验收证据同步

## Goal

在五个行为修复子任务完成后，收口 Go module metadata、可复用 CI 门禁、AGENTS、
用户文档、Trellis specs、CHANGELOG 和新任务验收证据，使代码、自动检查和维护合同
描述同一个已验证状态，并防止同类漂移再次被归档。

## Background

- Go 1.25.12 下 go mod tidy -diff 当前失败。
- github.com/prometheus/client_golang 被 main.go 和 service 直接 import，却在 go.mod:36
  标记为 indirect。
- tidy 还要求补充若干 dependency test/module 的 go.sum 条目。
- .github/workflows/quality.yml 执行 install/audit/lint/test/build、gofmt/vet/race/build、
  govulncheck、installer tests 和 ShellCheck，但不检查 tidy diff。
- AGENTS.md:21 和 docs/development.md:75 仍描述已删除的 CacheStore。
- AGENTS.md:28、docs/operations.md:131 和多个 backend spec 仍称 total failure 固定 30s。
- docs/operations.md:198 与 CHANGELOG 较旧 bullet 仍记录 5/10/20/30 和 stale 5s，当前实现
  与最新 bullet 是 stale 15s、partial 30s、failure 10/20/30/60s。
- .trellis/spec/backend/directory-structure.md、runtime-state-and-cache.md、
  quality-guidelines.md 和 error-handling.md 含旧 cache/backoff 签名。
- 旧 07-10 归档任务的 implement/acceptance checklist 未勾选且 task.json.commit 为空。

## Dependencies

- 本任务必须最后实施。
- metrics、backend jitter、frontend jitter、installer URL 和 Unicode sanitizer 五个
  子任务必须先完成定向测试、spec 更新、CHANGELOG bullet、提交和归档。
- 本任务只做最终一致性审计和残留修正，不替代前置任务同 commit 的文档/变更记录义务。

## Requirements

### R1 — Go module 整洁性

- 使用受支持 Go 1.25.12 运行 go mod tidy，提交准确的 go.mod/go.sum diff。
- prometheus/client_golang 必须作为 direct dependency，transitive modules 保持 indirect。
- go mod verify 和 clean-cache/fresh module download 场景必须通过。
- 不在同一任务升级依赖版本；只收口现有 import graph metadata。

### R2 — CI 防回归

- reusable quality workflow 在 Go setup 后执行 go mod tidy -diff。
- CI 还必须执行 go mod verify，确保 checksum/module cache 合同。
- tidy check 失败时输出可读 diff 并阻止 PR、main 和 tag quality gate。
- 保持 Node/pnpm/Go/govulncheck/Actions pins、权限、release needs 和 asset layout。
- actionlint 必须通过。

### R3 — 规范和文档同步

- AGENTS、docs 和 .trellis/spec 使用 TodayClassroomCache、TodayClassroomsStore 和 Clock
  当前签名，不再使用 CacheStore/generic key/cast 合同。
- refresh 文档区分 total adaptive base ladder + bounded jitter、partial fixed 30s 和
  warmup missing-cache ladder。
- metrics 文档说明单层 gzip owner、login source/outcome 语义、loopback scrape 和
  public Nginx 404。
- frontend 文档记录最终 stale/partial/failure delay、正向 jitter、stale_until hard cap、
  visibility 和 40s fetch timeout。
- installer 文档记录只允许 HTTPS 或显式 HTTP、拒绝 userinfo/query/fragment/non-HTTP
  scheme、safe logging 和 curl redirect policy。
- logging/error spec 记录 Unicode control/space/format normalization 与 rune cap。
- deployment topology、API payload、release asset 和 single-instance 合同不得漂移。

### R4 — CHANGELOG 一致性

- [Unreleased] 中每个用户可见/安全变化由 owning child 在同一实现 commit 添加 bullet。
- 本任务合并重复分类和相互矛盾的旧/新轮询、backoff、metrics 描述，但不删除独立变化。
- 稳定历史版本 section 不重写。
- 最终 Unreleased 可被 scripts/extract-changelog.sh 完整提取，且没有互斥行为声明。

### R5 — 验收证据

- 不 retroactively 勾选或填写无法证明的旧归档任务。
- 父任务记录旧证据缺口，并以新六个 child 的实际验证补齐未来追踪链。
- 每个新 child 的 implement.md 必须按执行进度增量勾选，不允许归档时一次性伪填。
- 每个 child 在归档前记录验证命令/结果、实现 commit 和 spec/changelog 同步状态。
- 父任务最终记录六个 child commit 和完整集成门禁结果。

### R6 — Repository hygiene

- .agents/、.codex/ 和 .trellis/.template-hashes.json 保持不跟踪。
- 不把 frontend/dist、run_log、临时二进制或 audit 输出加入提交。
- git diff --check 和最终 git status 只显示预期 task/code/docs 变化。

## Acceptance Criteria

- [ ] Go 1.25.12 的 go mod tidy -diff 无输出且 go mod verify 通过。
- [ ] prometheus/client_golang 在 direct require block，go.sum 与 tidy 完全一致。
- [ ] PR/main/tag 共用的 quality workflow 会阻止 untidy module metadata。
- [ ] actionlint 证明 reusable workflow caller/callee、permissions 和 needs 有效。
- [ ] 搜索 CacheStore、旧 total fixed 30s 和旧前端轮询值不再命中当前合同描述。
- [ ] metrics、frontend、installer、Unicode 和 Clock 行为在 AGENTS/docs/spec 中一致。
- [ ] CHANGELOG Unreleased 无重复冲突，extract-changelog 对其成功。
- [ ] 旧归档未被伪造修改，新任务有增量 checklist、验证摘要和 commit 证据。
- [ ] .agents/、.codex/、模板哈希、dist、日志和临时产物未被跟踪。
- [ ] 完整 Go/frontend/installer/workflow/security 门禁通过。

## Out of Scope

- 依赖版本升级、Go/Node/pnpm pin 升级或 Dependabot/Renovate 引入。
- 修改业务代码来配合文档；若发现行为仍不符合前置任务合同，应退回 owning child。
- 重写历史 stable changelog 或伪造旧任务完成证据。
- 修改 Trellis runtime/template 本身。
