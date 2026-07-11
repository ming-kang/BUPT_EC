# 自适应刷新退避抖动与确定性测试设计

## Policy Boundary

将 base ladder 和 jitter 映射保持为纯函数：

~~~text
consecutive failures
  → totalFailureBackoffBase
  → one injected sample
  → normalized sample in 0..1
  → symmetric bounded offset
  → effective delay
~~~

建议接口：

~~~go
type RandomSample func() float64

func totalFailureBackoffBase(consecutive int) time.Duration
func jitteredBackoff(base time.Duration, sample float64) time.Duration
~~~

ClassroomServiceOptions 增加可选 BackoffRandom。nil 使用 production random；service
保存 non-nil function。策略内部限制 offset 为正负 min(base/10, 5s)，并保证最终
duration 大于零。

## State Update

finishClassroomRefresh 继续在 refreshMu 下更新：

~~~text
failed:
  consecutiveTotalFailures++
  base = ladder(consecutive)
  delay = jitter(base, BackoffRandom())
  nextRefreshAllowed = Clock.Now + delay

partial:
  failures unchanged
  nextRefreshAllowed = Clock.Now + 30s

full:
  failures = 0
  nextRefreshAllowed = zero
~~~

random sample 只在 total failed 分支读取一次。metrics observation 不得重新采样或
重复计算 deadline。

## Clock Refactor

ClassroomService 保留 clock Clock 字段，增加方法：

~~~go
func (s *ClassroomService) now() time.Time
~~~

方法读取 s.clock.Now 并转换到 businessLocation。删除函数类型字段 now，所有 production
调用点保持 s.now() 形式，测试通过 options.Clock 注入 fake。

TokenManager 继续持有相同 Clock 实例，但其 elapsed 不做业务时区转换。时区只影响日期
和 deadline 表示，不影响 duration。

## Test Clock

测试目录定义 mutex 保护的 fakeClock：

~~~text
Now: RLock and return instant
Set: Lock and replace instant
Advance: Lock and add duration
~~~

同一个 fake 指针注入 ClassroomServiceOptions，并自动传入 TokenManager。测试不得再
通过赋值 svc.now 修改时间。

## Midnight Interaction

backoff 可以跨越上海午夜，这是有意的上游保护。合同是：

- 昨天 cache 在午夜后立即被 getCachedTodayClassroomsAt 拒绝；
- nextAllowed 之前无 cache 请求返回安全错误；
- 到达 nextAllowed 后 request 或 warmup 能启动一次新日 refresh；
- scheduler 不会因一次 suppression 睡到下一天。

## Compatibility and Rollback

- 默认随机源使生产行为增加小幅抖动，但基础阶梯和最大量级不变。
- 若 jitter 需要回滚，可把 sample 固定为 0.5，Clock refactor 和测试仍应保留。
- 不修改 RuntimeMetrics label 或公开 payload。
