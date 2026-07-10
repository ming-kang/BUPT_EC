# 单实例部署与扩展路线实施计划

## 1. Evidence inventory

- [ ] 列出 cache、TokenManager、refresh coordinator、warmup、runtime status 的 process-local
      ownership 证据。
- [ ] 核对 installer/systemd/Nginx 当前只创建一个 unit/upstream。
- [ ] 搜索 docs 中所有 “multiple instances/process-local/scale/HA” 表述。

## 2. Current topology docs

- [ ] README 添加当前推荐单实例定位和详细文档链接。
- [ ] deployment 添加 Nginx→loopback→single process 图和安装假设。
- [ ] operations 说明 restart、JW outage、host failure 和多实例差异。
- [ ] development/AGENTS 明确状态 ownership 和不支持的 distributed assumptions。

## 3. Future route comparison

- [ ] 比较 single active、shared cache+lock、leader/fetcher 三种路线。
- [ ] 对每项写出数据一致性、token ownership、failure/fencing、运维成本和迁移前提。
- [ ] 明确没有实现 Redis/leader/HA，不提供误导配置示例。

## 4. Spec sync

- [ ] 更新 runtime-state/cache 和 directory specs 的 deployment boundary。
- [ ] 检查内部链接、术语、APP_ADDR/systemd 路径一致。

## Validation

```bash
rg -n "process-local|multiple instances|single instance|Redis|leader|shared cache" README.md docs AGENTS.md .trellis/spec
git diff --check
```

## Review Gates

- 当前能力和未来候选必须明确分栏。
- 不承诺未经测试的 SLA/QPS。
- 不把 token 放入共享 cache 方案示例。
- 不修改代码、workflow、installer 或 release assets。

## Rollback Point

所有拓扑文档作为一个 docs/spec 提交，可独立回滚。
