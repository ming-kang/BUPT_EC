# 安装器晚期回滚与代理超时设计

## Transaction State Model

在现有文件 manifest 之外记录事务前的运行状态：

```text
service_file_present
service_enabled
service_active
nginx_site_present
nginx_enabled
```

文件恢复仍由 manifest 驱动；运行状态恢复由上述 snapshot 驱动，不能仅从“文件是否
存在”推断。

## Rollback Reconciliation

### Upgrade

1. 恢复 binary/env/unit/site/link。
2. `systemctl daemon-reload`。
3. `nginx -t` 后 reload，使旧配置重新生效。
4. 按 snapshot 恢复 service enabled/disabled 和 active/inactive 状态。

### First install

1. 若新 service 可能已启动，先 stop，再删除 unit/enablement 和其他目标。
2. daemon-reload 清除新 unit。
3. 删除新 Nginx site/link 后运行 `nginx -t` 并 reload，撤销已加载配置。
4. 不尝试 restart 不存在的旧 service。

回滚步骤全部显式传播失败；任何恢复失败保留 backup 并输出人工恢复路径。

## Side-effect Tracking

可以采用“记录事务前状态并总是 reconcile”的方式，避免依赖某个 commit 步骤是否
刚好执行过。若实现增加 side-effect flags，它们只用于减少无意义命令，不能替代最终
状态断言。

## Timeout Budget

```text
JW/classroom refresh context: 30s
Go HTTP WriteTimeout:          45s
Nginx /api proxy_read_timeout: 60s
```

60 秒为代理返回 JSON 错误或 stale 数据留下传输余量，同时仍是有界等待。普通 `/`
location 可保持 30 秒。

## Compatibility and Recovery

- 成功安装命令顺序保持不变。
- manifest 格式若扩展，rollback 只消费本次运行生成的格式，不需要兼容历史磁盘文件。
- 若新状态恢复逻辑出错，保留现有 root-only backup 作为最终人工恢复路径。
