# 安装器模式拆分、配置持久化与源文件解耦

父任务：`08-22-ops-experience`

## Goal

把安装器从单一全交互脚本演进为可用于首装、无人值守升级和重配置的部署引擎，同时修复升级时丢失受支持运行时配置的问题，并将超大安装器/测试源拆成可维护模块。发布给用户的 `install.sh` 仍是一个可直接 `curl | bash` 的自包含文件。

用户价值：

- 日常升级不再重复回答域名、证书和教务凭据问题；
- update 可在无 TTY 的 cron/CLI 场景运行；
- `LOG_CALLER`、`READYZ_DIAGNOSTICS` 等受支持设置不会在升级后静默消失；
- 后续 `bupt-ec` CLI 能复用稳定的 `--mode=update` 入口；
- 安装器与测试不再由两个超过 1,000 行的手工维护文件承载全部职责。

## Background and Confirmed Facts

- `08-22-drop-nightly` 已完成；首装默认版本是 `latest`，`nightly` 会被明确拒绝。
- 当前 `scripts/install.sh` 为 1,218 行，`scripts/install_test.sh` 为 1,104 行。
- 当前 `require_installer_environment` 在解析参数前无条件要求 `/dev/tty`，因此原设计若只在后面分派模式，`update < /dev/null` 仍会失败。
- 当前 `load_current_config` / `CURRENT_*` / `render_env_file` 仅覆盖十项安装器配置；运行时同时公开支持 `LOG_CALLER` 与 `READYZ_DIAGNOSTICS`，现有成功升级会删除这两项。
- `ALLOW_INSECURE_DOWNLOAD_BASE_URL` 是每次运行重新确认的安全逃生阀，不属于持久化部署配置；`SKIP_CHECKSUM` 同样不得写入安装 env。
- 发布工作流与 release tarball 都要求顶层 `install.sh` 是一个自包含资产，不能要求目标机器同时下载 helper 文件。
- 事务函数和现有行为测试覆盖 checksum、归档、暂存、快照、原子替换、服务状态恢复、Nginx/健康检查、回滚失败保留恢复目录等关键路径。

## In Scope

### R1：三种明确模式与入口兼容

支持：

```text
install.sh --mode=install       首装/兼容入口，完整交互；不传 --mode 时的默认值
install.sh --mode=update        非交互升级或显式版本切换
install.sh --mode=reconfigure   交互修改配置，沿用已安装版本
```

- 同时接受 `--mode=value` 与 `--mode value`；其余参数快速失败并打印用法。
- 参数必须在 TTY 检查前解析：root 检查始终执行，TTY 只对 `install` / `reconfigure` 要求。
- 不传 `--mode` 的提示文案、问题顺序、默认值优先级、凭据条件分支、下载/事务行为和完成提示保持兼容。
- `source scripts/install.sh` 的测试入口继续跳过 `main`。

### R2：完整部署配置模型

定义一个由 `CURRENT_*` 与 `CFG_*` 两组变量承载的统一部署配置，包含：

```text
RELEASE_REPO RELEASE_VERSION DOMAIN SSL_CERT SSL_KEY
JW_USERNAME JW_PASSWORD JW_TOKEN APP_ADDR DOWNLOAD_BASE_URL
LOG_CALLER READYZ_DIAGNOSTICS
```

契约：

- 调用环境覆盖值必须在读取已安装 env 前快照，确保交互模式的优先级真实为“显式环境变量 > 已安装值 > 硬编码默认”。
- `load_current_config` 只负责装载 `CURRENT_*`，不得让 source env 覆盖调用者显式输入或激活一次性开关；只接受安全目录中的 regular、非 symlink、预期 owner、mode `0600` 文件，并通过隔离子 shell + 严格 NUL framing 返回注册字段。
- loader 的权限、语法、source 输出或 framing 失败必须清空 current、清理临时文件并返回不泄露 secret 的通用错误；此类故障指引人工修复/移走 env 后跑 install，而不是无法绕过同一 loader 的 reconfigure。
- `collect_config_interactive` 保持现有可见问答文案与顺序；prompt 的换行/重试反馈写 stderr，不能污染命令替换捕获的 token/password。`LOG_CALLER` / `READYZ_DIAGNOSTICS` 不新增 prompt，而是采用显式环境值、已安装值、空值的优先级。
- `adopt_current_config` 为 update 采用已安装配置；只有 `VERSION` 可显式覆盖已安装 `RELEASE_VERSION`。
- `render_env_file` 必须写出完整十二项配置并保持 mode `0600`、root 所有权与 shell-safe quoting。
- `ALLOW_INSECURE_DOWNLOAD_BASE_URL`、`SKIP_CHECKSUM` 和其他一次性执行开关不得持久化。

### R3：模式行为

#### install

- 默认模式，完整交互。
- 版本为显式 `VERSION`，否则已安装 `RELEASE_VERSION`，否则 `latest`。
- 执行依赖安装、用户创建、下载、暂存和唯一事务路径。

#### update

- 零 prompt，不读取 `/dev/tty`，必须支持 `< /dev/null`。
- 必须存在有效已安装 env；缺少 domain、证书路径、监听地址、release metadata 或有效 JW 凭据组合时，在下载/快照前失败并提示运行 `--mode=reconfigure`。
- 版本为显式 `VERSION`，否则已安装 `RELEASE_VERSION`；官方 GitHub 源支持升级和回滚到显式稳定 tag。若已保存自定义 mirror，显式版本与 saved version 不同则在下载前拒绝，因为固定 base URL 无法证明它提供目标版本；需用 install 同时明确匹配的 VERSION/base。
- 跳过 `install_packages`，但在事务前检查升级路径所需命令并给出明确恢复指引。
- 保留幂等 `create_user`、证书存在性检查、下载/校验/暂存和同一事务提交路径。

#### reconfigure

- 必须存在有效已安装版本；完整交互修改配置。
- 沿用已安装 `RELEASE_VERSION`，忽略 `VERSION`，避免把“改配置”变成隐式升级。
- 仍重新下载同版本资产并走完整事务路径，不新增“复用当前二进制”的第二条暂存分支。

### R4：消除位置参数配置扇出

- `render_env_file` 只接收 destination，配置从已验证的 `CFG_*` 读取。
- `prepare_staging` 只接收 archive/work/staging 路径，配置从 `CFG_*` 读取。
- 渲染输出的 systemd/Nginx 语义保持不变；配置字段增加仅体现在 env 文件完整持久化。
- 测试 fixture 通过统一 helper 设置 `CFG_*`，不再复制 11/13 个位置参数。

### R5：安装器源拆分与确定性单文件生成

- 在 `scripts/installer/` 下按职责维护 Bash 源片段：入口/常量、配置与模式、版本下载、渲染暂存、事务、编排。
- `scripts/generate-install.sh` 按显式固定顺序组装片段，原子写出并保持可执行的 `scripts/install.sh`。
- `scripts/install.sh` 继续纳入版本控制并带生成标记；`--check` 模式比较工作树产物，发现漂移时非零退出。
- 质量门和 release workflow 在测试/打包前运行生成漂移检查；release 仍只发布一个 `install.sh`。
- 生成器不得从目录枚举隐式决定顺序，避免文件名或平台排序造成不同产物。

### R6：测试源拆分与行为保护

- `scripts/install_test.sh` 变为小型 suite entrypoint。
- 公共断言、mock 环境/fixture、release policy、render/config、transaction/rollback、mode/entrypoint 场景拆到 `scripts/installer_test/`。
- 现有断言与场景语义必须保留；允许为拆文件移动函数和把位置参数 fixture 改为统一配置 helper，但不得降低或删除旧期望。
- 新增模式分派、无 TTY update、零 prompt、配置缺失、跳过 apt、reconfigure 版本固定、完整配置保留和生成漂移测试。
- 行为测试始终 source/执行生成后的 `scripts/install.sh`，保证测试对象就是发布资产。

### R7：事务与发布不变量

以下行为不得改变：

- checksum 与 archive 校验在 snapshot 前完成；
- 所有候选先进入 mode `0700` staging，env 为 root:root / `0600`；
- snapshot、原子替换、systemd/Nginx 激活、健康检查、成功清理和失败回滚顺序不变；
- 先前 inactive/disabled 的服务在回滚后保持原状态；
- rollback 不完整时保留 root-only recovery 目录；
- release tarball 与顶层资产仍包含同一个自包含 `install.sh`。

### R8：规范、CI 与文档同步

- 将 installer executable contracts 收敛到 `.trellis/spec/backend/installer-guidelines.md`，并在 backend index / general quality guidelines 中链接：
  - 用稳定函数签名/生成源路径替代易漂移的精确行号；
  - 删除 11/13 位置参数反模式，记录 `CFG_*` 契约；
  - 记录 update 有意跳过包安装、先做工具检查；
  - 记录生成 artifact 与 drift gate。
- 更新 `Taskfile.yml`、`.github/workflows/quality.yml`、`.github/workflows/release.yml` 和 `docs/development.md`，使生成检查、测试和 shellcheck 覆盖一致。
- 更新 `README.md`、`docs/deployment.md`、`docs/upgrading.md`、`docs/release.md`、`docs/operations.md` 与 `CHANGELOG.md`。

## Out of Scope

- 新增 `bupt-ec` CLI；该工作仍属于 `08-22-bupt-ec-cli`。
- 改写事务算法、增加持久化事务 journal、解决 SIGKILL/断电后的自动恢复，或修复“unit 文件缺失但 enablement link 已存在”这类预先损坏状态的 rollback reconciliation；这些是独立事务任务，当前以 baseline function hash 保证不漂移。
- 改变 checksum 信任模型、镜像安全策略、systemd/Nginx 模板语义或健康检查预算。
- 持久化 `ALLOW_INSECURE_DOWNLOAD_BASE_URL` / `SKIP_CHECKSUM`。
- 引入 Bats、Go 安装器、Docker 或新的运行时依赖。

## Acceptance Criteria

- [x] 不传 `--mode` 的交互问题、顺序、默认值和首装/已安装机器行为与基线一致。
- [x] `--mode=update` 在完整已安装配置上零 prompt、无 TTY 成功，并且不执行 `apt-get update` / `install_packages`。
- [x] update 的显式 `VERSION` 优先；无显式值时沿用已安装稳定版本；自定义 mirror 下的跨版本选择在下载前拒绝，不能写入与二进制不一致的 metadata。
- [x] update 缺关键配置或工具时在下载/快照前失败，并明确指引 `--mode=reconfigure` 或完整安装模式。
- [x] `--mode=reconfigure` 可更新配置但保持已安装版本，并走完整下载/事务路径。
- [x] 非法/重复/缺值参数快速失败并列出 `install|update|reconfigure`。
- [x] `LOG_CALLER` 与 `READYZ_DIAGNOSTICS` 在 install/update/reconfigure 后按优先级完整保留；一次性安全开关不被写入 env，且 `SKIP_CHECKSUM` 仍只接受精确值 `1`。
- [x] 不可信 current env（symlink、错误 owner/mode、可写目录、source/framing 异常）在任何部署工作前被安全拒绝，不泄露内容；prompt 返回值不含反馈换行。
- [x] `render_env_file` / `prepare_staging` 不再传递配置位置参数，所有调用点和 fixture 使用同一 `CFG_*` 契约。
- [x] 手工维护的安装器源和测试模块均低于 1,000 行；生成的 `scripts/install.sh` 作为自包含 artifact 被明确豁免。
- [x] `bash scripts/generate-install.sh --check` 能发现任何源/生成产物漂移。
- [x] 既有 installer 行为场景和断言语义全部保留并通过；新增模式/配置/生成测试通过。
- [x] `bash -n`、ShellCheck、`bash scripts/install_test.sh`、`task check`、release dry-run 资产布局检查全部通过。
- [x] 规范、CI、Taskfile、用户文档和 CHANGELOG 与新入口及生成流程一致。

## Key Decisions

- 用户选择把配置持久化修复和安装器/测试源拆分同时纳入本任务，而不是继续在超大单文件上追加模式分支。
- 发布接口仍是一个自包含 `install.sh`；模块化只影响仓库内源代码组织。
- update 的“无 TTY”要求高于原 `require_installer_environment` 结构，因此模式解析必须前移并将 TTY 检查按模式执行。
- 配置传递从位置参数改为统一 `CFG_*`，以同时解决字段遗漏和参数错位风险。
- 事务内核允许移动到独立源片段，但其行为、顺序和测试期望不变。
