package service

import (
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestRuntimeStatusCacheStaleOnlyWhenPastFreshTTL(t *testing.T) {
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{Clock: clock})

	seedCache(t, svc, &model.TodayClassrooms{
		Date:       fixed.Format("2006-01-02"),
		ExpiresAt:  fixed.Add(time.Minute),
		StaleUntil: endOfDay(fixed),
	})

	status := svc.GetRuntimeStatus()
	if !status.CacheAvailable || !status.CacheFresh || status.CacheStale {
		t.Fatalf("fresh cache status = %#v, want fresh and not stale", status)
	}

	seedCache(t, svc, &model.TodayClassrooms{
		Date:       fixed.Format("2006-01-02"),
		ExpiresAt:  fixed.Add(-time.Minute),
		StaleUntil: endOfDay(fixed),
	})

	status = svc.GetRuntimeStatus()
	if !status.CacheAvailable || status.CacheFresh || !status.CacheStale {
		t.Fatalf("expired-but-usable cache status = %#v, want stale", status)
	}
}
