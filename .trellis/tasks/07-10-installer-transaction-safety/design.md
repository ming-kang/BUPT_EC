# 安装器事务安全 — Design

## Transaction Phases

### 1. Preflight

- 校验输入、证书路径和版本选择。
- 安装必要系统包、确保 service user 存在。
- 下载 archive/checksums。
- 验证 checksum。
- 解包到 root-only temp dir，定位候选 binary。
- 渲染候选 env/systemd/nginx 文件到 temp dir。

此阶段不写入现有目标文件。

### 2. Snapshot

对以下目标记录“原先存在/不存在”并复制已存在文件：

- `/opt/bupt-ec/bupt-ec`
- `/etc/bupt-ec/bupt-ec.env`
- systemd unit
- nginx available/enabled entry

temp dir 由 `mktemp -d` 创建并保持 root-only。

### 3. Commit

- 将候选文件复制到目标目录内的 `.new` 文件，设置 owner/mode。
- 使用 `mv` 在同一文件系统原子替换。
- `systemctl daemon-reload`。
- `nginx -t`。
- restart service、reload nginx。
- 验证 `systemctl is-active`，并在可安全构造本地 URL 时检查 `/healthz`。

### 4. Rollback

commit 后任何命令失败：

- 恢复原先存在的文件；删除原先不存在但本次创建的文件。
- 再次 daemon-reload/nginx -t。
- 尝试重启旧服务、reload nginx。
- 保留原始错误退出码并打印 rollback 状态，不打印 secrets。

## Function Boundaries

将直接写目标的函数拆成：

- `render_env_file destination ...`
- `render_systemd_service destination`
- `render_nginx_site destination ...`
- `stage_release archive work_dir`
- `snapshot_installation backup_dir`
- `commit_installation staging_dir`
- `rollback_installation backup_dir`

纯渲染和 snapshot/rollback 可在测试临时根目录执行。

## Test Harness

沿用版本策略任务提供的 source-safe installer。测试模式通过显式测试根目录和 mocked PATH 替代 `systemctl`、`nginx`、`curl` 等命令；生产默认路径不能被普通环境变量静默改变。

测试不会访问网络、不会要求 root、不会写真实 `/etc` 或 `/opt`。

## Security

- 环境候选和备份 mode 0600；临时根目录 mode 0700。
- rollback 日志只列文件角色和操作结果，不打印文件内容。
- `SKIP_CHECKSUM=1` 仍只跳过 checksum；其余 staging/transaction preflight 不跳过。
