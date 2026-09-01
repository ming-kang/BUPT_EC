# 新增 bupt-ec 运维 CLI 并纳入原子事务

父任务：`08-22-ops-experience`
前置：`08-22-installer-modes` 已完成并归档

## Goal

在已部署服务器上提供 `/usr/local/bin/bupt-ec` 运维命令，让操作员用短、稳定的入口完成
升级、重配置、状态/版本/健康检查、日志查看和服务控制，不再记忆 `systemctl`、
`journalctl` 与长 `curl | bash` 命令。

用户价值：

- `bupt-ec update` 零问答完成日常升级；配置修改由 `bupt-ec config` 明确分离；
- 非 root 用户可完整查看服务状态、运行版本和健康结果，但不能读取部署秘密；
- CLI、服务二进制与其公开元数据作为同一 release transaction 提交/回滚；
- CLI 自身失效或回滚到引入前版本时，current/latest `curl | bash` 仍是稳定兜底。

## Background and Constraints

- 当前 Installer 由 `scripts/installer/*.sh` 确定性生成 tracked、自包含的
  `scripts/install.sh`；`--mode=update` 已是无 TTY/零 prompt 路径，
  `--mode=reconfigure` 是交互配置路径。
- 私有 `/etc/bupt-ec/bupt-ec.env` 是 root-owned `0600`，不能供非 root 查询命令读取。
- 当前 transaction registry 有六个目标，snapshot/rollback 迭代 registry，commit 显式
  安装 regular-file candidates。
- 当前 tarball 精确包含 binary、`.env.example`、`README.md`、`install.sh`；workflow
  锁定 member list 与 packaged/generated Installer byte parity。
- `v0.2.0` Installer 没有 `--mode`，v0.2.x tarball 也没有 CLI。CLI 不能通过下载
  target-old Installer 实现 `update v0.2.x`。
- authoritative Installer/release contracts 位于
  `.trellis/spec/backend/installer-guidelines.md`。详细证据见
  `research/2026-09-planning-refresh.md`。

## Requirements

### R1：命令面与解析

支持：

```text
bupt-ec update [VERSION]     零问答升级或选择 CLI-bearing stable release
bupt-ec status               active/enabled、selector、运行版本、健康摘要
bupt-ec version              配置 selector、运行版本、CLI 版本三者对照
bupt-ec logs [-f] [-n N]     固定 systemd unit 的 journalctl 包装，默认 50 行
bupt-ec restart              重启服务
bupt-ec start                启动服务
bupt-ec stop                 停止服务
bupt-ec config               交互重配置，调用 --mode=reconfigure
bupt-ec config show          固定字段配置视图，密码/token 脱敏
bupt-ec health               同时探 /healthz 与 /readyz
bupt-ec -h | --help          帮助；无参数等价于帮助
```

- 无参数/帮助返回 0；unknown、extra、duplicate、missing/invalid option 返回 2。
- 参数错误必须在 root、文件、网络或 systemd side effects 前失败。
- `logs` 的 `-f` / `-n <positive integer>` 可组合、各最多一次；其余参数拒绝。
- CLI 是薄 Shell 分派层；archive/checksum、package setup、render、snapshot、commit、
  rollback 委托 generated Installer，不在 CLI 复制第二套部署引擎。

### R2：路径、权限与版本

- 源文件为 `scripts/bupt-ec-cli.sh`；安装为 `/usr/local/bin/bupt-ec`，root:root `0755`。
- PATH 中命令与 service binary 不冲突；systemd 继续绝对执行
  `/opt/bupt-ec/bupt-ec`。
- `update`、`config`、`config show`、`start`、`stop`、`restart` 需要 root；不足时在任何
  side effect 前输出含原安全参数的 `sudo bupt-ec ...` 指引。
- `status`、`version`、`health`、`logs` 不主动要求 root；`logs` 透传本机 journal ACL
  的成功/失败，不静默 sudo。
- source 中保留唯一 build-version marker，本地报告 `dev`；release 注入与 Go
  `-ldflags` 相同的 tag 或 `main-<short-sha>`。
- source 被测试加载时只定义函数；production path 不接受环境路径覆盖，tests 仅通过
  sourced-only explicit seam 重定向固定路径。

### R3：私有配置与安全展示

- root-only 命令读取 private env 前校验安全父目录、regular/non-symlink、root owner、
  exact `0600`。
- private env 只能在隔离 child 中求值，并通过 protected temp file + strict NUL framing
  返回固定十二字段；CLI 父进程不得 source，installed env 不得激活一次性开关、trap
  或分派状态。
- source/framing/owner/mode/cleanup 失败必须清空结果、返回不含值的通用错误并删除
  temp frame；source stdout/stderr/EXIT trap 不得泄露。
- `config show` 只遍历固定 registry；`JW_PASSWORD` / `JW_TOKEN` 始终显示 `***`，其余
  值转为单行 shell-escaped 输出。未知语句、原始文件与 source 输出不打印，多行 secret
  不得从续行泄露。

### R4：非 root 公共元数据

Installer 从已验证 `CFG_*` 渲染 `/etc/bupt-ec/deployment.meta`：

```text
RELEASE_VERSION=<latest-or-vX.Y.Z>
APP_ADDR=<validated-host:port>
```

- exact 两行/固定顺序、root:root `0644`、regular/non-symlink，父目录保持 root-controlled
  且 group/other 不可写。
- 文件不得包含 repo、mirror、domain、证书路径、JW 字段、logging/readiness flag 或
  一次性开关。
- CLI 不 source metadata；它严格校验 owner/mode/type/parent、key/order/count/value。
- `status` / `version` / `health` 只读 metadata。缺失或不可信时明确失败，不回退 private
  env，也不把 custom `APP_ADDR` 静默替换为默认地址。

### R5：健康、状态与版本语义

- probes 使用 metadata `APP_ADDR`、bounded timeout、no proxy、no redirect。
- `health` 显示两个 endpoint，仅当两者均为 2xx 时返回 0。
- `status` 一屏显示 unit active/enabled、configured selector、running version 与两 probe；
  仅当 unit active 且两 probe 均为 2xx 时返回 0。正常 warmup readiness 503 显示为
  degraded/not ready 并返回非零。
- `version` 明确区分 selector（可能是 `latest`）、`/readyz` 的实际 build version 和 CLI
  version；readyz 503 的有效 body 仍可提供运行版本，unreachable/malformed 不能伪造。
- `start` / `stop` / `restart` 以 systemctl 操作结果为准并回显 active state，不立即以
  strict readiness 将正常重启预热误报为控制命令失败。

### R6：current/latest Installer 自举与版本下限

- `update` / `config` 从安全 private snapshot 取得 repo、saved selector、saved mirror。
- official source 始终下载
  `https://github.com/${RELEASE_REPO}/releases/latest/download/install.sh`；保存的固定 mirror
  使用 `${DOWNLOAD_BASE_URL}/install.sh`。启用 CLI 的 mirror 除 tarball/checksums 外必须
  同步该 self-contained Installer；缺失时明确失败且不回退第三方/官方源。target
  `VERSION` 只选择 archive，不选择旧 Installer implementation。
- bootstrap 使用 fixed `/tmp` mode-`0700` session、bounded curl、cleanup trap 与协议限制；
  official/mirror 默认 HTTPS。保存的 HTTP mirror 仍要求 exact
  `ALLOW_INSECURE_DOWNLOAD_BASE_URL=true`，不得扩展到其他 scheme。
- downloaded Installer 的信任边界与已公开 `curl | bash` 相同；release tarball 仍由
  Installer 的 `checksums.txt` 路径验证。
- `update` target = 单个命令参数 > saved selector；`config` 在 bootstrap network 前要求
  interactive TTY，再调用 `--mode=reconfigure` 并保留 saved selector。
- `v0.3.0` 是 CLI floor。`latest` 或 stable `>=v0.3.0` 可由 CLI 选择；任何 stable
  `<v0.3.0` 在 curl 前拒绝并打印 current/latest Installer fallback。

### R7：release package 与 transaction

Architecture tarball 增加独立 member（不能命名为 service binary）：

```text
bupt-ec-linux-${arch}/
  bupt-ec
  bupt-ec-cli
  .env.example
  README.md
  install.sh
```

- whole tarball checksum 覆盖 CLI；顶层 release assets 仍是两个 tarball、
  `checksums.txt`、self-contained `install.sh`，不新增 standalone CLI asset。
- CLI-bearing target (`latest` 或 stable `>=v0.3.0`) 必须含 exact `bupt-ec-cli`；缺失在
  snapshot 前失败。service binary 继续通过 exact `bupt-ec` 路径选择，不能被 CLI 混淆。
- `transaction_targets` 新增：

  ```text
  cli       /usr/local/bin/bupt-ec
  metadata  /etc/bupt-ec/deployment.meta
  ```

- staging 写 protected `install|remove` action。CLI-bearing target 原子安装 CLI `0755` 与
  metadata `0644`；direct current/latest Installer 选择 pre-v0.3 target 时允许 legacy
  archive 无 CLI，并在同一 transaction 中移除 CLI/metadata。
- snapshot/rollback 仅复用 target registry，不新增 CLI-specific restore 算法；首装失败
  不残留两文件，升级/legacy removal 后任何晚期失败都恢复两文件的先前状态。
- generated `scripts/install.sh` 继续自包含，不 source/call CLI；fragments/test modules
  仍是 repository-only inputs。

### R8：测试、质量门与文档同步

- 新增 focused CLI behavior suite，覆盖 parser、root matrix、private/public loaders、
  secret non-disclosure、status/health/version exit semantics、logs/service controls、
  bootstrap URL/protocol、zero-prompt update、reconfigure 与 pre-v0.3 no-curl rejection。
- generated Installer suite 增加 CLI-bearing/missing/legacy archives、stage mode/owner、
  successful replace、rollback、first-install cleanup、legacy removal/restore。
- 修改 transaction 前记录 function hashes；除 `stage_release`、`prepare_staging`、
  `transaction_targets`、`commit_installation` 与新增 helper 外，既有 transaction function
  bodies 保持不变。
- `task installer:check`、Taskfile、quality/release workflows、development docs 同步执行
  generator drift、recursive `bash -n`、Installer + CLI behavior suites、recursive ShellCheck。
- `.trellis/spec/backend/installer-guidelines.md` 显式区分 self-contained Installer runtime
  与 independently packaged/transaction-installed CLI，并记录 metadata、v0.3 floor、八
  targets、legacy action 与 exact release layout。
- 更新 `README.md`、`docs/deployment.md`、`docs/upgrading.md`、`docs/operations.md`、
  `docs/release.md`、`docs/development.md` 与 `CHANGELOG.md`。

## Out of Scope

- `uninstall`；删除 env/logs/units 需要独立确认与事务设计。
- Go CLI、Docker/Bats/goreleaser 或新的运行时依赖。
- CLI 联网上游版本检查；只有显式 update 下载 Installer。
- 在 CLI 重写 archive checksum、package install、render、snapshot、rollback。
- 改变 private twelve-field deployment schema、checksum trust model、Nginx/systemd template
  semantics 或 readiness endpoint behavior。
- 支持 `bupt-ec update` 跨到 pre-v0.3 target；该路径明确使用 current Installer fallback。

## Acceptance Criteria

- [x] 安装后 `bupt-ec -h` 与无参数帮助可用、返回 0并列出全部命令；无效参数在 side
  effects 前返回 2。
- [x] `sudo bupt-ec update` 以 saved selector 零 prompt/无 TTY 升级；显式 stable
  `>=v0.3.0` 可在 CLI-bearing releases 间选择/回滚。
- [x] `sudo bupt-ec update v0.2.x` 在任何 curl 前拒绝并给 current/latest Installer
  fallback；direct Installer rollback 到 v0.2.x 仍可用并事务化移除 CLI/metadata。
- [x] `bupt-ec config` 走 interactive reconfigure；`config show` 不泄露 password/token、
  source output 或 multiline secret。
- [x] 非 root `status` / `version` / `health` 从可信 metadata 完整工作；metadata 不可信时
  明确失败且不读 private env/猜默认地址。
- [x] `health` 仅双 2xx 返回 0；`status` 还要求 active；readiness 503 显示 degraded 并
  返回非零；`version` 分离 selector/running/CLI。
- [x] `logs` 参数矩阵与默认 50 行正确；服务控制 root gate、systemctl dispatch 与状态
  回显正确。
- [x] metadata 仅含 `RELEASE_VERSION` / `APP_ADDR`，root:root `0644`；CLI root:root
  `0755`；private env 仍 `0600`。
- [x] CLI、metadata、binary 在同一 transaction 中 replace/remove；upgrade failure 恢复
  两文件，first-install failure 不残留，legacy-removal late failure 恢复。
- [x] tarball exact layout 含 `bupt-ec-cli`，CLI 版本与 Go build version 同源，service
  binary selection、generated Installer parity、top-level asset list 与 checksum/attestation
  boundary 保持正确。
- [x] dedicated CLI suite、existing + extended Installer suite、recursive syntax/ShellCheck、
  `task installer:check`、`task check`、`task test`、tagless/embed builds、actionlint 与 local
  release-layout simulation 全绿。
- [x] Installer spec、README、deployment/upgrading/operations/release/development docs 与
  CHANGELOG 完整同步，无仍主推旧长命令或声称 CLI 可回滚到 pre-v0.3 的冲突文案。

## Key Decisions and Rationale

| Decision | Rationale / trade-off |
| --- | --- |
| Shell thin CLI | 复用成熟 Installer transaction；root 运维不塞进非特权 Go service |
| current/latest Installer bootstrap | 升级逻辑可自举修复；target v0.2 Installer 无 `--mode` |
| CLI floor `v0.3.0` | 用户选择拒绝跨引入边界的 CLI rollback，避免新 CLI/旧 service skew |
| direct Installer legacy removal | 保留既有 rollback 契约，同时让 pre-v0.3 target 不残留未来产物 |
| public two-field metadata | 用户选择保持非 root 查询完整；用最小公开面隔离 `0600` secrets |
| strict readiness exit | 用户选择让命令直接可用于自动化；接受 warmup 期短暂非零 |
| tar member `bupt-ec-cli` | 避免 `find -name bupt-ec` 把脚本误当 service binary |
| no standalone CLI asset | whole-archive checksum 已覆盖；保持顶层发布接口稳定 |
