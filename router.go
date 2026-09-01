package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"BUPT_EC/logs"
	"BUPT_EC/web"

	"github.com/klauspost/compress/gzhttp"
)

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

// jsonBufPool avoids per-request buffer allocation for JSON encoding.
// Each buffer is Reset before use and returned after the response is written.
var jsonBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// writeJSON encodes v as JSON into a pooled buffer and writes it as the HTTP
// response body. The output is byte-compatible with the former json.Marshal
// path (no trailing newline). Encode errors are unreachable for the typed
// envelope structs used by all callers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	err := json.NewEncoder(buf).Encode(v)
	if err != nil {
		jsonBufPool.Put(buf)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// json.Encoder.Encode appends '\n'; trim it to match the prior Marshal
	// output that existing tests and the gzip byte-comparison assert on.
	body := buf.Bytes()
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
	jsonBufPool.Put(buf)
}

// writePreserializedGetData writes a success /api/get_data response by
// splicing pre-serialized "data" JSON into the envelope template. This
// eliminates per-request reflection-based json.Marshal for the large
// campus/room tree. The output is byte-identical to what writeJSON would
// produce for a getDataSuccessResponse with the same data.
func writePreserializedGetData(w http.ResponseWriter, status int, logID, ver string, dataJSON string) {
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	// Build: {"code":0,"log_id":"...","version":"...","data":...}
	// Use json.Marshal for the string fields to handle any escaping.
	buf.WriteString(`{"code":0,"log_id":`)
	logIDJSON, _ := json.Marshal(logID)
	buf.Write(logIDJSON)
	buf.WriteString(`,"version":`)
	verJSON, _ := json.Marshal(ver)
	buf.Write(verJSON)
	buf.WriteString(`,"data":`)
	buf.WriteString(dataJSON)
	buf.WriteByte('}')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
	jsonBufPool.Put(buf)
}

// newGzipWrapper builds the production compression wrapper. It is shared with
// the endpoint tests so test assemblies cannot drift from production gzip
// behavior. Contract: content-type allowlist + 1KB minimum size +
// single-layer compression (responses that already carry Content-Encoding
// pass through untouched).
func newGzipWrapper() func(http.Handler) http.HandlerFunc {
	wrapper, err := gzhttp.NewWrapper(
		// Keep the gzip-only behavior surface for now; delete this line to
		// enable zstd later as an independent change.
		gzhttp.EnableZstd(false),
		// Explicit allowlist: no image/png, font/woff2, or
		// application/octet-stream, so already-compressed static assets are
		// never re-compressed.
		gzhttp.ContentTypes([]string{
			"application/json",
			"text/html",
			"text/plain", // /metrics exposition: text/plain; version=0.0.4 (parameterless entries match any parameters)
			"text/css",
			"text/javascript",
			"application/javascript",
			"image/svg+xml",
			"application/xml",
			"application/wasm",
		}),
		gzhttp.MinSize(gzhttp.DefaultMinSize), // 1024; explicit as documentation
	)
	if err != nil {
		// Unreachable: all options above are statically valid.
		panic(fmt.Sprintf("gzhttp.NewWrapper: %v", err))
	}
	return wrapper
}

// gzipSkipProbes routes /healthz and /readyz around the gzhttp wrapper so
// probe responses are never compressed and never carry a Vary header,
// preserving the previous hard-coded path exemption byte for byte.
func gzipSkipProbes(compressed, uncompressed http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			uncompressed.ServeHTTP(w, r)
			return
		}
		compressed.ServeHTTP(w, r)
	})
}

// apiLogContext attaches exactly one request log_id for /api paths, including
// unknown /api routes handled by the fallback. Non-API traffic is left alone
// and must not carry an X-Log-Id header.
func apiLogContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			ctx := logs.GenNewContext(r.Context())
			w.Header().Set("X-Log-Id", logs.GetLogIDFromContext(ctx))
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// recoveryResponseWriter tracks whether the wrapped writer has produced any
// response bytes so the recovery middleware can decide whether a clean 500 is
// still possible after a panic.
type recoveryResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *recoveryResponseWriter) WriteHeader(status int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *recoveryResponseWriter) Write(data []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(data)
}

func (w *recoveryResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// recovery is the innermost middleware: panics are converted into a clean 500
// before anything reaches the gzhttp writer, so a panic can never emit a
// truncated gzip stream. The panic value and stack go to slog with the
// request log_id; no panic detail leaks to the client.
func recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &recoveryResponseWriter{ResponseWriter: w}
		defer func() {
			if v := recover(); v != nil {
				slog.ErrorContext(r.Context(), "handler panic",
					"panic", fmt.Sprint(v),
					"stack", string(debug.Stack()),
				)
				if !rw.wroteHeader {
					rw.WriteHeader(http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(rw, r)
	})
}

// immutableCache marks hashed /assets/* responses as immutable: the content
// hash in the filename is the version, so clients may cache for a year.
// Directory paths get a plain 404 instead: listings have no content hash and
// must never be served with (or without) the immutable header.
func immutableCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

// Routes assembles the complete HTTP handler chain (outer to inner):
// gzipSkipProbes -> gzhttp wrapper -> apiLogContext -> recovery -> mux.
func (server *HTTPServer) Routes() http.Handler {
	distFS, _ := web.Dist()

	// Read index.html once at assembly time. Both the embedded tree and the
	// placeholder FS guarantee its presence.
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		panic(fmt.Sprintf("frontend assets missing index.html: %v", err))
	}
	sum := sha256.Sum256(indexHTML)
	// Weak ETag: gzhttp keeps the original ETag on compressed variants, and a
	// weak validator legitimately covers encoding-equivalent representations.
	// http.ServeContent uses weak comparison for If-None-Match, so 304 works.
	indexETag := `W/"` + hex.EncodeToString(sum[:8]) + `"`

	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", indexETag)
		// Zero modtime disables the If-Modified-Since branch; conditional
		// requests go purely through If-None-Match -> 304.
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indexHTML))
	}

	staticFiles := http.FileServerFS(distFS)

	// fallback merges the previous NoRoute + static.Serve semantics:
	// unknown /api paths get the JSON 404 envelope, real files at the dist
	// root (favicon.ico) are served with no-cache, everything else is the SPA
	// fallback to index.html.
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			writeJSON(w, http.StatusNotFound, &notFoundResponse{
				Code:  http.StatusNotFound,
				Msg:   "not found",
				LogID: logs.GetLogIDFromContext(r.Context()),
			})
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && name != "index.html" {
			if info, statErr := fs.Stat(distFS, name); statErr == nil && !info.IsDir() {
				// Root-level files (favicon.ico) come from frontend/public
				// without a content hash, so they must not be immutable.
				w.Header().Set("Cache-Control", "no-cache")
				staticFiles.ServeHTTP(w, r)
				return
			}
		}
		serveIndex(w, r)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.Healthz)
	mux.HandleFunc("GET /readyz", server.Readyz)
	mux.HandleFunc("GET /metrics", server.Metrics)
	mux.HandleFunc("GET /api/get_data", server.GetData)
	mux.Handle("GET /assets/", immutableCache(staticFiles))
	// Method-independent catch-all: the ServeMux can never answer 405, so
	// e.g. POST /api/get_data lands here and yields the JSON 404 envelope.
	mux.Handle("/", fallback)

	inner := apiLogContext(recovery(mux))
	return gzipSkipProbes(newGzipWrapper()(inner), inner)
}
