# 执行计划：bupt-ec 运维 CLI

任务：`08-22-bupt-ec-cli`
前置：`08-22-installer-modes` 必须先归档（`update` / `config` 依赖 `--mode` 分派）

## 执行顺序

### 步骤 1：CLI 骨架与帮助

- [ ] 新建 `scripts/bupt-ec-cli.sh`，`set -euo pipefail`
- [ ] 路径常量（`INSTALL_DIR` / `CONFIG_DIR` / `ENV_FILE` / `SERVICE_NAME`），注明「与 install.sh 同步」
- [ ] `usage()`：列出全部子命令，含一行示例
- [ ] 子命令分派 `case`，未知命令报错并显示 usage，退出码 1
- [ ] 无参数 / `-h` / `--help` → usage，退出码 0
- [ ] `CLI_VERSION="dev"` 占位符，供 release 时替换

**验证**：`shellcheck scripts/bupt-ec-cli.sh` 全绿；本地跑 `bash scripts/bupt-ec-cli.sh -h` 输出正确

### 步骤 2：只读子命令

先做不需要 root、不改系统状态的部分，便于早期验证。

- [ ] `read_env_value KEY`：从 env 安全读取单个键（不 source 整个文件，避免污染 CLI 进程）
- [ ] `health`：探 `/healthz` 与 `/readyz`，地址取自 `APP_ADDR`，缺失回退 `127.0.0.1:8080`
- [ ] `version`：三行输出，区分「配置频道 / 运行版本 / CLI」（design D6）
- [ ] `status`：`systemctl is-active` + `is-enabled` + 运行版本 + 健康摘要，一屏
- [ ] `logs [-f] [-n N]`：`journalctl -u bupt-ec` 包装，默认 `-n 50`

**验证**：`shellcheck` 全绿；env 缺失、服务未运行、`/readyz` 返回 503 等降级路径均给出可读输出而非报错堆栈

### 步骤 3：权限检查与服务控制

- [ ] `require_root <cmd>`：`EUID != 0` 时打印 `sudo bupt-ec <cmd>` 指引并退出
- [ ] `start` / `stop` / `restart`：`systemctl` 包装，前置 `require_root`
- [ ] 操作后回显新状态，避免用户再敲一次 `status`

**验证**：非 root 执行给出指引而非底层权限错误

### 步骤 4：update 与 config 的自举

- [ ] `resolve_installer_url`：复刻 `install.sh` 的 URL 规则（latest / vX.Y.Z / 镜像），注释指向 `install.sh:resolve_download_base_url`
- [ ] `fetch_installer`：下载到 `mktemp -d` 的临时目录，`trap` 清理
- [ ] `update [VERSION]`：目标版本 = 参数 > env `RELEASE_VERSION`；执行 `VERSION=<目标> bash <新脚本> --mode=update`
- [ ] `config`：执行 `bash <新脚本> --mode=reconfigure`
- [ ] `config show`：打印 env，**密码与 token 脱敏**（显示为 `***` 或长度占位）
- [ ] 两者前置 `require_root`

**验证**：`shellcheck` 全绿；脱敏在测试中断言（防止密码进日志或截图）

### 步骤 5：接入分发链路

- [ ] `release.yml` 的 Compose 步骤：把 `scripts/bupt-ec-cli.sh` 复制为 `bupt-ec-linux-${arch}/bupt-ec-cli`，`chmod +x`
- [ ] 同一步骤中把 `CLI_VERSION` 占位符替换为构建版本（与 `-ldflags` 注入同源）
- [ ] 确认 CLI 随 tarball 进入 `checksums.txt` 覆盖范围（它是 tarball 内容，天然覆盖）
- [ ] `docs/release.md` 的资产布局清单加入 `bupt-ec-cli`

**验证**：本地模拟 Compose 步骤，解压 tarball 确认布局

### 步骤 6：接入安装事务

- [ ] `stage_release` 扩展：`find -name bupt-ec-cli` → `install -m 0755` 到 `${staging_dir}/bupt-ec-cli`，属主 `root:root`；缺失时明确报错（design D2）
- [ ] `transaction_targets` 新增 `cli /usr/local/bin/bupt-ec`（design D3）
- [ ] `commit_installation` 中安装 CLI（复用 `atomic_install_file`，不写专用逻辑）
- [ ] 确认 `restore_snapshot_target` 对「首装失败需删除新文件」的语义已覆盖 CLI

**评审关卡**：确认没有为 CLI 编写任何专用的快照或回滚代码——若写了，说明没有正确复用 `transaction_targets` 的迭代机制。

**验证**：
```bash
bash scripts/install_test.sh
shellcheck scripts/*.sh
```

### 步骤 7：测试补充

- [ ] `stage_release` 提取 CLI 成功 / CLI 缺失时报错
- [ ] 解压后服务二进制不是 shell 脚本（防 D1 回归，断言 ELF 魔数或非文本）
- [ ] 升级路径：CLI 被原子替换
- [ ] 事务失败：CLI 回滚到旧内容
- [ ] 首装失败：`/usr/local/bin/bupt-ec` 不残留
- [ ] CLI 文件权限 `0755`、属主 `root:root`
- [ ] `config show` 脱敏断言

### 步骤 8：文档

- [ ] `README.md`：「Deploy to a server」后增加 CLI 简介；升级改推 `bupt-ec update`
- [ ] `docs/operations.md`：日常运维以 CLI 为主，保留 `systemctl` / `journalctl` 原始命令作为参考
- [ ] `docs/upgrading.md`：主推 `sudo bupt-ec update`，保留 `curl | bash` 作为兜底与首装路径
- [ ] `docs/deployment.md`：安装完成后的验证步骤改用 `bupt-ec status`
- [ ] `CHANGELOG.md` `[Unreleased]` 的 `Added` 记录 CLI

## 全量验证命令

```bash
shellcheck scripts/*.sh
bash scripts/install_test.sh
task check
task test
```

## 回滚点

| 步骤 | 回滚方式 |
|---|---|
| 步骤 1–4 后 | CLI 尚未进入分发链路，revert 无任何系统影响 |
| 步骤 5 后 | revert Compose 改动；tarball 恢复原布局 |
| 步骤 6 后 | **风险最高点**：revert 时需同时处理已安装机器上的 `/usr/local/bin/bupt-ec` 遗留文件，否则留下孤儿命令（design 回滚形状表） |
| 全部完成后 | 同上 |

## 完成信号

- `bupt-ec update` 在真实已安装机器上零 prompt 完成升级，且失败可回滚
- 没有为 CLI 编写任何专用的下载、校验、事务或回滚逻辑
- `install_test.sh` 既有断言仍全绿，新增断言覆盖 CLI 的安装与回滚
