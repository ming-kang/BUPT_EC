# 单实例部署与扩展路线

## Goal

明确当前推荐单实例拓扑、进程内缓存限制和未来共享缓存或 leader/fetcher 扩展边界。

## Background

- `docs/operations.md:103-109` 只说明 cache 与 warmup 是 process-local、多实例不共享，
  但没有明确当前是否推荐水平扩容或如何避免重复 JW 调用。
- 每个进程独立持有 token、API URL、refresh singleflight/backoff、runtime status 和
  TodayClassrooms cache；多实例会重复 login/warmup/refresh，且 readiness/错误状态不一致。
- installer/systemd/Nginx 当前部署模型自然是单 host、单 `bupt-ec.service` 实例。
- 项目没有本地 timetable DB，也不应在文档任务中暗示已经支持 Redis/leader election。

## Requirements

### R1 — 明确当前支持和推荐的拓扑

- 生产推荐为一个 `bupt-ec` 进程、一个 Nginx reverse proxy、loopback APP_ADDR。
- 明确 restart 会丢失 cache，多个实例不会共享 token/cache/backoff/readiness。
- 说明在现状下直接 round-robin 多实例会增加 JW 压力并产生短暂响应差异，因此不是推荐
  的高可用方案。

### R2 — 给出容量和故障边界

- 描述当前扩展方式：单实例内并发请求由 refresh singleflight 合并，静态前端由同一 Go
  服务提供，Nginx 做 TLS/rate limit。
- 列出单实例故障、重启 warmup、JW 上游故障和 process-local cache 的运维影响。
- 给出可以观察的 readiness/log/未来 metrics 信号，不承诺不存在的 SLA。

### R3 — 定义未来扩展选项而不实施

- 方案 A：共享 typed cache + distributed refresh lock。
- 方案 B：单 leader/fetcher 写共享 snapshot，多只读 API 实例消费。
- 方案 C：保持单实例，通过 systemd restart/备机切换实现简单恢复。
- 对每个方案说明一致性、复杂度、秘密/token ownership、JW 压力和迁移前提。

### R4 — 文档一致性

- 更新 README、deployment、operations、development、AGENTS 和 runtime/directory specs。
- 使用明确的“当前支持 / 不支持 / 未来候选”措辞。
- 本任务不修改代码、installer、配置或 release assets；通常不需要用户 changelog。

## Acceptance Criteria

- [ ] 文档明确推荐当前单实例拓扑及其原因。
- [ ] 操作者能理解 restart、多实例和 JW outage 对 cache/readiness 的影响。
- [ ] 文档明确禁止把当前 process-local singleflight 当作跨实例协调。
- [ ] 至少比较共享 cache+lock、leader/fetcher 和维持单实例三个路线。
- [ ] 未来路线说明 token、安全、失败恢复和数据一致性边界。
- [ ] 所有文档互相一致，不声称 Redis、leader election 或 HA 已实现。
- [ ] Markdown links、命令示例和 `git diff --check` 通过。

## Out of Scope

- 实现 Redis、数据库、消息队列、leader election 或 Kubernetes。
- 给出未经容量测试支持的 QPS/SLA 数字。
- 修改当前 installer/systemd/Nginx 行为。
