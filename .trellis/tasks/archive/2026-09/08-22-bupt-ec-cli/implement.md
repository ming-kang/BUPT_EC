# 执行计划：bupt-ec 运维 CLI 与事务化分发

任务：`08-22-bupt-ec-cli`
前置：`08-22-installer-modes` 已归档

## 0. 基线与兼容快照

- [x] 记录当前 `scripts/install.sh` / fragments / installer test suite 的
  function inventory、场景数和相关 transaction function body hashes。
- [x] 标记本任务允许变化的函数：`stage_release`、`prepare_staging`、
  `transaction_targets`、`commit_installation`；记录其他 transaction functions
  必须保持 body hash 不变。
- [x] 保存 v0.2.0 Installer 无 `--mode`、v0.2.0 package 无 CLI 的 tag 证据，及当前
  release tar 精确布局/installer parity gate。
- [x] 运行基线：
  ```bash
  task installer:check
  git diff --check
  ```

**Gate**：基线失败先停止，不在本任务里掩盖既有失败。

## 1. CLI 骨架、解析与测试 seam

- [x] 新建 `scripts/bupt-ec-cli.sh`，启用严格模式并定义固定 production paths、
  `CLI_MIN_RELEASE=v0.3.0` 与唯一 build-version marker。
- [x] 实现 direct-execute / sourced-test guard；只允许 sourced tests 调用
  `configure_cli_test_root <absolute-root>`，生产不接受路径环境覆盖。
- [x] 实现 `cli_usage` / `cli_main` 固定分派：无参数、`-h`、`--help` 返回 0；未知、
  额外、重复或缺值参数返回 2，且在 root/network/system side effects 前失败。
- [x] 实现命令级 root matrix 和带原始安全参数的 `sudo bupt-ec ...` 指引。
- [x] 建立 `scripts/cli_test.sh`（必要时拆到 `scripts/cli_test/`），先覆盖 parser、
  source guard、production-path isolation 与权限矩阵。

**Validation**：

```bash
bash -n scripts/bupt-ec-cli.sh scripts/cli_test.sh
bash scripts/cli_test.sh
shellcheck scripts/bupt-ec-cli.sh scripts/cli_test.sh
```

## 2. 私有配置与公共 metadata 安全边界

- [x] 在 CLI 中定义十二字段只读 registry；实现 private env 目录/file
  regular/non-symlink、root owner、exact `0600` 校验。
- [x] 通过 fixed `/tmp` mode-`0600` frame、隔离 child source 与 strict NUL framing
  装载 private config；父进程不 source，source stdout/stderr/EXIT trap 不泄露，
  一次性开关不被激活，失败清空状态并清理 frame。
- [x] 实现 `config show` 固定 registry 输出；`JW_PASSWORD` / `JW_TOKEN` 无条件 `***`，
  其他值 shell-escape 为单行；未知 env 内容不输出。
- [x] 在 Installer `30-render.sh` 新增
  `render_deployment_metadata <destination>`，只从已验证
  `CFG_RELEASE_VERSION` / `CFG_APP_ADDR` 写严格两行格式，root:root `0644`。
- [x] CLI 实现 public metadata strict loader：安全父目录、regular/non-symlink、
  root owner、exact `0644`、exact key/order/count/value shape；绝不 source 或回退 private env。
- [x] 增加 private/public loader 的 symlink、mode、owner、writable parent、malformed/
  duplicate/extra record、source/framing/trap、multiline secret 和 cleanup tests。

**Security gate**：测试输出搜索所有 fixture password/token/source-secret；任何泄露先停止。

## 3. 只读查询、日志与服务控制

- [x] 实现 bounded/no-proxy/no-redirect `/healthz` / `/readyz` probe，并安全提取
  readyz `version`；记录 HTTP code 与可读状态。
- [x] `health`：两 probe 均 2xx 才返回 0。
- [x] `status`：一屏输出 active/enabled、selector、running version、两个 probe；
  unit active 且两 probe 2xx 才返回 0。
- [x] `version`：selector / running / CLI 三者分离；readyz 503 body 中的有效 version
  仍可成功展示，unreachable/invalid 显示 unavailable 并非零。
- [x] `logs`：默认 50，支持 `-f` / `-n positive` 任意顺序各一次，固定 unit，
  透传 journalctl status。
- [x] `start` / `stop` / `restart`：root gate 后仅委托 systemctl，回显 active state，
  不立即用 strict readiness 把正常 warmup 变成控制命令失败。
- [x] 覆盖 success、inactive、disabled、readyz 503、probe timeout/unreachable、
  malformed body、logs flag matrix/journal failure、service dispatch。

## 4. current/latest Installer 自举与 CLI 版本下限

- [x] 从安全 private snapshot 选择 repo/saved selector/mirror；目标版本为一个 CLI
  参数优先，否则 saved selector。
- [x] 复用 Installer 接受的 `latest|vX.Y.Z` 语义并实现 semver floor 比较；任何
  stable `< v0.3.0` 在 curl 前失败，输出 current/latest Installer fallback。
- [x] official bootstrap URL 固定为
  `https://github.com/${repo}/releases/latest/download/install.sh`；mirror 为
  `${DOWNLOAD_BASE_URL}/install.sh`；mirror 缺脚本 fail closed，不回退 GitHub/第三方。
- [x] Bootstrap URL 进行 secret-safe shape/protocol validation；HTTPS-only，保存的 HTTP
  mirror 只接受 exact `ALLOW_INSECURE_DOWNLOAD_BASE_URL=true`，不接受其他 scheme。
- [x] 以 fixed `/tmp` mode-`0700` session、bounded curl 和 EXIT cleanup 下载 Installer；
  `update` 传 `VERSION=<target> --mode=update`；`config` 在任何 network 前验证 TTY 后传
  `--mode=reconfigure`。
- [x] 断言 update stdin `/dev/null`、prompt count 0；config 保留 TTY；download/child
  status 原样传播；临时脚本总被清理。
- [x] 覆盖 official/latest、stable target、mirror、HTTP break-glass、invalid URL、
  missing metadata、pre-v0.3 no-curl rejection、saved target precedence。

## 5. CLI/metadata staging 与原子事务

- [x] `00-preamble.sh` 新增 `CLI_FILE`、`DEPLOYMENT_METADATA_FILE`、
  `CLI_MIN_RELEASE` 及 test-root 重定向。
- [x] 新增通用 semver floor helper：`latest` 与 stable `>=v0.3.0` 为 CLI-bearing；
  stable `<v0.3.0` 为 legacy。
- [x] 扩展 release archive fixture：CLI-bearing archive 含独立 `bupt-ec-cli`；新增
  missing-CLI 与 legacy-no-CLI fixtures。
- [x] `stage_release` 保持原 service binary 查找不变，单独 stage `bupt-ec-cli`；
  CLI-bearing missing member 在 snapshot 前失败，legacy absence 被接受。
- [x] `prepare_staging` 为 CLI-bearing target 渲染 metadata，并写 protected、严格值的
  CLI action marker；legacy target marker 为 remove 且无 CLI/metadata candidate。
- [x] `transaction_targets` 增加 `cli` / `metadata`；不得新增 CLI-specific snapshot 或
  rollback function。
- [x] `commit_installation` 验证 action；install 时通过 `atomic_install_file` 提交 CLI
  `0755` 与 metadata `0644`，remove 时检查式 unlink 两者；任一失败进入既有 rollback。
- [x] 测试 successful replacement、snapshot failure、commit/late health failure rollback、
  first-install cleanup、legacy direct rollback removal、legacy later failure restore、
  candidate owner/mode/action tamper。
- [x] 重新生成 `scripts/install.sh`，检查 fragment/artifact drift。
- [x] 对比步骤 0 hashes，除允许变化函数外所有受保护 transaction body hash 必须一致。

**Rollback gate**：若为 CLI/metadata 新增专用 snapshot/restore 分支，退回并改为复用
`transaction_targets` registry。

## 6. Release composition 与版本注入

- [x] `.github/workflows/release.yml` Compose step 计算与 Go binary 同源的 tag 或
  `main-<short-sha>` CLI version。
- [x] 校验 CLI source 中 exact-one marker，复制/替换为 package member
  `bupt-ec-cli` 并设 executable；两个 architecture package 内容相同。
- [x] 精确 tar member gate 加入 `bupt-ec-cli`，保留 packaged/generated installer
  byte parity，并断言 source fragments/test modules 不出现。
- [x] 保持顶层资产仍为两个 tarball、`checksums.txt`、`install.sh`；attestation/dry-run/
  publish lists 不新增独立 CLI asset。
- [x] 本地模拟 tag 与 main dry-run composition，解包校验 layout、CLI version、mode、
  installer parity 与 tarball checksum entry。
- [x] 运行 actionlint。

## 7. 质量门同步与回归保护

- [x] `Taskfile.yml` 的 `installer:check` 在 installer suite 后运行 CLI suite。
- [x] `.github/workflows/quality.yml` 同步两个行为 suite，维持 generator → syntax →
  behavior → ShellCheck 顺序。
- [x] `docs/development.md` 同步命令、CLI source/test layout 与 release simulation。
- [x] Suite 输出稳定场景/test count，避免拆分后无声丢覆盖。
- [x] 所有手工维护 shell source/test module 保持低于 1,000 行；CLI 尽量保持薄，
  复杂安全 helper 按职责拆分而非压缩可读性。

## 8. Spec、运维文档与 CHANGELOG

- [x] 更新 `.trellis/spec/backend/installer-guidelines.md`：
  - self-contained Installer 与 independent packaged CLI 的边界；
  - CLI/public metadata paths、mode/owner/format；
  - v0.3 floor 与 latest-bootstrap/legacy direct-Installer matrix；
  - 八 transaction targets 与 install/remove candidate action；
  - CLI behavior suite 和 release exact-layout gate。
- [x] 同步 backend index / quality guidelines / logging guidelines 的链接和 shell gate。
- [x] `README.md` 与 `docs/deployment.md`：安装后首选 `bupt-ec status` / help。
- [x] `docs/upgrading.md`：主推 `sudo bupt-ec update`，说明 >=v0.3 floor、pre-v0.3
  current Installer fallback、自定义 mirror 与 CLI 消失语义。
- [x] `docs/operations.md`：命令表、strict readiness exit、logs/config show、systemctl/
  journalctl 原始兜底。
- [x] `docs/release.md`：package layout、version marker、独立产物/self-contained boundary。
- [x] `CHANGELOG.md` `[Unreleased]` Added/Changed 记录 CLI、metadata 与 rollback floor。

## 9. 全量验证与独立终检

```bash
bash scripts/generate-install.sh --check
find scripts -type f -name '*.sh' -exec bash -c 'for script; do bash -n "$script" || exit 1; done' bash {} +
bash scripts/install_test.sh
bash scripts/cli_test.sh
find scripts -type f -name '*.sh' -exec shellcheck {} +

task installer:check
task check
task test
pnpm -C frontend build
rm -rf web/dist && cp -r frontend/dist web/dist
go build ./...
go build -tags embed_assets ./...

actionlint .github/workflows/quality.yml .github/workflows/release.yml
git diff --check
python ./.trellis/scripts/task.py validate 08-22-bupt-ec-cli
```

- [x] 独立 review 对照 PRD/design、Installer specs、v0.2 compatibility boundary、
  private/public secrecy、command exit codes、transaction rollback 与 exact release layout。
- [x] 搜索 repository/docs 中旧主推 `systemctl` / `curl update` 文案；保留的命令必须
  明确标为 raw/fallback，不得与 CLI floor 矛盾。
- [x] 确认无 credentials、logs、build artifacts、generated drift 或未识别 dirty files。

## Commit and Rollback Shape

CLI source、release package、Installer fragments/generated artifact、transaction fixtures、
spec/docs/CI 构成一个不可拆的发布契约，使用一个 scoped work commit：

```text
feat(cli): add transactional operations command
```

Trellis archive commit 单独生成。不得先提交 package layout 而留下不识别 CLI 的
Installer，也不得先提交 generated `install.sh` 而不提交 fragments。

回滚该 commit 只恢复仓库代码；已安装 v0.3 host 的 CLI/metadata 清理必须由一个知道
这些旧 target 的后续 Installer transaction 完成，不能假设 git revert 会触达服务器。

## Ready-to-Start Gate

- [x] PRD 完成 convergence pass，无 blocking open question。
- [x] design / implement 与当前 generated Installer 架构一致。
- [x] `implement.jsonl` / `check.jsonl` 包含真实 Installer/spec/research context，无 seed-only
  状态。
- [x] `task.py validate` 通过。
- [x] 用户在最终规划摘要之后明确批准实施；批准前不运行 `task.py start`。
