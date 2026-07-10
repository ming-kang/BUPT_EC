package service

import (
	"time"
)

type RuntimeStatus struct {
	LastLoginSuccessAt   *time.Time `json:"last_login_success_at,omitempty"`
	LastLoginError       string     `json:"last_login_error,omitempty"`
	LastRefreshSuccessAt *time.Time `json:"last_refresh_success_at,omitempty"`
	LastRefreshWarning   string     `json:"last_refresh_warning,omitempty"`
	LastRefreshError     string     `json:"last_refresh_error,omitempty"`
	CacheAvailable       bool       `json:"cache_available"`
	CacheFresh           bool       `json:"cache_fresh"`
	CacheStale           bool       `json:"cache_stale"`
	CachePartial         bool       `json:"cache_partial"`
	PartialCampuses      []string   `json:"partial_campuses,omitempty"`
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
	s.status.LastRefreshWarning = ""
	s.status.LastRefreshError = ""
}

func (s *ClassroomService) recordRefreshPartial(at time.Time) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.LastRefreshSuccessAt = cloneTime(at)
	s.status.LastRefreshWarning = partialCampusErrorMessage
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
		// CacheStale means usable but past the fresh TTL (not merely "within StaleUntil").
		status.CacheStale = !status.CacheFresh && now.Before(cached.StaleUntil)
		status.CachePartial = len(cached.PartialCampuses) > 0
		status.PartialCampuses = append([]string(nil), cached.PartialCampuses...)
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
