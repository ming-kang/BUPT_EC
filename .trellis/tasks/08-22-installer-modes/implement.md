# 执行计划：安装器模式、配置持久化与源文件拆分

任务：`08-22-installer-modes`
前置：`08-22-drop-nightly` 已归档

## 0. 基线与安全快照

- [x] 记录 `scripts/install.sh` / `scripts/install_test.sh` 行数、函数清单和 transaction function body hashes。
- [x] 运行并记录：
  ```bash
  bash scripts/install_test.sh
  bash -n scripts/install.sh scripts/install_test.sh
  shellcheck scripts/*.sh
  ```
- [x] 逐项记录现有交互 prompt：文案、顺序、默认表达式、secret 保留条件、必填条件。
- [x] 记录现有 release asset 组合路径和 `install.sh` checksum，作为生成 artifact/发布布局基线。
- [x] 搜索并记录 `render_env_file`、`prepare_staging`、transaction functions 的全部调用点。

**Gate**：基线失败则先停止；不在本任务中掩盖既有失败。

## 1. 建立确定性 installer generator（行为中性）

- [x] 创建 `scripts/installer/` 固定职责片段与 `scripts/generate-install.sh`。
- [x] 生成器使用显式 ordered array，不依赖 glob/locale 排序。
- [x] 支持普通生成与 `--check`；临时文件 + `chmod 0755` + 原子替换。
- [x] 将当前 `install.sh` 按函数所有权整块移动到片段；transaction functions 整块移动，不重写。
- [x] 生成并提交带 generated marker 的 `scripts/install.sh`。
- [x] 对比拆分前后函数清单、transaction body hashes 与行为测试。

**Validation**：

```bash
bash scripts/generate-install.sh
bash scripts/generate-install.sh --check
bash -n scripts/install.sh scripts/generate-install.sh scripts/installer/*.sh
bash scripts/install_test.sh
```

**Rollback point**：若 behavior test 或 transaction hash 漂移，恢复到单文件并重新划分片段，不继续模式开发。

## 2. 拆分 installer test suite（覆盖中性）

- [x] 创建 `scripts/installer_test/`：assertions、mocks/fixtures、policy、render_config、transaction、modes。
- [x] 将现有测试函数和失败注入场景移动到对应模块；不删除旧 assertion。
- [x] 让 `scripts/install_test.sh` 成为固定模块清单的 entrypoint，并先运行 generator `--check`。
- [x] 保持测试对象为生成后的 `scripts/install.sh`。
- [x] 在 entrypoint 输出 suite/test 计数，防止后续无声丢场景。

**Validation**：拆分前后旧 test function 清单与 scenario count 一致，`bash scripts/install_test.sh` 全绿。

## 3. 引入完整配置 registry 与调用环境快照

- [x] 定义十二项 `DEPLOYMENT_CONFIG_KEYS` 及 `CURRENT_*` / `OVERRIDE_*` / `CFG_*` 生命周期。
- [x] 在读取已安装 env 前捕获显式调用环境的 presence/value；在隔离子 shell 中 source，父环境保持不变。
- [x] `load_current_config` 装载 `LOG_CALLER` / `READYZ_DIAGNOSTICS`，并校验 regular/non-symlink、owner、`0600`、安全目录与严格 NUL framing。
- [x] source/framing/权限失败清空 current、清理 mode-`0600` 临时 frame 文件并给出可执行且 secret-safe 的恢复指引。
- [x] 增加 `reset_config_state` 与测试 `set_valid_test_config`，避免跨测试泄漏。
- [x] 添加优先级测试：显式值、显式空值、current、默认；`RELEASE_VERSION` 只来自 current，显式选择只认 `VERSION`。
- [x] 添加 prompt 返回值与 secret-safe failure assertions，确保反馈换行及错误不污染/打印 token/password。

**Validation**：配置测试覆盖十二项 round trip；现有交互默认仍匹配步骤 0 快照。

## 4. 删除配置位置参数扇出

- [x] 改为 `render_env_file <destination>`，读取已验证 `CFG_*`。
- [x] 改为 `render_nginx_site <destination>` 读取相关 `CFG_*`；systemd signature 保持最小。
- [x] 改为 `prepare_staging <archive> <work-dir> <staging-dir>`，读取 `CFG_*`。
- [x] 更新全部生产调用点和 test fixtures。
- [x] env 固定写出十二项；断言 root:root、`0600`、shell quoting 与可重新 source。
- [x] 断言 `ALLOW_INSECURE_DOWNLOAD_BASE_URL` / `SKIP_CHECKSUM` 不出现在 env。

**Gate**：rendered systemd/Nginx snapshot 语义不得变化；只有 env 增加受支持字段。

## 5. 模式解析与 mode-aware 环境检查

- [x] 增加 `INSTALLER_MODE=install` 与 `parse_mode`。
- [x] 覆盖 `--mode=value`、`--mode value`、缺值、未知值、重复参数、额外参数。
- [x] 拆分 `require_root_environment` 与 `require_interactive_tty`。
- [x] `main` 在 root/TTY/config load 前解析 mode；root 始终检查，TTY 只供 install/reconfigure。
- [x] 保持无参数 stdin entrypoint 的 root-check 行为，不重新引入 `BASH_SOURCE` 问题。

**Validation**：update 路径在不可读 TTY 和 stdin `/dev/null` 下能越过入口检查；交互模式仍明确要求 TTY。

## 6. 配置生产者与统一校验

- [x] 将现有 prompt 段纯搬移为 `collect_config_interactive`，逐项对照步骤 0 快照。
- [x] install 版本：explicit → current → latest。
- [x] reconfigure：require current version，忽略 `VERSION`；其余字段 explicit → current → defaults。
- [x] 实现 `adopt_current_config`：update 复制全部 current，仅 VERSION 可覆盖版本。
- [x] 实现 `validate_config`：repo/version/domain/path/app addr/download URL、JW 凭据组合、证书存在、模式所需 current metadata。
- [x] 所有失败发生在 download/snapshot 前，并给 mode-specific 恢复指引。

**Validation**：原 prompt 文案/顺序/条件 snapshot 全等；update prompt 调用计数为零。

## 7. 接入 mode-specific deployment

- [x] 抽取 `execute_deployment`，保持公共 download → staging → transaction 顺序。
- [x] install/reconfigure 调用 `install_packages`；update 跳过。
- [x] update 前运行 `require_update_tools`，覆盖 curl/tar/sha256sum/install/systemctl/nginx。
- [x] 三模式保留幂等 `create_user`。
- [x] reconfigure 重新下载 saved version，不复用已安装 binary。
- [x] `print_completion_summary` 保持 install 默认输出兼容，并为 update/reconfigure 增加明确结果。

**Validation scenarios**：

- [x] no-mode install on clean root
- [x] no-mode interactive run with existing env
- [x] update current version / explicit version rollback
- [x] update `< /dev/null`, zero prompt, zero apt
- [x] update missing config/tool failure before curl/snapshot
- [x] reconfigure changes config but not version
- [x] LOG_CALLER/READYZ persistence across all modes

## 8. 生成漂移、CI、Taskfile 与 release gate

- [x] `Taskfile.yml` 新增 installer generate/check task，并将 check 纳入 `task check`。
- [x] `quality.yml` 按 generator check → behavior tests → recursive bash-n/ShellCheck 执行。
- [x] `release.yml` Compose assets 前再次 `generate-install.sh --check`。
- [x] release dry-run 验证 tarball 与顶层 `install.sh` 为同一生成 artifact，且不含 runtime fragments。
- [x] 更新 shellcheck 命令覆盖 `scripts/installer/` 与 `scripts/installer_test/`。

## 9. 规范与用户文档同步

- [x] 将 Installer Release Selection 与 Transactional Installer 场景拆入 `.trellis/spec/backend/installer-guidelines.md`，并从 backend index/general quality guidelines 链接：
  - mode signatures / validation matrix；
  - CFG contract 与完整持久化字段；
  - 删除精确行号和 11/13 positional warning；
  - update 跳过 apt、工具 preflight；
  - generated self-contained artifact 与 drift check；
  - recursive test/lint commands。
- [x] 更新 `README.md` 与 `docs/deployment.md`：默认 install、三模式语义、传参示例。
- [x] 更新 `docs/upgrading.md`：无 TTY update、显式版本回滚、配置失败恢复。
- [x] 更新 `docs/release.md` / `docs/development.md` / `docs/operations.md`：生成源、质量命令、配置持久化。
- [x] 更新 `CHANGELOG.md` `[Unreleased]`。

## 10. 全量验证与终检

```bash
bash scripts/generate-install.sh --check
bash -n scripts/install.sh scripts/install_test.sh scripts/generate-install.sh \
  scripts/installer/*.sh scripts/installer_test/*.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh scripts/installer/*.sh scripts/installer_test/*.sh

task check
task test
pnpm -C frontend build
rm -rf web/dist && cp -r frontend/dist web/dist
go build -tags embed_assets ./...

git diff --check
python ./.trellis/scripts/task.py validate 08-22-installer-modes
```

- [x] 独立 review 对照 PRD、design、installer specs、原 transaction behavior 与 release layout。
- [x] 确认所有手工维护 installer/test 模块低于 1,000 行；仅 generated `install.sh` 记录豁免。
- [x] 确认工作树无未生成漂移、无 credential/log/build artifact。

## Completion Evidence

- Baseline：旧单文件 suite 13 个入口场景通过；记录 46 个 installer 函数、34 个 test/helper 函数、文件 hash、prompt 契约和 11 个 transaction body hash。
- 最终 installer gate：`task installer:check` 通过；generator drift、递归 `bash -n`、36 个入口场景 / 39 个 test 函数、递归 ShellCheck 全绿。
- Transaction：11 个受保护函数的 SHA-256 body hash 全部与基线一致；durable interruption 与预损坏 systemd link 状态明确延期。
- 仓库质量：`task check`、`task test`、普通 Go build、embed-assets build、frontend production build 与 164,862 B gzip bundle gate 全绿；前端 18 files / 127 tests 和两类 dependency audit 通过。
- CI/release：Taskfile、quality/release YAML 解析通过；actionlint v1.7.7 无输出通过；本地 release-layout 模拟验证两个 tarball 的精确文件表及 packaged/generated installer byte parity。
- 结构：所有手工维护 `scripts/installer/*.sh` / `scripts/installer_test/*.sh` 均低于 1,000 行；仅 tracked generated `scripts/install.sh` 作为自包含发布 artifact 豁免。
- Trellis：最终独立只读 review 无 blocker/high/medium/low finding；`task.py validate` 与 `git diff --check` 在提交前复核。

## Commit and Rollback Shape

按仓库“一项任务一个 scoped Conventional Commit”约定，生成源、生成 artifact、模式/配置行为、测试、CI/spec/docs 作为一个不可拆的工作提交落地：

```text
feat(installer): add generated deployment modes
```

原因：`scripts/install.sh` 是 fragments 的 tracked 派生产物；若先提交拆分、后提交行为，当前最终 fragments 与中间 artifact 无法在一次干净 staging 中保持 `--check` 与行为门同时为绿。Trellis 归档提交另行自动生成。

回滚使用一次工作提交 revert；env 新增字段向后兼容，release asset 名称/布局不变。

## Ready-to-Start Gate

- [x] PRD / design / implement 已完成最终收敛。
- [x] `implement.jsonl` / `check.jsonl` 无 `_example` 行且包含真实 spec/research。
- [x] 用户在最终规划摘要之后明确批准实施。
- [x] 通过 `task.py validate` 后才运行 `task.py start`。
