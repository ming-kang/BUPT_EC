package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

func New() *gocache.Cache {
	return gocache.New(5*time.Minute, 1*time.Minute)
}
