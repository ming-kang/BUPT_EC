# Research: 静态服务与 embed 解耦

- **Query**: go:embed 现状、frontend/dist 产物结构、裸克隆症状、web/ 双实现方案、FileServerFS/ETag 接线、CI 构建标签影响面
- **Scope**: mixed（内部代码 + 实测 + Go 标准库/gin-contrib 源码）
- **Date**: 2026-07-27
- **关联**: PRD R5/R6（`.trellis/tasks/07-26-degin-http-rewrite/prd.md:24-25`）、父任务审计 `.trellis/tasks/07-26-arch-simplify-refactor/research/audit-engineering.md:7-14,50`

---

## 1. 现状：embed 与静态服务

### go:embed 位置

- `router.go:17-18`：`//go:embed frontend/dist` + `var f embed.FS`，在根包 `main` 内。这就是裸克隆全挂的根源——embed 模式在**包加载期**解析，dist 不入库（`frontend/.gitignore:11` 的 `dist`）。

### gin-contrib/static 用法

- `router.go:13` import `github.com/gin-contrib/static`（go.mod:6，v1.1.6）。
- `router.go:20-41`：本地复刻了一个 `embedFileSystem`（实现 `static.ServeFileSystem` 接口的 `Exists`）+ `EmbedFolder`（`fs.Sub(fsEmbed, "frontend/dist")` 后包 `http.FS`）。注意这是**自写的**，没有用库自带的 `static.EmbedFolder`（库版在 v1.1.6 `embed_folder.go:30` 返回 `(ServeFileSystem, error)`）。
- `router.go:80`：`r.Use(static.Serve("/", EmbedFolder(f, "frontend/dist")))` 挂在 `/healthz`、`/readyz`、`/metrics`、`/api` 路由**之后**注册，作为兜底中间件。
- 库内部实现（模块缓存 `gin-contrib/static@v1.1.6/serve.go:19-31`）：`Serve` 就是 `http.FileServer(fs)` + `Exists` 命中才 `ServeHTTP` 并 `c.Abort()`。即所有响应头行为等同标准库 `http.FileServer`。

### SPA fallback / index.html 服务

- `router.go:82-100` `r.NoRoute`：API 路径返回 JSON 404 信封（`writeAPINotFound`，router.go:58-65）；非 API 路径**每次请求**用 `f.Open("frontend/dist/index.html")` + `io.ReadAll` 重读 index.html，`c.Data(200, "text/html; charset=utf-8", data)` 返回（父审计 audit-engineering.md:50 已点名"NoRoute 每次重读"）。
- `/` 根路径由 `static.Serve` 命中（`Exists("/", "/")` 时 `http.FileServer` 自动服务目录下 index.html）。
- `/index.html` 显式访问会被 `http.FileServer` 规范化 **301 → `./`**（实测见下）。

### 静态响应缓存头（实测，gin-contrib/static v1.1.6 默认行为）

在临时克隆中以 `fakeClassroomService` 装配真实 `RegisterRoutes` 后 httptest 实测：

| 请求 | 状态 | 响应头 |
|---|---|---|
| `GET /` | 200 | `Content-Type: text/html; charset=utf-8`、`Accept-Ranges: bytes`、`Content-Length: 797` |
| `GET /index.html` | 301 | `Location: ./` |
| `GET /assets/index-WFBypEFs.css` | 200 | `Content-Type: text/css; charset=utf-8`、`Accept-Ranges: bytes` |
| `GET /favicon.ico` | 200 | `Content-Type: image/x-icon`、`Accept-Ranges: bytes` |
| `GET /some/deep/link`（SPA fallback） | 200 | `Content-Type: text/html; charset=utf-8`（无 Accept-Ranges，走 c.Data 路径） |

结论：
- **完全没有 `Cache-Control` / `ETag` / `Last-Modified`**（全仓 Go 代码 grep 三者零命中）。
- `Last-Modified` 缺失是 embed.FS 固有行为：embed 文件 `ModTime()` 为零值，`http.FileServer` 对零 modtime 不发 Last-Modified、也**忽略 If-Modified-Since**（实测带 `If-Modified-Since: 2035` 仍 200，永远全量回包）。
- `Accept-Ranges: bytes` 有（http.FileServer 自带 Range 支持）；SPA fallback 路径没有。
- gzip 由 `router.go:201-221` 的 gzipMiddleware 统一包裹（本文件不展开，属 gzhttp 主题）。

## 2. frontend/dist 产物结构

当前实际产物（总计 ~936 KB）：

```
frontend/dist/
├── index.html                                (797 B)
├── favicon.ico                               (9.8 KB, 来自 frontend/public/favicon.ico)
└── assets/
    ├── index-mabor-gJ.js                     (415 KB, 入口 chunk)
    ├── react-vendor-Dv6xG0fm.js              (143 KB, manualChunks 见 vite.config.js:10-17)
    ├── TodayClassroomTable-BHNvPZQi.js       (310 KB, 懒加载 chunk)
    ├── CampusSettingsModal-WM2fax4v.js       (13 KB, 懒加载 chunk)
    ├── index-DPVRhY_J.js                     (29 KB)
    ├── index-WFBypEFs.css                    (4.9 KB)
    └── TodayClassroomTable-DQdjHLRD.css      (2.6 KB)
```

- **hash 规律**：Vite 默认 `[name]-[hash:8(base64url)]` 全部集中在 `assets/` 一层，无子目录。`/assets/*` 可安全上 immutable。
- **index.html 引用方式**（frontend/dist/index.html:10-12）：绝对路径 `/assets/index-mabor-gJ.js`（module script）+ modulepreload + stylesheet，均带 crossorigin；无内联 JS（CSP script-src 'self'，index.html:16）。
- **顶层非 hash 文件**：`index.html` 与 `favicon.ico` 两个。favicon 来自 `frontend/public/`（public 目录原样拷贝、**不带 hash**）→ 不能 immutable，建议与 index.html 同档（no-cache 或短 max-age）。
- 无 `.woff2`/图片/manifest 等其他产物；无预压缩 `.gz/.br`（PRD 已列 Out of Scope）。

## 3. 裸克隆问题的精确症状（实测）

`git clone` 到干净目录（无 frontend/dist）后：

```
$ go vet ./...
router.go:17:12: pattern frontend/dist: no matching files found   # exit 1

$ go build ./...
router.go:17:12: pattern frontend/dist: no matching files found   # exit 1

$ go test ./...
# BUPT_EC
router.go:17:12: pattern frontend/dist: no matching files found
FAIL    BUPT_EC [setup failed]
ok      BUPT_EC/config
ok      BUPT_EC/logs
ok      BUPT_EC/service
ok      BUPT_EC/utils
```

只有根包（及依赖根包的一切：build、vet、gopls）挂；子包测试仍可跑。注意 **`mkdir -p frontend/dist` 空目录也不行**——embed 模式要求匹配到至少一个文件，空目录同样报 `no matching files found`。

## 4. 目标方案：web/ 独立包 + 构建标签双实现

### 构建标签写法（两文件草案）

`web/web.go`（无标签，公共接口 + 提示页）：

```go
// Package web owns the embedded frontend assets. By default the package
// compiles without frontend/dist so a bare clone passes go vet/test; the
// real assets are embedded only with -tags embed_assets (see Taskfile).
package web

import "io/fs"

// placeholderHTML is served when the binary was built without embed_assets.
const placeholderHTML = `<!DOCTYPE html><html><head><meta charset="utf-8">` +
	`<title>frontend not built</title></head><body>` +
	`<h1>Frontend not built</h1>` +
	`<p>This binary was compiled without -tags embed_assets. ` +
	`Run <code>task build</code> for a complete binary.</p></body></html>`

// Dist returns the frontend asset tree rooted at dist/ (index.html at the
// root) and whether real embedded assets are present.
func Dist() (fs.FS, bool) { return distFS() }
```

`web/embed_enabled.go`：

```go
//go:build embed_assets

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

func distFS() (fs.FS, bool) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // impossible: dist is the embedded root
	}
	return sub, true
}
```

`web/embed_disabled.go`：

```go
//go:build !embed_assets

package web

import (
	"io/fs"
	"testing/fstest"
)

func distFS() (fs.FS, bool) {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(placeholderHTML)},
	}, false
}
```

要点：
- `//go:build embed_assets` / `//go:build !embed_assets` 互斥，二者必居其一，包始终可编译。
- `all:dist` 前缀确保连 `.`/`_` 开头文件也嵌入（当前 dist 没有这类文件，但 Vite 升级后可能出现，加 `all:` 更稳）。
- `fstest.MapFS` 在非 test 代码中使用是合法且零依赖的（testing/fstest 是普通库包，不会拉 testing flag 副作用——它只 import testing/iotest 之类；若介意，可换成 `embed.FS` 嵌一个入库的 `web/placeholder/index.html`，代价是多一层目录）。
- 布尔返回值让 main 在启动日志里打印 "serving placeholder frontend (built without embed_assets)"，避免线上误发裸二进制无感知。

### //go:embed 相对路径约束与 dist 位置

Go 规范：embed 模式**只能匹配包目录及其子目录**，禁止 `..`、绝对路径、符号链接目录。`web/` 包无法直接 `//go:embed ../frontend/dist`（编译错误 `pattern ../frontend/dist: invalid pattern syntax`）。三个选项：

| 选项 | 做法 | 评估 |
|---|---|---|
| A. 包放 frontend/ 下 | `frontend/frontend.go` + `//go:embed all:dist` | 不需要拷贝，但 Go 包进入 frontend/ 后 `go build ./...`/`go vet ./...`/gopls 会**遍历 frontend/node_modules**（Go 工具只跳过 `.`/`_` 前缀和 testdata，不跳过 node_modules；pnpm 的 .pnpm 目录树上万个目录），拖慢所有 Go 工具链；Go 代码和 pnpm 工程混居也脏 |
| B. web/ 包 + 构建时拷贝 | Taskfile `frontend:build` 末尾 `cp -r frontend/dist web/dist`（或 go:generate）；`.gitignore` 加 `/web/dist/` | 多一步拷贝，但拷贝由 Taskfile 统一编排（本任务恰好要引入 Taskfile）；CI 里 release.yml 的 download-artifact 可直接把 `path:` 指到 `web/dist`，零拷贝 |
| C. 改 Vite outDir | `vite.config.js` `build.outDir: '../web/dist'`（需 `emptyOutDir: true`） | 完全消除拷贝步骤，但前端构建产物直接落进 Go 包目录，Vite 配置与 Go 布局耦合；quality.yml 的 upload-artifact path、frontend/.gitignore 全要跟着改；前端单独开发时 dist 落在 frontend 外不符直觉 |

**推荐 B**：web/dist 作为"构建暂存区"语义清晰；Taskfile 正是本任务要新增的编排点（`task build` = frontend:build → copy → `go build -tags embed_assets`）；CI 免拷贝（download-artifact 直达 web/dist）。go:generate 变体不推荐——`go generate` 不会被 `go build` 自动触发，等于又造了一条隐式前置步骤，不如显式写进 Taskfile 依赖链。
注意 B 需要：`.gitignore`（根）加 `/web/dist/`；本地开发者文档说明 `task build` 会自动拷贝。

## 5. 服务接线草案：FileServerFS + ETag + immutable

Go 1.22+（go.mod:3 为 go 1.25.12，无版本障碍）。`http.FileServerFS(fsys fs.FS)` 是 1.22 新增，等价 `http.FileServer(http.FS(fsys))`。

```go
// 装配（main 或 internal/httpapi）
distFS, embedded := web.Dist()

// 启动时读一次 index.html，算 ETag
indexHTML, err := fs.ReadFile(distFS, "index.html")
if err != nil { /* fatal：嵌入产物必须含 index.html */ }
sum := sha256.Sum256(indexHTML)
indexETag := `"` + hex.EncodeToString(sum[:8]) + `"` // 强 ETag，16 hex 足够

serveIndex := func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("ETag", indexETag)
    // ServeContent 负责 If-None-Match→304、Range、Content-Type(按 name 后缀)
    http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indexHTML))
}

assets := http.FileServerFS(distFS) // 内部按扩展名设 Content-Type，带 Range

mux.Handle("GET /assets/", immutableCache(assets))
mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Cache-Control", "no-cache") // public/ 文件无 hash，不可 immutable
    assets.ServeHTTP(w, r)
})
mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/" || r.URL.Path == "/index.html" {
        serveIndex(w, r) // 顺带消灭现状的 /index.html→301 怪癖（或保持 301，design.md 定）
        return
    }
    serveIndex(w, r) // SPA fallback：非 /assets/ 非 /api 的一切深链接
})

func immutableCache(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
        next.ServeHTTP(w, r)
    })
}
```

要点与坑：
- **`GET /` 模式在 1.22 mux 里就是兜底**（匹配所有未被更具体模式命中的 GET/HEAD），天然承担 SPA fallback；`GET /api/get_data`、`GET /healthz` 等更具体模式优先。但 **API 404 信封**（现状 router.go:82-87：`/api/*` 未知路径返回 JSON 404）需要显式加 `mux.HandleFunc("/api/", apiNotFound)`，否则 `/api/unknown` 会掉进 `GET /` 返回 index.html——**这是行为回归高危点**（AC 第 2 条）。
- `/assets/` 下未命中文件：FileServerFS 返回 404 纯文本，与现状一致（现状 static.Serve Exists=false 后 NoRoute 返回 index.html——注意！现状 `/assets/nonexistent.js` 实际回落 index.html 200。新方案改为真 404 更正确，但要在 design.md 记录这个行为差异并调整/新增测试）。
- ETag+304：`http.ServeContent` 在响应头已有 `ETag` 时自动处理 `If-None-Match`（net/http/fs.go 的 checkIfNoneMatch），304 时自动剥 body。modtime 传零值即禁用 If-Modified-Since 分支，纯走 ETag——正合 embed 无 modtime 的现实。
- `HEAD` 请求：mux 的 `GET` 模式同时匹配 HEAD，ServeContent/FileServerFS 均正确处理。
- placeholder 模式（未嵌入时）：`web.Dist()` 第二返回值为 false 时 index.html 即提示页，整条链路不变，只是内容不同；ETag 照算，无需分支。
- gzhttp 包裹在这些 handler 外层时，其默认 Content-Type 白名单会跳过 image/x-icon 等，woff2/png 不二次压缩（本文件不展开）。

## 6. CI 关系：quality.yml / release.yml 的顺序与标签改造

### 现状顺序

- `quality.yml`（被 ci.yml:15 PR 触发、release.yml:19 quality-gate 复用）单 job 内**先前端后 Go**：
  - `.github/workflows/quality.yml:49-51` `pnpm build` 产出 frontend/dist → `:53-58` 上传 artifact `frontend-dist` → `:85-97` `go vet` / `go test -race` / `go build` / govulncheck。
  - 即：CI 里 Go 步骤从未见过"无 dist"状态，裸克隆问题只在本地/gopls 暴露，且 Go 检查被迫排在前端构建之后（人为串行化）。
- `release.yml`：`build-go` job（:23-69）checkout 后 **download-artifact `frontend-dist` → `frontend/dist`**（:45-49），再 `go build -trimpath -ldflags "-s -w -X main.version=..."`（:62-63），无任何 tags。

### 改造后哪些加 -tags embed_assets、哪些故意不加

| 位置 | 是否加 tag | 理由 |
|---|---|---|
| quality.yml `go vet ./...`（:86） | **不加** | 故意验证裸克隆路径可编译（embed_disabled 分支）。vet 默认只跑默认标签组合 |
| quality.yml `go test -race ./...`（:89） | **不加**（主跑）| 同上；测试在 placeholder FS 下也应全绿（SPA/静态测试对内容做语义断言而非全文比对） |
| quality.yml `go build ./...`（:92） | **不加** | 裸克隆可编译门禁 |
| quality.yml **新增一步** `go build -tags embed_assets ./...`（需先把 dist 拷到 web/dist，或直接 `task build`） | **加** | 否则 embed_enabled.go 的编译错误/embed 模式失配（如 dist 改名）只能在 release 才发现。embed 模式匹配失败正是要在 PR 门禁抓住的 |
| quality.yml govulncheck（:97） | 建议加 `-tags embed_assets` 或两遍 | govulncheck 按 build tag 分析可达性；不加则 embed_enabled.go 不在分析范围（低风险，无第三方依赖差异，可只跑默认） |
| release.yml `go build`（:62） | **必加** `-tags embed_assets` | release 二进制必须含真资产；同时 download-artifact `path:` 从 `frontend/dist` 改为 `web/dist`（:48），消灭拷贝步骤 |
| Taskfile `build` | 加 | `frontend:build` → copy dist → `go build -tags embed_assets -ldflags ...` |
| Taskfile `test` / `check` | 不加（默认） | 本地日常与 CI 门禁对齐；可另设 `test:embed` 变体按需 |

附加机会（可写入 design.md）：quality.yml 里 Go 步骤改用不依赖前端产物后，可把 Go 检查提到前端构建**之前**（或拆并行 job），缩短 PR 反馈；但 `go build -tags embed_assets` 验证步仍需排在 pnpm build 之后。

### 其他需同步的文件

- 根 `.gitignore`：加 `/web/dist/`（现有 `/dist/`、`/build/` 在 :12-14，不覆盖 web/dist）。
- `go mod tidy` 门禁（quality.yml:65-71）：`testing/fstest` 是标准库，无 go.mod 影响。
- README/AGENTS.md/docs/development.md 中"先 pnpm build 再 go build"的散文（父审计 audit-engineering.md:9,13）统一改口 `task build`（属 R7 范围）。

## Caveats / Not Found

- **行为差异清单（新方案 vs 现状）需在 design.md 逐条决策**：① `/assets/不存在的文件` 现状回落 index.html 200，新方案 404；② `/index.html` 现状 301→`./`，新方案可直接 200；③ 现状零缓存头 → 新增 ETag/Cache-Control 是纯增强，但 router_test 若有精确头断言需同步。
- 现状 `router_test.go` 对静态路径的具体断言未逐条核对（本文件聚焦装配层）；重写测试前应先跑一遍现有断言清单。
- `fstest.MapFS` 用于生产代码路径无先例风险实测（仅确认其为普通库包）；若团队洁癖，备选方案是嵌入一个入库的真实 placeholder 文件。
- gzhttp 与 recovery 的包装顺序、metrics DisableCompression 属其他研究主题，本文未展开。
