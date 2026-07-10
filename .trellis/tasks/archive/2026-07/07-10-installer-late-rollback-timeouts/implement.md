# 安装器晚期回滚与代理超时实施计划

## 1. State baseline

- [ ] 列出当前 transaction targets、manifest 字段和 systemctl/Nginx 命令顺序。
- [ ] 为 mock systemctl 增加 active/enabled 状态查询与状态变更记录能力。

## 2. Snapshot and rollback

- [ ] snapshot service active/enabled 和 Nginx enablement 状态，权限保持 root-only。
- [ ] 将 rollback 分为文件恢复和运行状态 reconcile 两阶段。
- [ ] 首次安装回滚停止新 service、撤销 enablement、daemon-reload，并 reload Nginx。
- [ ] 升级回滚精确恢复旧 active/inactive 与 enabled/disabled 状态。
- [ ] 每个命令失败都显式累计 rollback failure 并保留恢复目录。

## 3. Timeout rendering

- [ ] 将 `/api/` `proxy_read_timeout` 提升到 60 秒。
- [ ] 保持 SPA location 的超时策略，补充渲染断言。
- [ ] 同步 deployment、operations、upgrading、release、AGENTS、spec 和 changelog。

## 4. Regression tests

- [ ] 首次安装 restart failure。
- [ ] 首次安装 is-active failure。
- [ ] 首次安装 Nginx reload failure。
- [ ] 首次安装 health failure（service 已启动、Nginx 已 reload）。
- [ ] 旧服务原为 inactive/disabled 的升级失败恢复。
- [ ] rollback stop/reload 自身失败保留 recovery backup。
- [ ] 成功安装和已有升级失败矩阵继续通过。

## Validation

```bash
bash -n scripts/install.sh scripts/install_test.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh
rg -n "proxy_read_timeout|ClassroomRefreshLimit|WriteTimeout" scripts docs AGENTS.md
git diff --check
```

## Review Gates

- 不能只验证文件消失；必须验证实际 service/Nginx 状态。
- 首次安装与升级必须分别覆盖早期和晚期失败。
- 任何 success 输出只能出现在 backup 清理之后。
- release asset 名称、env mode `0600` 和 backup mode `0700` 不变。

## Rollback Point

状态 snapshot/reconcile、mock 测试和超时渲染作为一个提交批次回滚；若测试暴露事务
模型缺陷，回到 design 而不是增加未记录的特殊分支。
