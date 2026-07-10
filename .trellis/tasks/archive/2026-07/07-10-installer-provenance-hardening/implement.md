# 安装器下载来源与制品信任实施计划

## 1. Baseline

- [ ] 固定当前 GitHub、代理、显式镜像和保存镜像测试基线。
- [ ] 搜索 README/docs/spec 中所有 `gh-v6.com`、fallback 和 installer pipe 命令。

## 2. Source selection

- [ ] 删除自动第三方代理常量和 reachability fallback 分支。
- [ ] 保留官方 GitHub URL 与显式 `DOWNLOAD_BASE_URL` 两条来源路径。
- [ ] 确保来源解析失败发生在下载、snapshot 和系统变更之前。
- [ ] 为显式镜像输出清楚的信任边界提示，且不打印凭据或敏感 URL 参数。

## 3. Integrity checks

- [ ] 保持包与 `checksums.txt` 下载、条目选择和 SHA-256 fail-closed 顺序。
- [ ] 验证 `SKIP_CHECKSUM=1` 仍需显式设置并输出强警告。
- [ ] 确认 release asset 名称和单文件 installer 布局无变化。

## 4. Tests and docs

- [ ] 更新 installer mock 测试：GitHub 失败不再自动代理。
- [ ] 覆盖显式/保存 HTTPS 镜像、非法 URL、HTTP opt-in 和 checksum 失败。
- [ ] 更新 README、deployment、upgrading、operations、release、AGENTS、spec 和
      changelog。

## Validation

```bash
bash -n scripts/install.sh scripts/install_test.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh
rg -n "gh-v6|fallback|DOWNLOAD_BASE_URL" README.md docs scripts .trellis/spec
git diff --check
```

## Review Gates

- 默认路径中不得出现自动选中的第三方域名。
- 所有失败必须在事务前发生。
- 文档必须区分 checksum 完整性与发布者真实性。
- `.agents/`、`.codex/` 和 release asset layout 不得进入变更范围。

## Rollback Point

来源选择、测试与文档作为一个原子提交回滚；不触碰事务实现文件的其他逻辑。
