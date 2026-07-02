package service

import (
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

func (s *ClassroomService) recordLoginSuccess(at time.Time) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.LastLoginSuccessAt = cloneTime(at)
	s.status.LastLoginError = ""
}

func (s *ClassroomService) recordLoginFailure(err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.LastLoginError = SafeErrorMessage(err)
}

func (s *ClassroomService) recordRefreshSuccess(at time.Time) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.LastRefreshSuccessAt = cloneTime(at)
	s.status.LastRefreshError = ""
}

func (s *ClassroomService) recordRefreshFailure(err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.LastRefreshError = SafeErrorMessage(err)
}

func (s *ClassroomService) GetRuntimeStatus() RuntimeStatus {
	status := s.snapshotRuntimeStatus()
	now := s.now()
	if cached, ok := s.getCachedTodayClassrooms(); ok {
		status.CacheAvailable = true
		status.CacheFresh = !cached.ExpiresAt.Before(now)
		status.CacheStale = now.Before(cached.StaleUntil)
		status.CacheDate = cached.Date
	}
	return status
}

func (s *ClassroomService) HasUsableTodayCache() bool {
	cached, ok := s.getCachedTodayClassrooms()
	return ok && s.now().Before(cached.StaleUntil)
}

func (s *ClassroomService) snapshotRuntimeStatus() RuntimeStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	status := s.status
	if s.status.LastLoginSuccessAt != nil {
		status.LastLoginSuccessAt = cloneTime(*s.status.LastLoginSuccessAt)
	}
	if s.status.LastRefreshSuccessAt != nil {
		status.LastRefreshSuccessAt = cloneTime(*s.status.LastRefreshSuccessAt)
	}
	return status
}

func cloneTime(t time.Time) *time.Time {
	copy := t
	return &copy
}
