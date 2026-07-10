package cache

import (
	"time"

	"BUPT_EC/service/model"

	gocache "github.com/patrickmn/go-cache"
)

// todayKey is an internal storage key. Business policy never depends on it.
const todayKey = "TODAY_CLASSROOMS_CACHE"

// TodayClassroomsStore is a process-local typed cache for today's classrooms.
// The default go-cache TTL is NoExpiration so it cannot masquerade as the
// product freshness window; every Store must pass an explicit expiration.
type TodayClassroomsStore struct {
	inner *gocache.Cache
}

// New returns an independent typed cache instance.
func New() *TodayClassroomsStore {
	return &TodayClassroomsStore{
		// Cleanup interval is housekeeping only; business expiry is always explicit.
		inner: gocache.New(gocache.NoExpiration, time.Minute),
	}
}

// Load returns the cached *model.TodayClassrooms value when present.
func (s *TodayClassroomsStore) Load() (*model.TodayClassrooms, bool) {
	if s == nil || s.inner == nil {
		return nil, false
	}
	raw, ok := s.inner.Get(todayKey)
	if !ok || raw == nil {
		return nil, false
	}
	value, ok := raw.(*model.TodayClassrooms)
	if !ok || value == nil {
		return nil, false
	}
	return value, true
}

// Store writes value with an explicit expiration (typically until StaleUntil).
func (s *TodayClassroomsStore) Store(value *model.TodayClassrooms, expiration time.Duration) {
	if s == nil || s.inner == nil || value == nil {
		return
	}
	s.inner.Set(todayKey, value, expiration)
}
