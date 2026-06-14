package main

import (
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	Init()
	r := gin.Default()
	SetRouter(r)
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if err := r.Run(addr); err != nil {
		panic(err)
	}
}
