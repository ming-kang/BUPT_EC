# install.sh 拆分安装/升级/重配置三种模式

父任务：`08-22-ops-experience`

## Goal

让升级路径不再需要重答一遍安装问答。把 `install.sh` 从「一个交互式安装脚本」演进为「一个可被非交互调用的部署引擎」，同时**事务内核一行不动**。

## 背景：痛点与约束

**痛点**：今天 `main()` 是一条线性全交互流程。用户只想换个二进制，却要重新回答 GitHub 仓库、域名、证书路径、私钥路径、JW token、用户名、密码、监听地址共 8 项问答（即便每项都按回车保留默认值）。

**核心约束**：`install.sh`（约 1079 行）的事务内核 —— `snapshot_installation` / `atomic_install_file` / `atomic_install_symlink` / `restore_snapshot_target` / `rollback_installation` / `commit_installation` / `installer_cleanup` —— 由 `install_test.sh`（约 1000 行）覆盖 checksum 失败、归档损坏、升级回滚、首装清理、权限、成功提交等真正会咬人的路径。**这是项目最有价值的测试资产，本任务不得改动其行为。**

因此：**演进入口分层，不重写内核**。

## Requirements

### R1 三种模式

```
install.sh --mode=install       首装，全交互（默认值，兼容今天的 curl | bash）
install.sh --mode=update        非交互，配置全部取自已存 env，只换版本
install.sh --mode=reconfigure   交互改配置，版本沿用已存 RELEASE_VERSION
```

- 不传 `--mode` 时等价于 `--mode=install`，**现有 `curl -fsSL ... | sudo VERSION=latest bash` 行为逐字节不变**
- 参数经 `main "$@"` 传入（现已接受 `$@` 但未解析）
- 非法 `--mode` 值快速失败并打印可用值

### R2 阶段抽取

把 `main()` 中从 `repo="${REPO:-...}"` 到 `download_base_url="${DOWNLOAD_BASE_URL:-...}"` 的整段问答抽成 `collect_config_interactive()`，通过全局变量或有序输出回传结果。抽取必须是**纯搬移**：问答顺序、默认值来源优先级（`ENV 变量 > CURRENT_* > 硬编码默认`）、token/password 的条件必填逻辑全部保持原样。

抽取后 `main()` 的形状：

```
require_installer_environment
load_current_config
parse_mode "$@"
case mode in
  install|reconfigure) collect_config_interactive ;;
  update)              adopt_current_config ;;      # 新增，不问任何问题
esac
validate_all                                        # 三模式共用
execute_deployment                                  # 三模式共用，内核零改动
```

### R3 update 模式的具体行为

| 环节 | update 模式行为 | 理由 |
|---|---|---|
| 问答 | **完全跳过**，零 prompt | 核心目标；副产品是可在 cron/脚本中无 tty 运行 |
| 版本解析 | 显式 `VERSION` 优先，否则沿用 `RELEASE_VERSION` | 支持 `bupt-ec update v0.2.0` 回滚 |
| 配置来源 | 全部取自 `load_current_config` 的 `CURRENT_*` | env 是唯一真相源 |
| 配置缺失 | **快速失败**，提示改跑 `--mode=reconfigure` | 防御 env 被手工改坏 |
| `install_packages` | **跳过** | `apt-get update` 是升级路径上最慢的一步；升级不改变依赖集。若 `curl`/`tar` 缺失，在后续步骤暴露并给出明确指引 |
| `create_user` | 保留（幂等） | 成本近零，防御用户被误删 |
| SSL 文件存在检查 | 保留 | nginx 配置引用它们，缺失必须在事务前拦截 |
| 下载/暂存/事务 | 与 install 完全相同 | 单一事务路径，测试资产继续有效 |

### R4 reconfigure 模式的具体行为

- 走完整交互问答（与 install 相同）
- 版本**不提升**，沿用 `RELEASE_VERSION`；但仍**完整下载并走同一条事务路径**
  - 理由：保持事务路径单一。若为「只换配置不换二进制」新增分支，就要在 `prepare_staging` 里特殊处理 binary 来源，凭空多出一条未被测试覆盖的路径
  - 代价：重新下载一次，reconfigure 是低频操作，可接受

### R5 Spec 契约同步（必须，易漏）

`.trellis/spec/backend/quality-guidelines.md` 的 **Scenario: Transactional Installer Commit and Rollback** 需要同步：

- **Signatures 段的行号会失效**：该段以「Common Mistake (positional parameters)」注明 `render_env_file`（install.sh:607, 11 args）与 `prepare_staging`（install.sh:749, 13 args）的**精确行号与参数个数**。模式拆分会插入新函数并整体下移行号，必须更新为改动后的实际值。
- **Contracts 阶段 1**「Validate input, certificates, release selection, and platform prerequisites」需补充说明：`--mode=update` 有意跳过 `install_packages`（platform prerequisites），改为对 `curl` / `tar` 的可用性检查。
- 若 `prepare_staging` 的参数个数因抽取而变化，必须按该段警告同步更新签名、全部调用点与 `install_test.sh` 的 fixture 参数——**本设计有意保持其签名不变以规避此风险**。

同时检查 **Scenario: Installer Release Selection** 的 Signatures 段是否需要补充模式相关入口。

### R6 前置依赖与不做的事

- **依赖 `08-22-drop-nightly` 先完成**：否则版本解析逻辑要改两遍
- 不改动事务内核任何函数的签名与行为
- 不改动 `render_env_file` / `render_systemd_service` / `render_nginx_site` 的输出格式
- 不引入 bats（沿用既有 backlog 判定）
- 不在本任务内新增 `bupt-ec` CLI（那是 `08-22-bupt-ec-cli`）

## Acceptance Criteria

- [ ] 不传 `--mode` 的行为与改动前逐字节一致（首装与升级两条路径均验证）
- [ ] `--mode=update` 在已安装机器上零 prompt 完成版本切换
- [ ] `--mode=update` 可在**无 tty**环境下成功运行（`< /dev/null` 验证）
- [ ] `--mode=update` 在 env 缺关键项时快速失败，并明确指引改用 `--mode=reconfigure`
- [ ] `--mode=update` 不执行 `apt-get update`
- [ ] `--mode=reconfigure` 可修改域名/证书/凭据而不改变版本
- [ ] 非法 `--mode` 值快速失败并列出可用值
- [ ] **既有 `install_test.sh` 断言零修改即全绿**（内核未动的直接证据）
- [ ] 新增测试覆盖：三种模式的分派、update 的非交互性、update 的配置缺失失败、无 tty 运行
- [ ] `shellcheck scripts/*.sh` 全绿
- [ ] `.trellis/spec/backend/quality-guidelines.md` 的事务契约按 R5 更新完毕（行号、参数个数、update 模式的 prerequisites 例外）
- [ ] `docs/upgrading.md`、`docs/deployment.md` 记录新模式

## Notes

- 本任务是 `08-22-bupt-ec-cli` 的地基：CLI 的 `update` 子命令直接调用 `--mode=update`
- 「既有测试零修改即全绿」是本任务最重要的验收信号。若不得不改既有断言，说明抽取不是纯搬移，需回到设计重新审视
