# 新增 bupt-ec 运维 CLI 并纳入原子事务

父任务：`08-22-ops-experience`
前置：**强依赖 `08-22-installer-modes`**（`update` 子命令直接调用 `--mode=update`）

## Goal

让服务器上有一个 `bupt-ec` 命令承担日常运维。用户不必再记 `systemctl` / `journalctl` / `curl` 三件套，也不必为升级去翻文档找 `curl | bash` 那行长命令。

## Requirements

### R1 子命令集

```
bupt-ec update [VERSION]     升级或切换版本（默认沿用已保存频道）
bupt-ec status               服务状态 + 运行版本 + 健康摘要，一屏说清
bupt-ec version              配置频道 / 运行版本 / CLI 版本 三者对照
bupt-ec logs [-f] [-n N]     journalctl -u bupt-ec 的包装
bupt-ec restart              重启服务
bupt-ec start                启动服务
bupt-ec stop                 停止服务
bupt-ec config               重新配置（交互式，调用 --mode=reconfigure）
bupt-ec config show          打印当前配置，密码与 token 脱敏
bupt-ec health               探 /healthz 与 /readyz
bupt-ec -h | --help          帮助（无参数时也显示）
```

**设计枢纽是 `update` 与 `config` 的分离**：`update` 一个问题都不问，`config` 才走交互问答。这正是本次改造要解决的痛点。

### R2 实现语言与位置

- **Shell 脚本**，安装到 `/usr/local/bin/bupt-ec`，权限 `0755`，属主 `root:root`
- 不与服务二进制冲突：PATH 里的 `bupt-ec` 是 CLI，systemd 的 `ExecStart` 是绝对路径 `/opt/bupt-ec/bupt-ec`
- 目标规模约 250 行；一切重逻辑委托给 `install.sh`，CLI 只做分派与展示

### R3 分发与安装

- CLI 源文件位于 `scripts/bupt-ec-cli.sh`
- 随 tarball 分发：`release.yml` 的 Compose 步骤把它复制进 `bupt-ec-linux-${arch}/bupt-ec-cli`
  - **文件名不能叫 `bupt-ec`**：`stage_release` 用 `find -name bupt-ec` 定位服务二进制，同名会导致提取到错误文件
  - 随 tarball 分发意味着 CLI 天然被 `checksums.txt` 覆盖，比独立资产更安全
- 安装时由 `stage_release` 额外提取到 staging，再由事务安装到 `/usr/local/bin/bupt-ec`

### R4 纳入原子事务

`transaction_targets` 新增第 7 个目标：

```
cli    /usr/local/bin/bupt-ec
```

由此获得：CLI 与服务二进制在同一事务中替换；任一步失败则一并回滚，不会出现「新 CLI + 旧二进制」的错配组合；CLI 的 bug 修复随版本下发。

### R5 update 的自举

`bupt-ec update` 必须使用**新版本的** `install.sh`，不能用本地旧脚本——新版本可能修复了升级逻辑本身。流程：

1. 读 `/etc/bupt-ec/bupt-ec.env` 取 `RELEASE_REPO` / `RELEASE_VERSION` / `DOWNLOAD_BASE_URL`
2. 目标版本 = 命令行参数 > 已存 `RELEASE_VERSION`
3. 按与 `install.sh` 相同的 URL 规则下载对应版本的 `install.sh` 到临时目录
4. 执行 `bash <新install.sh> --mode=update`，`VERSION` 经环境变量传入

信任模型与现状 `curl | bash` 完全一致（HTTPS + 官方 GitHub 或运维自选镜像），真正的产物 tarball 仍由 `install.sh` 做 checksum 校验。**不降低现有安全性**。

### R6 权限模型

| 子命令 | 需要 root |
|---|---|
| `update` `config` `restart` `start` `stop` | 是 |
| `status` `version` `health` `logs` | 否 |

需要 root 的子命令检查 `EUID`，不足时明确报错并给出 `sudo bupt-ec <cmd>` 提示，而不是让底层命令抛出晦涩的权限错误。

### R7 版本语义要分清

`RELEASE_VERSION` 存的可能是频道名（`latest`）而非具体版本。因此 `version` / `status` 必须区分两个概念：

```
配置频道:   latest              ← env 里的 RELEASE_VERSION
运行版本:   v0.3.0              ← /readyz 的 version 字段
```

混为一谈会让用户以为自己跑在名为 "latest" 的版本上。

### R8 Spec 契约修订（**存在直接冲突，必须显式处理**）

`.trellis/spec/backend/quality-guidelines.md` 的 **Scenario: Transactional Installer Commit and Rollback** 末尾写着：

> Release archives and top-level assets remain self-contained;
> **no runtime helper beside `install.sh` is allowed.**

本任务向 tarball 加入 `bupt-ec-cli` 并安装到 `/usr/local/bin/bupt-ec`，**字面上违反这条契约**。同样的表述也出现在 `docs/release.md`：「`install.sh` is intentionally a self-contained release asset... do not add runtime helper files to the published layout」。

**处理方式：显式修订，而非绕过或忽略。**

该约束的原意是保证 `install.sh` **自身**自包含——它通过 `curl | bash` 单文件管道执行，若运行时依赖同目录的其他脚本，管道就会断。CLI 在这一点上不冲突：`install.sh` 既不 source 也不调用它；它是被事务安装到系统的**独立产物**，地位等同服务二进制。

因此修订为区分两类文件：

| 类别 | 约束 |
|---|---|
| `install.sh` 的运行时依赖 | 仍然禁止——`install.sh` 必须保持单文件可执行 |
| 随包分发、由事务安装到系统的产物 | 允许（服务二进制、CLI） |

同时需更新：

- **Contracts 阶段 4** 列举的快照目标（binary、env、systemd unit + link、Nginx site + link）加入 CLI
- **Contracts 阶段 3/5** 涉及 staging 与原子替换的描述覆盖 CLI 的 mode/owner
- `docs/release.md` 的资产布局清单与自包含表述同步

**若不修订 spec 就实现本任务**，未来的会话会读到一条与代码矛盾的契约，并可能据此「修复」掉 CLI。

### R9 非目标

- **不做 `uninstall`**：它是纯新增的破坏性操作，需要自己的事务设计与二次确认流程（删不删 env？删不删日志？），值得单独立项，不塞进本次范围
- 不做 `bupt-ec version` 的联网上游检查：会让每次查看都变慢；`update` 自己下载时天然知道
- 不把 CLI 改写成 Go 子命令（父任务已定论）
- 不在 CLI 中重新实现下载、校验、事务、回滚——全部委托 `install.sh`

## Acceptance Criteria

- [ ] 安装后 `bupt-ec -h` 可用，列出全部子命令
- [ ] 无参数运行等价于 `-h`，退出码为 0
- [ ] `bupt-ec update` 在已安装机器上零 prompt 完成升级
- [ ] `bupt-ec update v0.2.0` 可回滚到指定版本
- [ ] `bupt-ec status` 一屏显示：服务活跃/启用状态、运行版本、健康结果
- [ ] `bupt-ec version` 明确区分「配置频道」与「运行版本」
- [ ] `bupt-ec config show` 输出中密码与 token 被脱敏
- [ ] 非 root 执行 `update` / `config` / `restart` 等时给出 sudo 指引而非底层错误
- [ ] CLI 与服务二进制在同一事务中替换；事务失败后 `/usr/local/bin/bupt-ec` 回滚到旧版本
- [ ] 首装失败时不残留 `/usr/local/bin/bupt-ec`
- [ ] tarball 内 CLI 文件名为 `bupt-ec-cli`，`stage_release` 仍能正确定位服务二进制
- [ ] `install_test.sh` 覆盖：CLI 安装、CLI 回滚、首装失败清理
- [ ] `shellcheck scripts/*.sh` 全绿（含新脚本）
- [ ] `.trellis/spec/backend/quality-guidelines.md` 的事务契约已按 R8 修订：自包含约束区分「install.sh 的运行时依赖」与「随包分发的独立产物」，快照目标清单含 CLI
- [ ] `docs/release.md` 的资产布局与自包含表述已同步
- [ ] `README.md`、`docs/operations.md`、`docs/upgrading.md` 以 `bupt-ec` 命令为主推路径
- [ ] CHANGELOG `[Unreleased]` 的 `Added` 记录 CLI

## Notes

- 路径常量（`/opt/bupt-ec`、`/etc/bupt-ec/bupt-ec.env`、`bupt-ec.service`）在 CLI 与 `install.sh` 中各存一份，有漂移风险。缓解：CLI 只硬编码路径，一切可变配置从 env 读取；在两个文件中互相注明「修改时需同步」
- 本任务是四个子任务中唯一新增用户可见命令面的，文档改动量最大
