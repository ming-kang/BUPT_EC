# 安装器事务安全 — Implementation Plan

## Dependencies

先完成 `07-10-installer-version-policy`，复用 source-safe installer 和脚本测试入口。

## Implementation

- [ ] 搜索 install.sh 所有直接写 `/etc`、`/opt`、systemd、nginx 的调用点。
- [ ] 将 download/verify/stage 移到任何配置写入之前。
- [ ] 将 env/systemd/nginx 生成重构为 destination 参数的 render 函数。
- [ ] 实现 snapshot manifest，记录目标原先存在状态和备份路径。
- [ ] 使用目标目录 `.new` + atomic rename 提交文件。
- [ ] 为 commit 安装 ERR/EXIT rollback trap，并防止 rollback 递归触发。
- [ ] 成功验证后禁用 rollback、清理备份并打印成功。
- [ ] 扩展 `scripts/install_test.sh`，用临时根和 mocked commands 测试 preflight/commit/rollback。
- [ ] 更新 CI shell tests、部署/升级/运维/发布文档和 changelog。

## Focused Tests

- [ ] checksum missing/mismatch leaves targets unchanged。
- [ ] malformed archive leaves targets unchanged。
- [ ] render failure leaves targets unchanged。
- [ ] nginx -t failure restores all target files。
- [ ] service restart/health failure restores old binary and env。
- [ ] successful commit replaces all targets and removes backups。
- [ ] first install rollback removes newly created files。
- [ ] env candidate/installed/backup permissions do not broaden secret access。

## Validation

```powershell
bash -n scripts/install.sh scripts/install_test.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh
git diff --check
```

同时审查 `.github/workflows/release.yml`，确认归档仍只依赖单个 `install.sh`，没有遗漏新的运行时辅助文件。

## Rollback Point

该子任务本身修改 installer 的 rollback 路径，风险最高。提交前保存原脚本行为 diff；若无法证明首次安装和升级两种 rollback 均正确，不启动该任务实现。代码回滚必须保持版本策略任务的文档和 RELEASE_VERSION 修复。
