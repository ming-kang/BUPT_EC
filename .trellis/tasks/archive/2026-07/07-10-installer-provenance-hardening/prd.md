# 安装器下载来源与制品信任

## Goal

移除安装器对第三方代理的隐式信任，建立显式下载来源、HTTPS 与制品真实性验证合同。

## Background

- `scripts/install.sh:5-7` 同时内置 GitHub 与 `gh-v6.com` 主机。
- `scripts/install.sh:297-318` 在 GitHub 探测失败后自动切到第三方代理，用户
  没有做出显式信任决定。
- `scripts/install.sh:340-349` 从同一个下载基址获取包与 `checksums.txt`；这能
  检查传输完整性，但第三方同时提供二者时不能证明 GitHub 发布者身份。
- `docs/deployment.md:104-116` 当前把代理描述为自动 fallback，并给出直接通过
  代理执行 root 安装器的命令。
- 已完成的版本选择、checksum fail-closed、事务安装和固定 release asset 布局
  必须保持。

## Requirements

### R1 — 默认来源只能是 GitHub

- 未设置显式下载覆盖时，只允许从官方 GitHub release URL 下载。
- GitHub 不可达时必须安全失败并给出可操作提示，不能静默切换第三方来源。
- 删除或停用 `GITHUB_IPV6_PROXY_HOST` 自动探测分支。

### R2 — 镜像必须由操作员显式信任

- 继续支持显式 `DOWNLOAD_BASE_URL`，并保留 HTTPS 默认要求及
  `ALLOW_INSECURE_DOWNLOAD_BASE_URL=true` 的本地 break-glass 语义。
- 使用镜像时，包与 checksum 的来源、信任边界和风险必须在输出及文档中明确；
  同源 checksum 只能声明完整性，不能被描述为独立发布者认证。
- 已保存的 `DOWNLOAD_BASE_URL` 可以继续用于升级，但必须来自先前的显式配置，
  不能由网络探测自动写入。

### R3 — 下载与验证仍然 fail closed

- 包下载、checksum 下载、条目查找或哈希校验任一失败都不得进入 snapshot/commit。
- `SKIP_CHECKSUM=1` 仍是唯一显式跳过哈希验证的 break-glass 开关，并应输出强警告。
- 不在本任务中新增依赖外部密钥服务或改变 release asset 名称。

### R4 — 测试和文档同步

- 安装器行为测试覆盖 GitHub 不可达、显式 HTTPS 镜像、保存镜像、非法 URL、
  insecure opt-in 和 checksum 失败。
- 更新 README、deployment/upgrading/operations/release 文档和可执行 Trellis spec。
- 所有对用户可见的来源政策变化写入 `CHANGELOG.md`。

## Acceptance Criteria

- [ ] GitHub 探测失败不会自动使用 `gh-v6.com` 或任何第三方主机。
- [ ] 未显式配置镜像时，失败发生在下载/预检阶段且安装目标保持字节不变。
- [ ] 显式 HTTPS `DOWNLOAD_BASE_URL` 仍可完成下载与 checksum 验证。
- [ ] HTTP 镜像只有在显式 insecure opt-in 下才能使用。
- [ ] 文档不再提供隐式代理信任或从未知来源直接 `sudo bash` 的推荐命令。
- [ ] `bash scripts/install_test.sh`、`bash -n scripts/*.sh`、ShellCheck 和 release
      asset layout 检查通过。
- [ ] 安装事务、版本选择优先级和 release asset 名称没有变化。

## Out of Scope

- 建立 cosign/minisign/GPG 发布签名基础设施。
- 运行自建镜像、CDN 或代理服务。
- 修改安装事务、systemd/Nginx 回滚或应用运行时行为。
