package service

import (
	"sync"
	"time"
)

type RuntimeStatus struct {
	LastLoginSuccessAt   *time.Time `json:"last_login_success_at,omitempty"`
	LastLoginError       string     `json:"last_login_error,omitempty"`
	LastRefreshSuccessAt *time.Time `json:"last_refresh_success_at,omitempty"`
	LastRefreshError     string     `json:"last_refresh_error,omitempty"`
	CacheAvailable       bool       `json:"cache_available"`
	CacheFresh           bool       `json:"cache_fresh"`
	CacheStale           bool       `json:"cache_stale"`
	CacheDate            string     `json:"cache_date,omitempty"`
}

var (
	runtimeStatusMu sync.RWMutex
	runtimeStatus   RuntimeStatus
)

func recordLoginSuccess(at time.Time) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastLoginSuccessAt = cloneTime(at)
	runtimeStatus.LastLoginError = ""
}

func recordLoginFailure(err error) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastLoginError = SafeErrorMessage(err)
}

func recordRefreshSuccess(at time.Time) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastRefreshSuccessAt = cloneTime(at)
	runtimeStatus.LastRefreshError = ""
}

func recordRefreshFailure(err error) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastRefreshError = SafeErrorMessage(err)
}

func GetRuntimeStatus() RuntimeStatus {
	status := snapshotRuntimeStatus()
	now := nowFunc()
	if cached, ok := getCachedTodayClassrooms(); ok {
		status.CacheAvailable = true
		status.CacheFresh = !cached.ExpiresAt.Before(now)
		status.CacheStale = now.Before(cached.StaleUntil)
		status.CacheDate = cached.Date
	}
	return status
}

func HasUsableTodayCache() bool {
	cached, ok := getCachedTodayClassrooms()
	return ok && nowFunc().Before(cached.StaleUntil)
}

func snapshotRuntimeStatus() RuntimeStatus {
	runtimeStatusMu.RLock()
	defer runtimeStatusMu.RUnlock()
	status := runtimeStatus
	if runtimeStatus.LastLoginSuccessAt != nil {
		status.LastLoginSuccessAt = cloneTime(*runtimeStatus.LastLoginSuccessAt)
	}
	if runtimeStatus.LastRefreshSuccessAt != nil {
		status.LastRefreshSuccessAt = cloneTime(*runtimeStatus.LastRefreshSuccessAt)
	}
	return status
}

func cloneTime(t time.Time) *time.Time {
	copy := t
	return &copy
}
