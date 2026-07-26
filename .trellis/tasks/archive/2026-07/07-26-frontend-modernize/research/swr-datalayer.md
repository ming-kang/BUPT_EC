# Research: 数据层 SWR 化（R3/R4/R5）

- **Query**: useTodayClassrooms 手写数据层现状解剖 + SWR 2.x 能力核实 + 语义等价映射 + R4 normalizeResponse/log_id + 风险与逃生方案
- **Scope**: mixed（内部源码/测试/契约 + npm registry / swr 2.4.2 官方 tarball 源码验证）
- **Date**: 2026-07-27
- **外部证据方式说明**: deepwiki MCP 工具本会话不可用；SWR 行为全部以 npm 正式包 `swr@2.4.2` 的 `dist/index/index.mjs` 构建产物逐行核对（比文档更权威），文中 `swr:index.mjs:NNN` 即该文件行号。

---

## 1. 现状解剖

### 1.1 useTodayClassrooms.js（235 行）逐段

**常量与导出纯函数**（这些是可保留资产）：

| 位置 | 内容 |
|---|---|
| `frontend/src/useTodayClassrooms.js:16-17` | `CLIENT_FETCH_TIMEOUT_MS = 40_000`（注释说明预算：> 后端 ClassroomRefreshLimit 30s，< Go WriteTimeout 45s，< Nginx proxy_read_timeout 60s）、`CLIENT_FETCH_TIMEOUT_MESSAGE = "请求超时，请稍后重试"` |
| `:20-25` | `hasUsableClassroomData(resp, nowMs)` = `resp.code === 0 && isUsableBusinessDaySnapshot(resp.data, nowMs)` |
| `:31-36` | `shouldFullPageSpin(isBackground, hasUsableData)`：背景轮询永不全屏 spin；前台（首载/手动重试）仅在无可用数据时 spin |
| `:38-40` | `nextFailureCount(current, succeeded)`：成功清零、失败 +1 |
| `:47-79` | `mergeFetchResult(prev, next, nowMs)`：核心合并逻辑，详见下 |
| `:81-87` | `errorEnvelope(message, code=500)` 私有工具 |
| `:89-94` | `isPageVisible()` 私有工具 |

**mergeFetchResult（:47-79）三分支**：
1. `next` 可用 → 直接替换（:49-51）。
2. `next` 不可用但 `prev` 仍可用（同日且 stale_until 未过）→ 返回 `code:0`，`data = {...prev.data, stale: true, error: {type: "client_refresh_failed", message}}`（:58-71）——这就是"失败保留快照 + stale Alert"的注入点。
3. 两者都不可用 → 硬空 envelope `{code, msg, data: null}`；成功信封但 data 校验失败时 code 强制 500（:73-78）。

**状态机（:96-107）**：6 个 useState + 1 个 ref：
- `reloadRequest {key, background}`（:97-100）——fetch effect 的唯一触发器；
- `spinning`（初值 true）、`reloading`、`failureCount`、`resp`（初值 `loadingResponse`）、`pageVisible`；
- `respRef`（:106-107）**渲染期写入** `respRef.current = resp`——审计已标记的纯度违规。

**visibilitychange effect（:109-118）**：监听 `document.visibilitychange` → `setPageVisible`。

**fetch effect（:120-196）**，deps=[reloadRequest]：
- 手写超时：`new AbortController()` + `setTimeout(() => { timedOut = true; controller.abort(); }, 40_000)`（:121-127）；
- `loadData()` 开头按 `respRef.current` 判断 usable → 设 `spinning`/`reloading`（:130-135）；全屏 spin 且无可用数据时把 resp 重置回 `loadingResponse`（:137-140）；
- `fetch("/api/get_data", { signal, headers: {Accept: "application/json"} })`（:143-146）→ `readJson`（:147）→ `!response.ok` 则 `throw new Error(extractMessage(payload) || \`请求失败 (${status})\`)`（:149-153）——**真实 HTTP status 只进了文案，结构上丢失**；
- 成功路径：`normalizeResponse(payload)`（:155，可能 throw，见 §4）→ `setFailureCount(nextFailureCount)` → `setResp(updater)` 且 **updater 内部写 respRef**（:159-163，第二处纯度违规）；
- catch：`controller.signal.aborted && !timedOut` → 静默 return（卸载/重入取消，:165-167）；否则 `errorEnvelope(timedOut ? 超时文案 : error.message)` → 同样的 merge 流程（:168-181，与 :157-163 逐行重复——审计已点名）；
- finally：清 timer；非取消或已超时才清 spinning/reloading（:182-188）；
- cleanup：clearTimeout + abort（:192-195）——**卸载会 abort 在途 fetch**。

**retry（:198-203）**：bump `reloadRequest`，`background: false`（→ 允许全屏 spin）。

**定时器调度 effect（:205-225）**，deps=[failureCount, pageVisible, reloading, resp, spinning]：
- 隐藏 / spinning / reloading 时不排程（:206-208）——**隐藏即取消 pending timer**（cleanup :224）；
- `delay = nextReloadDelay(resp.data, { failureCount })`（:209），null 则不排程；
- timer 到点：若 `resp.code === 0` 但已不可用（跨日/过 stale_until）→ **先注入** `errorEnvelope("当前缓存已失效，正在重新获取")` 清空 UI（:214-218），再 bump `background: true` 的 reloadRequest（:219-222）。

**返回值（:227-233）**：`{ resp, spinning, reloading, isError: resp.code !== 0 && !spinning, retry }`。

**消费方**：`frontend/src/App.jsx:36` 只解构 `resp/spinning/isError/retry`（`reloading` 仅 lifecycle 测试探针用到）；stale Alert 条件 `resp.code === 0 && (resp.data?.stale || resp.data?.error)`（App.jsx:103-110，文案来自 `classroomWarningMessage`）；`GlobalEmpty todayData={resp}`（App.jsx:136）。

### 1.2 reloadSchedule.js 导出接口与职责（PRD 要求原样保留）

| 导出 | 位置 | 职责 |
|---|---|---|
| `STALE_POLL_MS=15_000` / `PARTIAL_POLL_MS=30_000` / `MIN_FRESH_DELAY_MS=1_000` / `FAILURE_RETRY_DELAYS_MS=[10,20,30,60]s` / `JITTER_RATIO=0.1` / `JITTER_MAX_MS=5_000` | `frontend/src/reloadSchedule.js:4-11` | Nginx 30 req/min 限流感知的调度常量 |
| `failureRetryDelay(failureCount)` | `:13-19` | 退避阶梯，count clamp≥1、封顶 60s |
| `normalizeRandomSample(random)` | `:26-43` | 非法/抛错 RNG 回退 0.5 |
| `withJitter(delayMs, random)` | `:52-59` | **单次采样**、只正向、`min(base*10%, 5s)` 上限 |
| `nextReloadDelay(data, {failureCount, nowMs, random})` | `:81-129` | 总管道：失败阶梯 / null / 不可用→1s / partial→30s / stale→15s / fresh→expires_at（1s 地板）→ 加抖动 → **jitter 后钳制到 `stale_until - now`**（:122-128） |

### 1.3 classroomDataValidity.js（PRD 要求原样保留）

唯一导出 `isUsableBusinessDaySnapshot(data, nowMs)`（`frontend/src/classroomDataValidity.js:7-32`）：`campuses` 是数组 + `date` 等于 Asia/Shanghai 今天 + `stale_until` 可解析且在未来。失败一律 fail-closed 不抛。

### 1.4 todayClassroomsResponse.js（PRD 要求保留，normalizeResponse 按 R4 改造）

| 导出 | 位置 | 职责 |
|---|---|---|
| `loadingResponse = {code:1, msg:"加载中", data:null}` | `frontend/src/todayClassroomsResponse.js:1` | 初始态 envelope |
| `fallbackErrorMessage` | `:3` | 兜底文案 |
| `classroomWarningMessage(data)` | `:5-35` | stale Alert 文案：partial_campuses ID → 校区名解析 |
| `extractMessage(payload)` | `:37-41` | 安全取 msg |
| `normalizeResponse(payload)` | `:43-76` | **三处 throw 做控制流**（见 §4） |
| `readJson(response)` | `:78-84` | json 解析失败返 null |

### 1.5 相关测试清单与各自锁定语义

| 文件 | 锁定内容 | SWR 化后命运 |
|---|---|---|
| `frontend/src/useTodayClassrooms.test.js`（206 行） | 纯函数：hasUsableClassroomData 接受/拒绝矩阵（:41-79）、shouldFullPageSpin 四象限（:81-91）、nextFailureCount（:93-99）、mergeFetchResult 六场景——保留快照+stale 注入（:102-121）、无先验硬空（:123-140）、跨日/过期清空（:142-157）、成功信封坏元数据 fail-closed（:159-175）、成功替换（:177-204） | 基本原样保留（这些函数继续做渲染期派生）；仅 `nextFailureCount` describe 可删 |
| `frontend/src/useTodayClassrooms.lifecycle.test.jsx`（357 行，jsdom + fake timers `shouldAdvanceTime:true`） | 8 个 case：①mount 初载（:98-111）②**unmount abort 在途 fetch**（:113-140）③手动 retry 二次请求清全屏错误（:142-169）④背景失败保留快照置 stale（:171-207）⑤unmount 清调度 timer（:209-220）⑥40s 超时安全文案（:222-249）⑦隐藏不排背景轮询（:251-274）⑧隐藏→过 stale_until→可见：单次提示重载、重复 visible 事件不加班（:276-355） | 需重写（详见 §5.2）；②语义在 SWR 下不成立 |
| `frontend/src/reloadSchedule.test.js`（300 行） | 10/20/30/60 封顶、null/malformed、15s/30s/expires_at、跨日/过期 1s、stale_until 钳制（含 sample=1）、硬期限先于限流地板、单次采样、正向抖动界、坏 RNG 回退 | **原样全保留**（模块不动） |
| `frontend/src/classroomDataValidity.test.js`（46 行） | 同日接受、跨日/过期拒绝、malformed fail-closed | **原样全保留** |
| `frontend/src/todayClassroomsResponse.test.js`（76 行） | normalizeResponse 成功保留/非零信封安全化（:8-41）、**malformed 用 `toThrow` 断言**（:42-53）、classroomWarningMessage（:56-75） | toThrow 断言随 R4 改为返回值断言；其余保留 |

### 1.6 权威契约逐条（`.trellis/spec/backend/api-contract.md:181-268` "Scenario: Frontend Snapshot Validity and Reload Backoff"）

§3 Contracts（:200-222）：
1. 可展示快照 = `campuses` 数组 + `date` 为上海当日 + `stale_until` 可解析且未来（:202-203）。
2. `mergeFetchResult` 仅当先验快照仍可展示时才保留，否则 `data: null`（:204-205）。
3. 连续客户端失败按 10s/20s/30s、封顶 60s 重试；有效 HTTP 200 教室 payload 清零计数（:206-207）。
4. partial payload 基础轮询 30s；普通 stale 15s；fresh 等到 `expires_at`（1s 地板）（:208-209）。
5. 调度管道：base → **单次**随机采样 → 只正向抖动 `base + sample * min(base*10%, 5s)` → 快照仍可展示时绝对钳制 `max(0, stale_until - now)`；业务硬期限早于限流地板时业务期限赢；抖动不得缩短最小间隔（:210-214）。
6. 非法/抛错/非有限 RNG 回退 sample 0.5，永不产出 NaN 延迟（:215-216）。
7. 背景重试永不启用全屏 spinner；timer 在无效边界醒来时先清 campuses 再重载；**隐藏标签取消 timer**；过 `stale_until` 后变可见要及时重验，不能按普通轮询间隔保留昨天的筛选（:217-220）。
8. `partial_campuses` 可选；警告用 payload 内校区名解析 ID，无名回退 ID（:221-222）。

§4 验证矩阵（:226-235）：同日+失败→保留+stale+client_refresh_failed；跨日/过期+失败→硬空；缺字段→fail-closed 不抛；失败计数 1/2/3/4+→10/20/30/60s；valid partial→清零计数+30s 轮询；valid stale→15s；fresh expiry 晚于 stale_until→在 stale_until 醒；sample=1 靠近硬期限→最终延迟 ≤ 剩余 stale_until。

§6 Tests Required（:247-259）**点名了 5 个测试文件及场景**——特别是 `useTodayClassrooms.lifecycle.test.jsx`: "hidden pause; hide → past deadline → visible → single prompt reload"。SWR 化重写测试后此节需主 agent 用 update-spec 同步文件名/场景描述。

---

## 2. SWR 调研（基于 swr@2.4.2 正式包源码核对）

### 2.1 版本与体积

- npm dist-tags：`latest = 2.4.2`（2026-06-22 发布）；`2.5.0-beta.1` 存在但不采用。
- peerDependencies：`react ^16.11 || ^17 || ^18 || ^19` → react 18.3.1 兼容，React 19 升级（批次④）也无阻。
- 体积实测：`dist/index/index.mjs` gzip 9.2KB + `dist/_internal/index.mjs` gzip 1.8KB（**未 minify** 的 ESM 源）；经 Vite minify 后估约 5-6KB gz。审计写的 "~4KB" 偏乐观，以 rollup-plugin-visualizer 实测为准（R8）。
- 内部用 `useSyncExternalStore`（React 18 原生），无兼容层负担。

### 2.2 refreshInterval 函数形式（关键机制，逐条源码核对）

签名：`refreshInterval?: number | ((latestData) => number)`；函数收到的是 `getCache().data` 即 **fetcher 上次返回值**（swr:index.mjs:605）。

轮询实现（swr:index.mjs:599-634）要点：
1. **effect deps = `[refreshInterval, refreshWhenHidden, refreshWhenOffline, key]`——数据到达不会重启该 effect**。
2. `next()`：`if (interval && timer !== -1) timer = setTimeout(execute, interval)` ——**返回 0/null/undefined 会直接终止轮询链**。若 `refreshInterval` 函数身份稳定（useCallback）且首载时 data 为 undefined（`nextReloadDelay(undefined)` 返回 null），轮询链**永远不会启动**。若传内联箭头（每渲染新身份），effect 每次渲染重排程——链能活，但 pending timer 每次渲染被重置重算（15s/30s 相对间隔会被无关重渲染顺延）。**两种身份策略都有坑，实现时必须显式选择**：推荐稳定身份 + 包装器 `(data) => Math.max(1, nextReloadDelay(data, {failureCount: 0}) ?? MIN_FRESH_DELAY_MS)` 之类，保证永不返回 falsy/0。
3. `execute()`：`if (!getCache().error && (refreshWhenHidden || isVisible()) && (refreshWhenOffline || isOnline())) revalidate(WITH_DEDUPE).then(next); else next();`（swr:index.mjs:616）。三个结论：
   - **缓存中有 error 时轮询完全暂停**，调度权独占移交 onErrorRetry；成功后自动恢复——与现有"单调度器"模型天然吻合，无双发风险；
   - 隐藏时 **timer 照常重排但不发请求**（与现状"取消 timer"实现不同、可观察行为等价：隐藏期零 fetch）；
   - 下一个间隔在 revalidate **完成后**才计算排程（`.then(next)`），间隔起点是请求完成点（现状也是：调度 effect 在 reloading 清零后才跑）。

### 2.3 keepPreviousData 真实语义（PRD 假设需修正）

swr:index.mjs:285-286：`returnedData = keepPreviousData ? (cachedData 为 undefined ? laggyDataRef.current : cachedData) : data`——laggy 数据只在**当前 key 无缓存**（即 key 切换瞬间）生效。本项目 key 恒为 `"/api/get_data"`，**keepPreviousData 是 no-op**。
"失败保留上次快照"的真正机制是 SWR 的 **data / error 双轨存储**：fetcher 抛错只写 `error`，**不清 `data`**（swr:index.mjs:470-477，`finalState.error = err`，data 不动）。PRD R3 把 keepPreviousData 列为手段属于误配——不影响可行性，但 design.md 不应照抄。

### 2.4 onErrorRetry 自定义

- 签名：`onErrorRetry(err, key, config, revalidate, revalidateOpts)`，`revalidateOpts = { retryCount, dedupe: true }`。
- **retryCount 从 1 开始**：`retryCount: (opts.retryCount || 0) + 1`（swr:index.mjs:493）；链式重试把 opts 传回 `revalidate(opts)` 继续递增——与 `failureRetryDelay(failureCount)` 的 1→10s、2→20s、3→30s、4+→60s **完全对齐，零换算**。
- 默认实现是指数退避（`~~((Math.random()+0.5) * (1 << min(retryCount,8))) * errorRetryInterval`，config.ts），必须替换。
- 触发前置条件（swr:index.mjs:483）：`!revalidateOnFocus || !revalidateOnReconnect || isActive()`，`isActive = isVisible() && isOnline()`。默认配置下（两开关都 true）：**标签隐藏时不再发起下一轮重试**，靠 revalidateOnFocus 回归后续命——正合"隐藏暂停"。**反直觉陷阱：若关掉 revalidateOnFocus，此条件恒真，隐藏页会继续重试**，违反契约。
- **已排定的 retry timer 不受可见性门控**：`ERROR_REVALIDATE_EVENT → revalidate(opts)` 无 isActive 检查（swr:index.mjs:562-563）。即失败后 10s 的 setTimeout 若在用户隐藏标签后到点，会照常 fetch。现状实现会在隐藏瞬间 cleanup 掉 timer。**等价做法：自定义 onErrorRetry 的 setTimeout 回调内自查 `document.visibilityState`，隐藏则直接放弃（revalidateOnFocus 会在回归时补一发）**。
- retryCount 重置：成功 revalidation 或任何非 retry 来源（focus/mutate/轮询）的新请求都不带 retryCount → 计数归零。差异：现状 `failureCount` 只在**成功**后清零；SWR 下 focus 触发的失败会把阶梯重置回 10s。影响很小（更激进一档），但测试断言要按新语义写。
- 退避期间的 stale_until 钳制（契约 §3.5"failureCount=4 也要在 stale_until 醒"，reloadSchedule.test.js:156-170 锁定）：自定义 onErrorRetry 内不要用裸 `failureRetryDelay`，而是 `nextReloadDelay(latestData, { failureCount: retryCount })`——latestData 可经 hook 作用域的 dataRef 或 `cache.get(key)` 取得，该函数自带钳制。

### 2.5 默认值与本项目需求映射（config.ts + web-preset.ts 核对）

| 配置 | 默认值 | 本项目取值与理由 |
|---|---|---|
| `revalidateOnFocus` | true | **保持 true**：承担"过期后变可见即重验"；且 false 会破坏隐藏时重试暂停（§2.4） |
| `focusThrottleInterval` | 5000 | 建议**调大（如 15_000，对齐 STALE_POLL_MS）**：SWR 每次 visible/focus 都会 refetch（现状只在到期时才会），5s 节流下频繁切标签会超出现有请求预算（Nginx 30r/m 多标签场景） |
| `revalidateOnReconnect` | true | 保持 true（增益语义，现状没有；online 事件重验） |
| `refreshWhenHidden` | false（undefined→falsy） | 保持默认：隐藏暂停轮询 |
| `refreshWhenOffline` | false | 保持默认 |
| `dedupingInterval` | 2000 | 生产默认即可；**测试中设 0** 防止吞掉相邻断言请求 |
| `shouldRetryOnError` | true | 保持 true + 自定义 onErrorRetry |
| `revalidateIfStale` | true | 保持（挂载即首载） |
| `errorRetryCount` | undefined（不封顶） | 不设（阶梯自身封顶 60s，永续重试符合现状） |

focus 事件源：`initFocus` 同时监听 `document.visibilitychange` 与 `window.focus`（web-preset.ts:31-41）；FOCUS_EVENT 处理带节流 + isActive 检查（swr:index.mjs:550-555）——**重复 visible 事件天然去重**（对应 lifecycle 测试 :336-339 的断言）。

### 2.6 fetcher 错误、AbortSignal.timeout、状态位

- fetcher **抛出**即错误路径：`error` 持有抛出的对象（可以是自定义 Error 子类携带 status/log_id），`data` 保持上次成功值。
- `isLoading`：请求在途且**无缓存数据**（首载/首载失败后重试）；`isValidating`：任意请求在途。全屏 spin 映射见 §3。
- fetcher 内超时写法：
  ```js
  async function fetchTodayClassrooms(url) {
    let response;
    try {
      response = await fetch(url, {
        headers: { Accept: "application/json" },
        signal: AbortSignal.timeout(CLIENT_FETCH_TIMEOUT_MS),
      });
    } catch (err) {
      if (err?.name === "TimeoutError") throw new ApiError(CLIENT_FETCH_TIMEOUT_MESSAGE, ...);
      throw err;
    }
    ...
  }
  ```
  注意超时抛的是 `DOMException name="TimeoutError"`（不是 AbortError）。已实测：jsdom 26.1.0 与本机 Node 的 `AbortSignal.timeout` 均存在（typeof function）。**未验证** vi.useFakeTimers 能否推进其内部计时器（大概率不能，jsdom 内部计时器不走被 patch 的全局 setTimeout）——测试改用 `vi.spyOn(AbortSignal, "timeout")` 返回可控 AbortController.signal 即可绕开，或 fetcher 保留可注入的超时 signal 工厂。
- **SWR 卸载不 abort 在途请求**（仅 `unmountedRef` 忽略晚到结果）——现状 lifecycle 测试②的"unmount abort fetch"保证会丢失（见 §5）。
- SWRConfig provider：应用侧**非必须**（所有配置可作 `useSWR(key, fetcher, config)` 第三参）；**测试侧必须**用 `<SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>` 包 probe，否则模块级全局缓存跨 case 污染。

---

## 3. 语义等价映射表

| 契约条目（api-contract.md 锚点） | 现有承担者 | SWR 化后承担者 | 类型 |
|---|---|---|---|
| 同日快照失败保留 + stale/client_refresh_failed 注入（:204-205, :228） | `mergeFetchResult` 在 setState 时合并（useTodayClassrooms.js:159-163,177-181） | SWR data/error 双轨天然保留原始 data；**渲染期派生**：`resp = mergeFetchResult(successEnvelope(data), errorToEnvelope(error), Date.now())` —— mergeFetchResult **原样复用**，测试原样活 | 内建+复用 |
| 跨日/过期硬清空（:205, :229） | mergeFetchResult 分支 3 + 调度 effect 预清（:214-218） | 同上渲染期派生：`hasUsableClassroomData` 为 false 即产出硬 envelope（含"当前缓存已失效，正在重新获取"态：data 不可用且 isValidating）；跨 stale_until 的重渲染由钳制到 stale_until 的 revalidation 触发 | 自定义保留 |
| 10/20/30/60 退避、成功清零（:206-207, :231） | failureCount state + nextFailureCount + nextReloadDelay | 自定义 `onErrorRetry`：`setTimeout(() => visible && revalidate(opts), nextReloadDelay(latestData, { failureCount: retryCount }))`；retryCount 起值 1 与阶梯对齐；成功清零 SWR 内建 | nextReloadDelay 复用 |
| partial 30s / stale 15s / fresh expires_at 1s 地板（:208-209, :232-233） | 调度 effect + nextReloadDelay | `refreshInterval: (data) => nextReloadDelay(data)` **加防 falsy 包装**（§2.2 陷阱） | nextReloadDelay 复用 |
| 单次随机采样 + 只正向抖动（:210-214） | withJitter | nextReloadDelay 内部行为，随复用直接满足；SWR 每次排程调用函数恰一次 | 复用 |
| stale_until 钳制（含 sample=1、含失败退避路径）（:212-214, :234-235） | nextReloadDelay :122-128 | 轮询路径随 refreshInterval 复用；**失败路径必须在 onErrorRetry 里也走 nextReloadDelay**（不能裸用 failureRetryDelay） | 复用+注意 |
| 隐藏暂停（:219 "Hidden tabs cancel timers"） | pageVisible state；隐藏即 cleanup timer | `refreshWhenHidden:false`（默认）：timer 照排但**零 fetch**（行为等价，机制不同）；error 重试链隐藏时不续（§2.4）；**已排定 retry timer 需回调内自查 visibility** | 内建+小补丁 |
| 过期后变可见及时重验、单次不抖动（:219-220） | pageVisible 变化 → effect 重跑 → nextReloadDelay(过期)=1s | `revalidateOnFocus`（visibilitychange+focus，立即执行，focusThrottleInterval 节流保证单发）——比现状 1s+jitter 更快 | 内建 |
| 重复 visible 事件不加班（lifecycle :336-339） | React state 相同不重跑 effect | focusThrottleInterval 节流 | 内建 |
| 背景重试永不全屏 spinner（:217, :230 fail-closed） | spinning state + shouldFullPageSpin | `isLoading`（首载=无缓存数据+在途）覆盖主场景；"数据存在但不可用时的手动重试"需局部 manual-flag（retry 包装 mutate 时置位）+ `shouldFullPageSpin` **原样复用** | 内建+复用 |
| 手动重试（retry） | setReloadRequest bump | bound `mutate()` | 内建 |
| 40s 超时安全文案 | 手写 AbortController+setTimeout（:121-127） | fetcher 内 `AbortSignal.timeout(40_000)` + TimeoutError→文案映射 | 内建 API |
| loadingResponse 首屏态 | resp 初值 | `!data && !error` 派生 loadingResponse | 自定义保留 |

**必须原样保留的模块（PRD R3 明文 + 契约 §6）**：
- `reloadSchedule.js` 全部导出 + `reloadSchedule.test.js`（一行不动）；
- `classroomDataValidity.js` + 测试（一行不动）；
- `todayClassroomsResponse.js`：`classroomWarningMessage`/`readJson`/`loadingResponse`/`fallbackErrorMessage`/`extractMessage` 不动；`normalizeResponse` 按 R4 改造（测试相应改）；
- `useTodayClassrooms.js` 导出的 `hasUsableClassroomData`/`shouldFullPageSpin`/`mergeFetchResult` 建议保留为渲染期派生纯函数（`useTodayClassrooms.test.js` 相应部分零改动存活）；`nextFailureCount` 可删（SWR retryCount 取代）。

---

## 4. R4：normalizeResponse 去 throw + log_id 透传

### 4.1 throw 控制流现状

`frontend/src/todayClassroomsResponse.js:43-76` 三处 throw，全部针对 malformed 输入：
- `:44-46` payload 非对象 → `throw "服务返回格式异常"`；
- `:49-51` code 非有限数 → `throw "服务返回状态异常"`；
- `:61-66` code===0 但 data 非对象 / campuses 非数组 → `throw "服务返回数据/校区数据异常"`。
非 0 信封已经走返回值（:53-59）。唯一调用点 `useTodayClassrooms.js:155`（try 块内），throw 被 :164 catch 统一吞成 `errorEnvelope(error.message)`——**HTTP status 结构性丢失，一律变 500**（审计已点名 errorEnvelope 默认 500 问题）。测试锁定在 `todayClassroomsResponse.test.js:42-53`（4 个 toThrow 断言）。

### 4.2 改返回值方案（与 SWR 的配合）

SWR 语境下 fetcher **应该** throw（那是 error 状态的唯一入口），所以 R4 的正确切分是：
- `normalizeResponse` 改为纯函数返回判别结果，例如 `{ ok: true, resp } | { ok: false, reason }`（或返回 envelope + 独立校验函数），**函数体内零 throw**；
- throw 收敛到 fetcher 边界一次：fetcher 拿到 `!response.ok` 或 `ok:false` 结果时 `throw new ApiError(msg, { status: response.status, code, logId })`——真实 HTTP status 与业务 code 都结构化保留在 error 对象上，渲染期派生把它转 envelope 时不再猜 500。
- `todayClassroomsResponse.test.js:42-53` 的 toThrow 断言改为返回值断言（约 12 行）。

### 4.3 log_id 现状与透传改动面

- 后端：`/api/get_data` 仅**错误** envelope 带 body `log_id`（`handler.go:46-52`）；成功信封只有 `code`/`data`（`handler.go:55-58`）。所有 /api 响应统一带 `X-Log-Id` 响应头（`router.go:87-94`）；404/panic 信封也带 body log_id（`router.go:194`）。
- 前端现状：`normalizeResponse` 只取 code/msg/data（todayClassroomsResponse.js:48-75）→ log_id 被丢弃。
- 改动面（小）：
  1. fetcher：`const logId = payload?.log_id || response.headers.get("X-Log-Id") || ""`，挂到 ApiError / 错误 envelope；
  2. 派生 resp envelope 增加 `logId` 字段（仅错误态需要）；
  3. `GlobalEmpty.jsx`（`frontend/src/components/GlobalEmpty.jsx:10-24`）：`props.isError` 分支的 description 下补一行小字（如 `<div className="global-empty__log-id">log_id: {logId}</div>`），CSS 加 2-3 行淡色小号样式；PropTypes 不需大改（todayData 是 `PropTypes.object`）。
- 注意：成功但 partial 的 payload body **无** log_id，X-Log-Id 头有——但 stale Alert 场景没有透传诉求，只做硬错误态展示即可（PRD 原意"GlobalEmpty 错误态展示"）。

---

## 5. 风险、测试工作量、逃生方案

### 5.1 最难等价的语义（按风险排序）

1. **refreshInterval 返回 falsy 的链死陷阱**（§2.2）：`nextReloadDelay(undefined)` 返回 null；稳定函数身份 + 首载 null = 轮询永不启动；内联函数身份 = pending timer 被无关重渲染重置。必须写包装器且 design.md 明确身份策略——这是 SWR 化最容易翻车、且现有测试矩阵覆盖不到的点（需新增"首载后轮询确实启动"的测试）。
2. **mergeFetchResult 的 stale 注入从 setState 时点移到渲染期**：nowMs 从"合并时刻"变"渲染时刻"。跨 stale_until 的重渲染依赖钳制过的 revalidation 触发（isValidating 翻转），链条是 refreshInterval 钳制 → revalidation → 重渲染 → 派生看到过期 → 硬清 UI。任何一环掉（如上条链死）UI 会冻结在昨日数据。缓解：派生逻辑保守（每次渲染重算 usable）+ 上条防御。
3. **visibilitychange 精确行为差异**：(a) 已排定的 error-retry timer 隐藏后到点仍会 fetch（需回调自查，§2.4）；(b) revalidateOnFocus 让**每次**切回标签都可能 refetch（现状只在到期时），5s 默认节流下多标签用户请求量上升，需调 focusThrottleInterval 并对照 Nginx 30r/m 预算；(c) "隐藏取消 timer"变"隐藏空转 timer"——可观察行为等价，但契约 §3.7 字面写的是 cancel timers，spec 措辞需随之更新。
4. **focus 失败重置退避阶梯**（§2.4 retryCount 重置差异）：语义轻微变化，测试要按新语义断言。
5. **unmount 不再 abort 在途 fetch**：SPA 单页常驻，实际影响≈0（只有关页时），但 lifecycle 测试②要改写成"晚到响应不更新已卸载组件"。

### 5.2 测试重写工作量评估

| 桶 | 文件/case | 工作量 |
|---|---|---|
| 零改动 | reloadSchedule.test.js（300 行）、classroomDataValidity.test.js、todayClassroomsResponse.test.js 的 classroomWarningMessage 段 | 0 |
| 小改 | useTodayClassrooms.test.js 删 nextFailureCount describe（~7 行）；todayClassroomsResponse.test.js toThrow→返回值（~12 行） | <0.5h |
| 重写 | lifecycle.test.jsx 8 case：①③④⑤⑦⑧可等价迁移（需加 SWRConfig `provider: () => new Map()` + `dedupingInterval: 0` 样板，注意 focusThrottleInterval 对⑧重复事件断言的作用）；②改写为"晚到响应无害"；⑥改用 stub AbortSignal.timeout 或注入 signal 工厂 | 1-1.5 天（主要耗在 fake timers × SWR `revalidate().then(next)` 微任务交错的 flaky 调参；现有 `shouldAdvanceTime: true` 配置是有利条件） |
| 新增 | 轮询启动测试（防链死）、onErrorRetry 阶梯抽成纯函数 `retryDelayFor(retryCount, data)` 的单测、log_id 展示测试 | ~0.5 天 |

### 5.3 逃生方案（fallback 判据）

**触发任一即回退**：
- (a) fake timers 下 SWR 轮询/重试链无法稳定测试（间歇性 flaky 且 2 小时内无解）；
- (b) 为对齐契约写的自定义胶水（refreshInterval 包装 + onErrorRetry + 可见性补丁 + 派生层）超过被删掉的手写代码量的一半（手写核心约 139 行），即"SWR 只省了不到 70 行却引入黑盒依赖"；
- (c) revalidateOnFocus 请求量调不进 Nginx 30r/m 预算（多标签场景）。

**fallback 内容**：保留手写数据层但按审计退路瘦身——6 useState 合并为 1 个 useReducer 状态机、消灭 respRef 渲染期写入与 :157-163/:176-181 重复块、fetch 改 `AbortSignal.timeout`；R4（normalizeResponse 去 throw + log_id）**照做不受影响**；PRD R3/R5 由主 agent 改注记。R4 与 R3 无耦合，可作为独立先行提交降低整批风险。

## Caveats / Not Found

- **未实测** vi.useFakeTimers 能否驱动 jsdom 的 `AbortSignal.timeout` 内部计时器（研究判断为不能）；实现期先跑一个 spike 测试确认，绕法已给（spy/注入）。
- deepwiki MCP 工具本会话不可用；SWR 结论全部来自 swr@2.4.2 npm tarball 构建产物源码核对（行号以 `dist/index/index.mjs` 为准），可信度高于二手文档。
- `swr` 2.4.2 的 `dist` 行号在未来 patch 版本可能漂移；关键行为（error 时轮询暂停、retryCount 起值 1、keepPreviousData 仅 key 切换、ERROR_REVALIDATE_EVENT 无可见性门控）自 2.0 起稳定。
- api-contract.md §3.7 "Hidden tabs cancel timers" 与 §6 点名的测试文件名在 SWR 化后需主 agent 走 update-spec 同步措辞，本研究不改 spec。
