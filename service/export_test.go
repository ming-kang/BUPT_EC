package service

// export_test.go centralizes every white-box seam unit tests use to reach
// ClassroomService/TokenManager internals. Tests must go through these seed
// helpers instead of touching unexported fields directly, so later refactors
// of the cache storage or refresh coordination only change this one file.

import (
	"testing"
	"time"

	"BUPT_EC/service/model"
)

// seedCache installs a prebuilt payload as the current today-cache entry.
// The storage backend is an implementation detail owned by this helper.
func seedCache(t *testing.T, svc *ClassroomService, today *model.TodayClassrooms) {
	t.Helper()
	if today == nil || today.Date == "" {
		t.Fatal("seedCache requires a payload with a business date")
	}
	svc.todayCache.Store(today)
	svc.updateCachedDataJSON(today)
}

// completeRefresh drives finishClassroomRefresh exactly like a production
// worker completing one refresh attempt with the given result. It installs an
// in-flight attempt under refreshMu (same lock ordering as the coordinator)
// and returns after the attempt's done channel is closed.
func completeRefresh(svc *ClassroomService, result classroomRefreshResult) {
	attempt := &classroomRefreshAttempt{done: make(chan struct{})}
	svc.refreshMu.Lock()
	svc.refreshInFlight = true
	svc.refreshAttempt = attempt
	svc.refreshMu.Unlock()
	svc.finishClassroomRefresh(attempt, result)
	<-attempt.done
}

// forceFailureState pre-positions the total-failure ladder and the backoff
// deadline without completing a refresh.
func forceFailureState(svc *ClassroomService, consecutive int, next time.Time) {
	svc.refreshMu.Lock()
	defer svc.refreshMu.Unlock()
	svc.consecutiveTotalFailures = consecutive
	svc.nextRefreshAllowed = next
}

// backoffState snapshots the refresh coordinator state under refreshMu.
func backoffState(svc *ClassroomService) (next time.Time, consecutive int, lastErr error) {
	svc.refreshMu.Lock()
	defer svc.refreshMu.Unlock()
	return svc.nextRefreshAllowed, svc.consecutiveTotalFailures, svc.lastRefreshErr
}

// installToken seeds an already-cached login token into the service's
// TokenManager, as if a previous login had stored it.
func installToken(svc *ClassroomService, token string) {
	svc.tokenManager.setToken(token, tokenSourceLogin)
}

// tokenManagerTestOptions configures newTokenManagerForTest. Zero values pick
// safe defaults: NoopMetrics and the production system clock.
type tokenManagerTestOptions struct {
	override string
	metrics  RuntimeMetrics
	clock    Clock
}

// newTokenManagerForTest builds a standalone TokenManager for white-box tests.
// Unlike a bare &TokenManager{} literal it always injects non-nil metrics and
// clock, so production code may drop its nil guards without breaking tests.
func newTokenManagerForTest(client JWClient, opts tokenManagerTestOptions) *TokenManager {
	metrics := opts.metrics
	if metrics == nil {
		metrics = NoopMetrics{}
	}
	clock := opts.clock
	if clock == nil {
		clock = systemClock{}
	}
	return &TokenManager{
		jwClient:      client,
		overrideToken: opts.override,
		clock:         clock,
		metrics:       metrics,
	}
}

// tokenState snapshots the current token provenance under the manager lock.
func tokenState(m *TokenManager) (source tokenSource, overrideInvalidated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tokenSource, m.overrideInvalidated
}

// invalidateOverride marks the startup override token as rejected, as if a
// prior auth recovery had invalidated it.
func invalidateOverride(m *TokenManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrideInvalidated = true
}

// schedulerDone returns the current scheduler's done channel (nil when Run
// has not been called).
func schedulerDone(svc *ClassroomService) chan struct{} {
	svc.backgroundMu.Lock()
	defer svc.backgroundMu.Unlock()
	return svc.schedulerDone
}

// isLifecycleCanceled reports whether the service lifecycle is canceled,
// mirroring the worker gate in startClassroomRefresh.
func isLifecycleCanceled(svc *ClassroomService) bool {
	svc.backgroundMu.Lock()
	defer svc.backgroundMu.Unlock()
	return svc.lifecycleCtx != nil && svc.lifecycleCtx.Err() != nil
}
