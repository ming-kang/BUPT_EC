# 设计：install.sh 模式拆分

任务：`08-22-installer-modes`

## 设计目标与边界

**目标**：在 `install.sh` 的入口处引入模式分层，使升级路径可以完全非交互执行。

**硬边界**：以下函数**不得修改**（行为、签名、输出格式均不变）：

```
事务内核    snapshot_installation, write_runtime_snapshot, read_runtime_snapshot_value,
            atomic_install_file, atomic_install_symlink, restore_snapshot_target,
            rollback_installation, commit_installation, perform_install_transaction,
            installer_cleanup, initialize_installer_session
渲染层      render_env_file, render_systemd_service, render_nginx_site
下载层      download_release, stage_release, resolve_download_base_url,
            validate_download_base_url, normalize_download_base_url
```

这条边界是本设计的基石：`install_test.sh` 的既有断言全部落在这些函数上，它们不动，测试资产就零损耗。

## 当前 main() 的结构

```
require_installer_environment          ← 环境前置
load_current_config                    ← 读 env 到 CURRENT_*
─────────────────────────────────────
repo/version/domain/ssl/token/user/pass/addr/base_url   ← 8 项交互问答（约 45 行）
─────────────────────────────────────
validate_repo/version/domain/path×2/app_addr/base_url   ← 校验
JW 凭据组合校验（token 为空时 user+pass 必填）           ← 校验，但目前混在问答段
ssl_cert/ssl_key 文件存在检查                            ← 校验
─────────────────────────────────────
detect_arch, mktemp, initialize_installer_session
install_packages, create_user
download_release → prepare_staging → perform_install_transaction
收尾提示
```

问题在于中间那段问答与前后两段是硬编织的，无法单独跳过。

## 目标结构

```
main() {
  require_installer_environment
  load_current_config
  parse_mode "$@"                    # 新增：设置 INSTALLER_MODE
  case "${INSTALLER_MODE}" in
    install|reconfigure) collect_config_interactive ;;   # 抽取，纯搬移
    update)              adopt_current_config ;;          # 新增
  esac
  validate_all                        # 抽取 + 收拢散落的校验
  execute_deployment                  # 抽取，内部逻辑零改动
  print_completion_summary            # 抽取，按模式给不同措辞
}
```

## 契约：配置传递用 `CFG_*` 全局变量

Bash 函数无法返回多值。三种候选中选定全局变量：

| 方案 | 判断 |
|---|---|
| **`CFG_*` 全局变量** ✅ | 与现有 `CURRENT_*` / `VALIDATED_DOWNLOAD_BASE_URL` / `TRANSACTION_*` 风格一致，零新机制 |
| `local -n` nameref | 需 bash 4.3+，且十个 out-param 调用点极丑 |
| 输出 tab 分隔行由调用方 read | 密码/token 可能含制表符或换行，脆弱到不可接受 |

契约变量集（十项，与 `render_env_file` 的入参一一对应）：

```
CFG_REPO  CFG_VERSION  CFG_DOMAIN  CFG_SSL_CERT  CFG_SSL_KEY
CFG_USERNAME  CFG_PASSWORD  CFG_TOKEN  CFG_APP_ADDR  CFG_DOWNLOAD_BASE_URL
```

三个生产者对同一契约赋值，下游完全不感知模式：

- `collect_config_interactive()` —— 交互问答（install / reconfigure）
- `adopt_current_config()` —— 直接从 `CURRENT_*` 复制（update）
- 两者都遵守既有优先级：`显式环境变量 > CURRENT_* > 硬编码默认`

## 数据流

```
                       ┌── install ─────→ collect_config_interactive ──┐
env file               │                                               │
  ↓ load_current_config├── reconfigure ─→ collect_config_interactive ──┼→ CFG_* ─→ validate_all ─→ execute_deployment
CURRENT_*              │                                               │              ↓                  ↓
                       └── update ──────→ adopt_current_config ────────┘         快速失败        既有事务内核
                                                                                                （零改动）
```

## 关键设计决策

### D1 版本解析按模式分叉

```
install      VERSION 或兜底 latest          （drop-nightly 完成后的兜底值）
update       VERSION 或沿用 RELEASE_VERSION  （支持 update v0.2.0 回滚）
reconfigure  沿用 RELEASE_VERSION，忽略 VERSION（语义是「改配置」而非「换版本」）
```

`resolve_release_version` 本身不改，由 `parse_mode` 之后的调用方决定传什么。

### D2 update 跳过 `install_packages`

`install_packages` 执行 `apt-get update` + `apt-get install -y ca-certificates curl tar nginx`，其中 `apt-get update` 是整条升级路径上最慢的一步（常达数十秒）。

升级不改变依赖集，因此跳过。逃生阀：update 模式开始前检查 `curl` 与 `tar` 可用，缺失则明确报错并指引改跑 `--mode=install`。

**权衡**：若未来版本引入新系统依赖，update 不会自动装。这是有意的——新依赖属于需要人工介入的变更，静默 apt 安装反而危险。届时在 CHANGELOG 注明需跑一次完整安装器即可。

### D3 reconfigure 仍完整下载

即便版本没变，reconfigure 也走完整的下载→暂存→事务路径。

**放弃的替代方案**：复用已安装的 `/opt/bupt-ec/bupt-ec` 作为 staging binary。它需要在 `prepare_staging` 里为 binary 来源开一条分支，凭空产生一条 `install_test.sh` 未覆盖的路径——为省一次下载而在事务前置引入未测试分支，不划算。

### D4 校验逻辑集中到 `validate_all`

目前两处校验散落在问答段：

- JW 凭据组合校验（`token` 为空时 `username`+`password` 必填）——夹在问答中间
- `ssl_cert` / `ssl_key` 文件存在检查——在执行段开头

两者都是纯校验，且三种模式都需要，统一移入 `validate_all()`。这不改变任何校验语义，只改变代码位置。**对 update 模式尤其重要**：它不经过问答段，若校验留在原地就会被整段跳过。

### D5 `parse_mode` 的解析范围

只识别 `--mode=<value>` 与 `--mode <value>` 两种形式，其余参数一律拒绝并打印用法。

不引入 getopts：目前只有一个参数，getopts 会带来 `OPTIND` 在 sourced 场景下的状态污染问题（`install_test.sh` 是 source 本脚本运行的）。

## 兼容性

| 调用方式 | 改动前 | 改动后 |
|---|---|---|
| `curl ... \| sudo VERSION=latest bash` | 全交互首装/升级 | **完全不变** |
| `curl ... \| sudo bash` | 全交互，兜底 nightly | 全交互，兜底 latest（由 drop-nightly 带来） |
| `curl ... \| sudo bash -s -- --mode=update` | 参数被忽略 | 非交互升级 |
| `source install.sh`（测试） | `INSTALLER_SOURCED` 拦截 main | **完全不变** |

`INSTALLER_SOURCED` 机制不动，`install_test.sh` 的 source-and-call 测试风格继续有效。

## 回滚形状

两个层次，互不干扰：

- **运行时回滚**：事务失败后的 `rollback_installation` —— 本任务不触碰，行为不变
- **变更回滚**：模式拆分若上线后出问题，`git revert` 单个提交即可；由于是纯结构抽取，无数据迁移、无 env 格式变更、无系统状态残留

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| 抽取时漏搬某项默认值优先级，导致某个 prompt 默认值变了 | 逐项对照原 `main()` 写 diff review；验收要求既有 `install_test.sh` 零修改全绿 |
| update 模式下 env 被手工改坏导致渲染出损坏的 nginx 配置 | `validate_all` 对 update 同样全量执行；事务内核的 `nginx -t` 校验兜底 |
| `CFG_PASSWORD` 等敏感值以全局变量长期驻留 | 与现状同级风险（`CURRENT_JW_PASSWORD` 已如此）；不新增泄露面，env 文件权限仍为 0600 |
| 无 tty 环境下 `prompt` 意外被触发 | update 路径不调用任何 `prompt*` 函数；验收含 `< /dev/null` 运行断言 |
