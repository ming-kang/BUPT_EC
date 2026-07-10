# 安装器晚期回滚与代理超时

## Goal

修复首次安装在服务或 Nginx 已激活后的晚期失败回滚，并使代理超时覆盖后端冷刷新预算。

## Background

- `scripts/install.sh:727-735` 在提交后依次 enable、restart、reload 并进行健康检查。
- `scripts/install.sh:632-674` 只在 snapshot 中存在旧 service/Nginx 文件时恢复运行
  状态；首次安装在 restart/reload 后失败时会删除文件，但不会停止新服务，也不会
  重新加载已删除配置后的 Nginx。
- `scripts/install_test.sh:563-584` 的首次安装回滚只覆盖 `nginx -t` 在 service
  restart 之前失败，未覆盖 active/health 等晚期路径。
- `scripts/install.sh:498-516` 对 `/api/` 与 `/` 使用 30 秒
  `proxy_read_timeout`，等于后端 `ClassroomRefreshLimit`，低于 45 秒 Go 写预算。

## Requirements

### R1 — 精确恢复首次安装运行状态

- snapshot 必须记录旧 service 是否存在、是否 enabled、是否 active，以及旧 Nginx
  site/enablement 是否存在。
- 首次安装在新 service 已启动后失败时，rollback 必须停止新 service、移除新目标、
  daemon-reload，并确保不存在仍运行但 unit 文件已删除的进程。
- 首次安装在 Nginx 已 reload 后失败时，删除新 site/link 后必须重新执行 `nginx -t`
  并 reload，使运行配置与磁盘一致。
- 升级回滚要恢复旧文件和旧 active/enablement 状态；原来 inactive 的服务不能被
  无条件启动。

### R2 — 晚期失败测试矩阵

- 覆盖首次安装的 restart、is-active、Nginx reload 和 health check 失败。
- 每个测试断言文件、symlink、service active/enablement、Nginx reload 次数和恢复
  目录行为，而不仅是退出码。
- 保留升级、snapshot、atomic write、checksum 和 incomplete rollback 现有测试。

### R3 — 超时预算一致

- Nginx `/api/` read timeout 必须严格大于 30 秒刷新上限和 45 秒 Go WriteTimeout，
  推荐固定为 60 秒。
- 普通 SPA/static proxy 可继续使用较短超时，除非统一值能简化且不降低保护。
- 文档必须给出 backend、Go server、Nginx 三层预算关系。

### R4 — 事务合同不回退

- 继续保持 prepare-before-snapshot、root-only staging/backup、same-filesystem atomic
  rename、自动 rollback 和失败时保留恢复文件的现有合同。
- 不改变 release asset 名称、安装路径、systemd 用户或 env 权限。

## Acceptance Criteria

- [ ] 首次安装在 restart、is-active、Nginx reload、health 任一点失败后没有遗留
      运行中的新 service 或已加载的新 Nginx site。
- [ ] 升级失败恢复旧文件、symlink、enablement 和 active/inactive 状态。
- [ ] rollback 自身失败仍保留 mode `0700` backup 和 mode `0600` env 快照。
- [ ] 生成的 `/api/` `proxy_read_timeout` 为 60 秒或其他有文档证明的安全预算。
- [ ] installer 行为测试覆盖所有晚期首次安装路径并通过。
- [ ] `bash -n`、ShellCheck、release asset layout 和 `git diff --check` 通过。
- [ ] 成功安装路径、版本选择和来源信任政策保持不变。

## Out of Scope

- 重写 installer 为其他语言或拆分为多个发布文件。
- 增加容器、Ansible 或 Kubernetes 部署方式。
- 修改后端 30 秒刷新上限或 45 秒 HTTP 写超时。
