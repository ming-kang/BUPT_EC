package main

import (
	"BUPT_EC/logs"
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

//go:embed frontend/dist
var f embed.FS

type embedFileSystem struct {
	http.FileSystem
}

func (e embedFileSystem) Exists(prefix string, path string) bool {
	file, err := e.Open(path)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

func EmbedFolder(fsEmbed embed.FS, targetPath string) static.ServeFileSystem {
	fsys, err := fs.Sub(fsEmbed, targetPath)
	if err != nil {
		panic(err)
	}
	return embedFileSystem{
		FileSystem: http.FS(fsys),
	}
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

// apiLogContextMiddleware attaches exactly one request log_id for /api paths,
// including unknown routes handled by NoRoute. Non-API traffic is left alone.
func apiLogContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAPIPath(c.Request.URL.Path) {
			logs.SetNewContextForGinContext(c)
		}
		c.Next()
	}
}

func writeAPINotFound(c *gin.Context) {
	ctx := logs.GetContextFromGinContext(c)
	c.JSON(http.StatusNotFound, gin.H{
		"code":   http.StatusNotFound,
		"msg":    "not found",
		"log_id": logs.GetLogIDFromContext(ctx),
	})
}

func (server *HTTPServer) RegisterRoutes(r *gin.Engine) {
	r.Use(gzipMiddleware())
	r.Use(apiLogContextMiddleware())

	r.GET("/healthz", server.Healthz)
	r.GET("/readyz", server.Readyz)
	r.GET("/metrics", server.Metrics)

	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/get_data", server.GetData)
	}

	r.Use(static.Serve("/", EmbedFolder(f, "frontend/dist")))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if isAPIPath(path) {
			writeAPINotFound(c)
			return
		}
		file, err := f.Open("frontend/dist/index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

// acceptsGzip implements Accept-Encoding negotiation for the gzip coding only.
// It is case-insensitive, honors q-values, treats malformed tokens as rejected,
// and allows gzip via "*" only when no explicit gzip token is present.
func acceptsGzip(header string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}

	var (
		gzipQ     float64
		hasGzip   bool
		starQ     float64
		hasStar   bool
		sawTokens bool
	)

	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sawTokens = true
		coding, params, _ := strings.Cut(part, ";")
		coding = strings.ToLower(strings.TrimSpace(coding))
		if coding == "" {
			continue
		}

		q := 1.0
		for _, param := range strings.Split(params, ";") {
			param = strings.TrimSpace(param)
			if param == "" {
				continue
			}
			key, value, ok := strings.Cut(param, "=")
			if !ok {
				q = 0
				break
			}
			if strings.ToLower(strings.TrimSpace(key)) != "q" {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				q = 0
				break
			}
			q = parsed
		}

		switch coding {
		case "gzip":
			hasGzip = true
			gzipQ = q
		case "*":
			hasStar = true
			starQ = q
		}
	}

	if !sawTokens {
		return false
	}
	if hasGzip {
		return gzipQ > 0
	}
	return hasStar && starQ > 0
}

func appendVaryAcceptEncoding(header http.Header) {
	const token = "Accept-Encoding"
	existing := header.Values("Vary")
	for _, value := range existing {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return
			}
		}
	}
	header.Add("Vary", token)
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (w gzipResponseWriter) Write(data []byte) (int, error) {
	w.Header().Del("Content-Length")
	return w.writer.Write(data)
}

func (w gzipResponseWriter) WriteString(data string) (int, error) {
	w.Header().Del("Content-Length")
	return w.writer.Write([]byte(data))
}

func gzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" || c.Request.URL.Path == "/readyz" {
			c.Next()
			return
		}
		if !acceptsGzip(c.GetHeader("Accept-Encoding")) {
			c.Next()
			return
		}

		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		c.Header("Content-Encoding", "gzip")
		appendVaryAcceptEncoding(c.Writer.Header())
		c.Writer.Header().Del("Content-Length")
		c.Writer = gzipResponseWriter{ResponseWriter: c.Writer, writer: gz}
		c.Next()
	}
}
