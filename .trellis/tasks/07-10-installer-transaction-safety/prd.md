# 安装器事务安全

## Goal

使安装和升级在下载、校验、渲染或服务验证失败时不会留下二进制与配置不一致的半安装状态。

## Requirements

- R1：下载 archive/checksums、checksum 验证、解包和候选文件生成必须在修改现有安装前完成。
- R2：checksum 缺失、条目缺失、hash 不匹配或 archive 缺少二进制时，现有 env、binary、systemd 和 nginx 文件保持字节不变。
- R3：候选 env 文件必须保持 mode 0600、root-owned；临时目录和备份不得向非 root 暴露凭据。
- R4：二进制和配置替换必须使用同文件系统临时文件加原子 rename，避免截断文件。
- R5：事务开始时保存现有 binary/env/systemd/nginx 状态；commit 后 `nginx -t`、systemd reload/restart 或健康验证失败时恢复旧状态。
- R6：首次安装没有旧文件时，rollback 必须删除本次新建的候选安装文件，而不是留下不可运行配置。
- R7：release archive 名称、目录布局、systemd hardening 和 nginx security 配置保持不变。
- R8：installer 必须在最终成功后才打印 installed/success；失败返回非零。
- R9：事务核心行为需通过使用临时根目录和 mocked system commands 的非破坏性脚本测试。

## Acceptance Criteria

- [x] checksum 下载/验证失败测试证明已有 env 和 binary 未改变。
- [x] archive 缺少 binary 时未进入 commit。
- [x] nginx validation failure 测试恢复旧 binary/config。
- [x] service restart/health failure 测试恢复并尝试重启旧服务。
- [x] 首次安装 commit failure 不留下声称成功的 service/nginx 文件。
- [x] 成功升级测试安装新 binary/config 并清理备份。
- [x] env 权限和 ownership 逻辑保持符合文档。
- [x] bash tests、bash -n、shellcheck 和 release workflow 资产检查通过。
- [x] deployment/upgrading/operations/release 文档和 CHANGELOG 更新。

## Out of Scope

- 不支持非 apt 系发行版。
- 不引入系统快照工具或外部配置管理系统。
- 不改变证书管理责任或自动申请 TLS 证书。
