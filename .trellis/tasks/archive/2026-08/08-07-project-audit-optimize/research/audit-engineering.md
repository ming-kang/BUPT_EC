# 工程化链路审计结论（2026-08-07）

对照基线：`archive/2026-07/07-26-arch-simplify-refactor/research/audit-engineering.md`。  
证据来源：`Taskfile.yml`、`.github/workflows/*`、`scripts/`、`docs/`、`.gitignore`、依赖与仓库杂项。  
总评：批次①③已落地 **embed 标签解耦、Taskfile、CI 缓存/artifact 复用、timeout、版本注入、gitignore/文档卫生**；工程债转为「**手写发布链 vs goreleaser**」「**install.sh 体量**」「**golangci-lint 仍缺**」「**agent 目录占比**」——多数应降级或关闭，避免为工具而工具。

## 07-26 对照总表（主要条目）

| 07-26 条目 | 现状 | 本轮状态 |
|---|---|---|
| go:embed 裸克隆全挂 | `web/` + `embed_assets` 双实现；无 tag 可 vet/test | **已落地** |
| Taskfile 构建编排 | `Taskfile.yml` build/test/check/vuln | **已落地** |
| quality 上传 dist、release 复用 | `quality.yml:60-65`；`release.yml` download artifact | **已落地** |
| pnpm cache / timeout / govulncheck go run | quality.yml 已配置 | **已落地** |
| 删 apt 装 shellcheck | 直接 `shellcheck` | **已落地** |
| ldflags 版本注入 | release + Taskfile | **已落地** |
| 删 dayjs / .gitignore / docs 表 | 已完成 | **已落地** |
| 静态 Cache-Control + index ETag | router 已实现 | **已落地** |
| GIN_MODE 矛盾 | 配置项已随去 Gin 删除 | **已过时** |
| golangci-lint | 仍无 `.golangci.yml`；仅 gofmt+vet | **仍有效** |
| nightly 删建空窗 | `release.yml:150-155` 仍 delete 再 create | **仍有效** |
| goreleaser | 无 `.goreleaser.yml` | **仍有效** |
| Dockerfile | 不存在 | **仍有效** |
| install.sh 大改 / bats | ~1079 / ~1000 行；自建 harness | **仍有效** |
| release.sh GNU-sed | `scripts/release.sh:65-69` `sed -i` | **仍有效** |
| env 应用/安装器拆分 | 仍 `bupt-ec.env`；snapshot 状态混在安装器逻辑 | **仍有效（收窄）** |
| agent 目录占比 | `.cursor` 52 + `.pi` 51 + `.trellis` ~811 文件 | **仍有效**（07-26 已拍板都保留） |
| 模块路径 `BUPT_EC` | 仍非 URL 形式 | **仍有效（低）** |
| 前端预压缩 .gz/.br | 未做（gzhttp 运行时压） | **仍有效（降级）** |

## 批次④遗留（工程相关）判定

| 项 | 判定 | 依据 |
|---|---|---|
| goreleaser | **降级** | 现有 quality→artifact→matrix build→attest→gh-release 完整可用；goreleaser 在矩阵仅 2 arch 时收益低、迁移成本中。 |
| Dockerfile | **关闭**（默认） | 生产路径是 `install.sh` + systemd/nginx；无容器部署需求前不要加第二条发布面。有人要容器再开可选任务。 |
| install.sh 大改 / bats | **降级** | 脚本质量高且 CI 绿；大改回归面=整个安装事务。可接受**小步**抽 template；bats 非必须。 |

---

## 发现清单

### E-01 · goreleaser 替代手写 release 链

- **状态相对 07-26**：仍有效
- **类别**：DX
- **证据**：`.github/workflows/release.yml`（quality-gate + build-go matrix + softprops/action-gh-release）；`scripts/release.sh`、`extract-changelog.sh`；无 `.goreleaser.yml`
- **建议**：**暂不引入**。若未来增加 OS/归档种类或要统一 SBOM，再写最小 goreleaser 配置并一次替换。
- **收益**：配置集中；当前重复逻辑有限。
- **风险**：中 — 易弄丢 attestation/nightly 语义。
- **工作量**：L
- **契约影响**：无（发布过程）
- **建议后续任务**：默认不建；记录为「发布矩阵扩张时再开」

### E-02 · Dockerfile / compose

- **状态相对 07-26**：仍有效
- **类别**：DX
- **证据**：仓库无 Dockerfile；安装器走二进制 tarball（`release.yml:98-109`）
- **建议**：**关闭**默认交付。若需要，另开任务：多阶段 build + distroless，文档标明「非主路径」。
- **收益**：仅对容器用户有价值。
- **风险**：低–中 — 双路径文档漂移。
- **工作量**：S–M
- **契约影响**：无
- **建议后续任务**：关闭，除非用户明确要容器

### E-03 · install.sh 体量与职责混杂

- **状态相对 07-26**：仍有效
- **类别**：可维护性
- **证据**：`scripts/install.sh` ~1079 行；`render_systemd_service` ~637+；`snapshot_installation`/`restore_snapshot_target` ~832+；`install_test.sh` ~1000 行自建断言
- **建议**：
  - **降级大重构**；
  - 可选小步：`render_*` → `scripts/templates/*.tmpl` + `envsubst`（−~100 行）；
  - bats 迁移仅在大改同开时考虑，否则测试框架重写无用户价值。
- **收益**：可读性；大改不降低故障率则 ROI 差。
- **风险**：大改高（事务回滚/权限/TTY）
- **工作量**：小步 S–M；大改 L
- **契约影响**：无（安装器 UX 需回归）
- **建议后续任务**：可选 `install-templates-extract`；**关闭** bats-only 任务

### E-04 · nightly delete-then-create 空窗

- **状态相对 07-26**：仍有效
- **类别**：可靠性
- **证据**：`release.yml:150-169` — `gh release delete nightly` 再 `action-gh-release` 创建
- **建议**：改为上传覆盖同 tag assets（或 `gh release upload --clobber`），避免短暂 404。
- **收益**：nightly 消费者（install 默认？）减少竞态失败。
- **风险**：低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：`nightly-release-clobber`（轻量，高性价比）

### E-05 · `release.sh` GNU-sed / 版本正则脆弱

- **状态相对 07-26**：仍有效
- **类别**：DX / 可靠性
- **证据**：`scripts/release.sh:65-69` `sed -i` 无 BSD 分支
- **建议**：改用 Python/`go run` 小工具或 `perl -pi` 可移植写法；加强 version 校验。
- **收益**：macOS 维护者不静默写坏 package.json/CHANGELOG。
- **风险**：低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：`release-sh-portable-sed`（可与 E-04 同卫生任务）

### E-06 · 缺少 golangci-lint

- **状态相对 07-26**：仍有效
- **类别**：DX
- **证据**：无 `.golangci.yml`；`quality.yml:83-93` 仅 gofmt+vet；历史决策曾刻意不引入以保持 Task/CI 等价
- **建议**：可选引入精简集（errcheck/ineffassign/misspell）；**必须**同步 Taskfile `check` 与 quality.yml（见 spec quality-guidelines）。
- **收益**：多抓一类静态问题；非 blocker。
- **风险**：低–中 — 首次会刷存量告警，需 allowlist/渐进。
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：**降级**；有存量意愿时开 `golangci-minimal`

### E-07 · `task check` 故意跳过体积预算

- **状态相对 07-26**：新发现（相对 07-26 基线；是落地后的新偏差）
- **类别**：DX
- **证据**：`Taskfile.yml:48-54` 注释说明 skip size；`quality.yml:56-58` CI 仍跑 `check-bundle-size.mjs`
- **建议**：保持现状并在 docs 强调「改前端依赖/chunk 后本地必跑 size」；或提供 `task check:full` 含 build+size。
- **收益**：避免本地/CI 漂移误判。
- **风险**：极低
- **工作量**：S
- **契约影响**：无
- **建议后续任务**：文档/Task 小补丁（可选）

### E-08 · agent 目录入库占比

- **状态相对 07-26**：仍有效（07-26 拍板保留 `.cursor`+`.pi`）
- **类别**：可维护性
- **证据**：`.cursor` ~52 文件、`.pi` ~51、`.trellis` ~811；技能目录曾字节级重复
- **建议**：遵守既有决策：**不在本审计后强行去重**。若未来选单一 agent 平台，再 gitignore 另一侧；Trellis 上游分发物是否减负另议。
- **收益**：clone 体积/噪声
- **风险**：中 — 影响本地 agent 工作流
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：关闭（需产品/工作流再批准）

### E-09 · Go module 路径非 URL 形式

- **状态相对 07-26**：仍有效（低优先）
- **类别**：DX
- **证据**：`go.mod` `module BUPT_EC`；不可 `go install github.com/...`
- **建议**：仅当要对外 module 消费时全仓改前缀；当前二进制分发为主，**关闭**。
- **收益**：低
- **风险**：高（全仓 import）
- **工作量**：L
- **契约影响**：破坏性（import path）
- **建议后续任务**：关闭

### E-10 · 前端预压缩资产（.gz/.br）

- **状态相对 07-26**：仍有效（降级）
- **类别**：性能
- **证据**：运行时 gzhttp；无 vite-plugin-compression；hashed assets 已 immutable
- **建议**：在 B-04 API 预序列化或高静态流量前不要做；Nginx 也可承担。
- **收益**：边际（已有运行时压 + 长缓存）
- **风险**：中 — 与 gzhttp Vary/预压缩协商易踩坑
- **工作量**：M
- **契约影响**：无
- **建议后续任务**：降级

### E-11 · 应用 env 与安装器状态边界

- **状态相对 07-26**：仍有效（收窄）
- **类别**：可维护性
- **证据**：`install.sh:9` `ENV_FILE=.../bupt-ec.env`；snapshot/runtime 逻辑仍在安装器（~783+）；`.env.example` 为应用模板
- **建议**：继续避免把安装器私有状态经 EnvironmentFile 注入 Go 进程；若仍有混入变量，拆 `installer.state`。
- **收益**：配置面清晰
- **风险**：低
- **工作量**：S–M
- **契约影响**：可能影响已部署机升级路径 — 需 CHANGELOG/升级说明
- **建议后续任务**：仅在确认混入变量仍存在时开；审计期不强制

---

## 建议执行顺序（工程）

1. **E-04 + E-05** — nightly 空窗 + sed 可移植（小、实）  
2. **E-07** — check/size 文档或 `check:full`（可选）  
3. **E-03 小步 templates** — 有维护痛感再做  
4. **E-06 golangci** — 意愿驱动  
5. **E-01 / E-02 / E-08 / E-09 / E-10** — 降级或关闭  
