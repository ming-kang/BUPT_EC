package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

var GlobalCache *gocache.Cache

func InitCache() {
	GlobalCache = gocache.New(5*time.Minute, 1*time.Minute)
}

func GetCache(key string) (interface{}, bool) {
	return GlobalCache.Get(key)
}

func SetCache(key string, value interface{}, expiration time.Duration) {
	GlobalCache.Set(key, value, expiration)
}

func DeleteCache(key string) {
	GlobalCache.Delete(key)
}
