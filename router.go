package main

import (
	"BUPT_EC/logs"
	"embed"
	"io/fs"
	"net/http"

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

func SetRouter(r *gin.Engine) {
	r.GET("/healthz", Healthz)
	r.GET("/readyz", Readyz)

	apiGroup := r.Group("/api").Use(logs.SetNewContextForGinContext)
	{
		apiGroup.GET("/get_data", GetData)
	}

	r.Use(static.Serve("/", EmbedFolder(f, "frontend/dist")))
}
