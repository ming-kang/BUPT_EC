package cache

import (
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

var (
	GlobalCache *gocache.Cache
	initOnce    sync.Once
)

func InitCache() {
	initOnce.Do(func() {
		GlobalCache = gocache.New(5*time.Minute, 1*time.Minute)
	})
}

func GetCache(key string) (interface{}, bool) {
	InitCache()
	return GlobalCache.Get(key)
}

func SetCache(key string, value interface{}, expiration time.Duration) {
	InitCache()
	GlobalCache.Set(key, value, expiration)
}

func DeleteCache(key string) {
	InitCache()
	GlobalCache.Delete(key)
}
