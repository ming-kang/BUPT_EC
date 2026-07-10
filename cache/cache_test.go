package cache

import (
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

func TestNewReturnsIndependentCachesWithDefaultExpiration(t *testing.T) {
	first := New()
	second := New()
	if first == second {
		t.Fatal("New() returned the same cache instance")
	}

	startedAt := time.Now()
	first.Set("key", "value", gocache.DefaultExpiration)
	if _, ok := second.Get("key"); ok {
		t.Fatal("cache value leaked into a separate instance")
	}
	item, ok := first.Items()["key"]
	if !ok {
		t.Fatal("first cache is missing the inserted value")
	}
	expiresAt := time.Unix(0, item.Expiration)
	if expiresAt.Before(startedAt.Add(4*time.Minute+59*time.Second)) || expiresAt.After(startedAt.Add(5*time.Minute+time.Second)) {
		t.Fatalf("default expiration = %v, want approximately five minutes", expiresAt.Sub(startedAt))
	}
}
