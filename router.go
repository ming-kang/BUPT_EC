package main

import (
	"BUPT_EC/logs"
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"net/http"
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

func (server *HTTPServer) RegisterRoutes(r *gin.Engine) {
	r.Use(gzipMiddleware())

	r.GET("/healthz", server.Healthz)
	r.GET("/readyz", server.Readyz)

	apiGroup := r.Group("/api").Use(logs.SetNewContextForGinContext)
	{
		apiGroup.GET("/get_data", server.GetData)
	}

	r.Use(static.Serve("/", EmbedFolder(f, "frontend/dist")))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "msg": "not found"})
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
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer.Header().Del("Content-Length")
		c.Writer = gzipResponseWriter{ResponseWriter: c.Writer, writer: gz}
		c.Next()
	}
}
