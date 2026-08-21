# classroom-display-contract 前置研究(2026-08-21)

## F-05 capacity 数据血统

- JW 上游无独立座位数字段;容量嵌在教室 token 尾缀:`CLASSROOMS: "教学实验综合楼-N104(229),..."`
- `service/classroom_builder.go:12` 正则 `^(.+)[(（](\d+)[)）]$`;`:142` Atoi **丢弃错误**
- capacity=0 的两条路径:
  1. 结构性:无 `(N)` 尾缀的 token → capacity=0 + 归入"未分组"楼栋(`:140`)
  2. Atoi 失败(仅溢出)
- 录得载荷仅一份(`jw_protocol_test.go:30` `教学楼-101(40)`),容量为正;所有 fixture 为正
- **结论:0 是否真实出现 = UNKNOWN,需活体探针**(integration test 需真实凭据,.env 已具备)
- 前端影响面:`TodayClassroomTable.tsx:133,229`(`|| "未知"`)+ 测试 `:120-129,276-298`

## F-07 display_name 可行性

- 先例:`parseRoom:144` 计算 `buildingName-roomName` → `RoomInfo.DisplayName`(`:129` 输出)
- 前端别名是纯标签层:value/选择状态/表格过滤始终用原始 name(`BuildingPicker.tsx:38-39`,测试 `:91-93`)
- 待搬移规则:1 条别名(未来学习大楼→主楼)+ 数字名加"教"前缀(`/^\d+$/`)
- 坑:当日缓存存序列化载荷,部署后旧缓存无新字段 → 前端须回退 name

## B-04 api-etag-preserialize(顺带研究,维持挂起)

两个推翻审计假设的发现:
1. **log_id 使 marshal+hash ETag 命中率为 0%**(B-11 后每请求体都不同)
2. **(stale, errKind) 键会发过期数据**(每次刷新 Store 新快照,键必须含快照标识 × 变体,约 5 种)

收益重估:Nginx 30 req/min/IP 下 CPU 节省为噪声;唯一收益是 304 省带宽。技术可行性已确认(gzhttp 不破坏 ETag、304 只写头不触发压缩、X-Log-Id 在 handler 前注入)。若将来做:方案 A=Store 时按快照×变体预序列化 data 段+拼接新鲜 log_id 信封(不动契约);方案 B=A+log_id 移出成功体+弱 ETag+304(部分逆转 B-11)。挂起至流量驱动。
