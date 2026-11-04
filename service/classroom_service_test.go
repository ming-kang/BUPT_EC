package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

// campusClient answers QueryCampus per campus ID; nil entries succeed.
func campusClient(failures map[string]error) *mockJWClient {
	return &mockJWClient{
		queryCampus: func(_ context.Context, _ string, campusID string, _ string) ([]model.JWClassInfo, error) {
			if err := failures[campusID]; err != nil {
				return nil, err
			}
			return tokenTestRows(campusID), nil
		},
	}
}

func mustRefresh(t *testing.T, svc *ClassroomService) {
	t.Helper()
	if err := svc.refresh(context.Background()); err != nil {
		t.Fatalf("refresh() error = %v", err)
	}
}

func TestRefreshPublishesFullSnapshot(t *testing.T) {
	svc, _ := newTestService(t, campusClient(nil))
	mustRefresh(t, svc)

	got, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if got.Date != "2026-09-01" || got.Stale || got.Error != nil || len(got.PartialCampuses) != 0 {
		t.Fatalf("snapshot metadata = %+v", got)
	}
	if !got.ExpiresAt.Equal(testNoon.Add(freshTTL)) || !got.StaleUntil.Equal(endOfDay(testNoon)) {
		t.Fatalf("expiry = %v / %v", got.ExpiresAt, got.StaleUntil)
	}
	for _, id := range []string{"01", "04"} {
		if len(requireCampusByID(t, got.Campuses, id).Buildings) == 0 {
			t.Fatalf("campus %s has no buildings", id)
		}
	}
	if !svc.Ready() {
		t.Fatal("Ready() = false after a successful refresh")
	}
}

func TestPartialRefreshKeepsPreviousCampusData(t *testing.T) {
	failures := map[string]error{}
	svc, clock := newTestService(t, campusClient(failures))
	mustRefresh(t, svc)
	previous := requireCampusByID(t, svc.cache.Load().Campuses, "04")

	clock.Advance(time.Minute)
	failures["04"] = errors.New("campus down")
	mustRefresh(t, svc)

	got, _ := svc.GetTodayClassrooms(context.Background())
	if len(got.PartialCampuses) != 1 || got.PartialCampuses[0] != "04" {
		t.Fatalf("PartialCampuses = %v, want [04]", got.PartialCampuses)
	}
	if got.Error == nil || got.Error.Message != partialCampusErrorMessage {
		t.Fatalf("Error = %+v, want partial warning", got.Error)
	}
	if kept := requireCampusByID(t, got.Campuses, "04"); len(kept.Buildings) != len(previous.Buildings) {
		t.Fatalf("failed campus did not keep previous data: %+v", kept)
	}
	select {
	case <-svc.kick:
	default:
		t.Fatal("a fresh partial snapshot should nudge the refresh loop")
	}
}

func TestPartialRefreshWithoutPreviousDataUsesEmptyCampus(t *testing.T) {
	svc, _ := newTestService(t, campusClient(map[string]error{"04": errors.New("down")}))
	mustRefresh(t, svc)
	empty := requireCampusByID(t, svc.cache.Load().Campuses, "04")
	if empty.Name != "沙河" || empty.Buildings == nil || empty.Nodes == nil || len(empty.Buildings) != 0 {
		t.Fatalf("empty campus = %#v, want named campus with empty (non-nil) slices", empty)
	}
}

func TestTotalFailureKeepsCacheAndReturnsError(t *testing.T) {
	failures := map[string]error{}
	svc, _ := newTestService(t, campusClient(failures))
	mustRefresh(t, svc)
	before := svc.cache.Load()

	failures["01"], failures["04"] = errors.New("a"), errors.New("b")
	if err := svc.refresh(context.Background()); err == nil {
		t.Fatal("refresh() error = nil, want joined failure")
	}
	if svc.cache.Load() != before {
		t.Fatal("total failure replaced the published snapshot")
	}
}

func TestAuthRejectionLogsInOnceAndRetries(t *testing.T) {
	var logins atomic.Int32
	client := &mockJWClient{
		login: func(context.Context, string) (string, error) {
			return []string{"", "old", "new"}[logins.Add(1)], nil
		},
		queryCampus: func(_ context.Context, _ string, campusID string, token string) ([]model.JWClassInfo, error) {
			if token != "new" {
				return nil, newJWError(jwErrorAuth, "jw query", nil, "expired")
			}
			return tokenTestRows(campusID), nil
		},
	}
	svc, _ := newTestService(t, client)
	mustRefresh(t, svc)
	if got := logins.Load(); got != 2 {
		t.Fatalf("logins = %d, want 2 (initial + one shared recovery)", got)
	}
	if svc.cache.Load().Error != nil {
		t.Fatal("retried refresh should be complete")
	}
}

func TestLoginFailureFailsEveryCampus(t *testing.T) {
	loginErr := newJWError(jwErrorLogin, "jw login", nil, "bad password")
	client := campusClient(nil)
	client.login = func(context.Context, string) (string, error) { return "", loginErr }
	svc, _ := newTestService(t, client)
	if err := svc.refresh(context.Background()); !errors.Is(err, loginErr) {
		t.Fatalf("refresh() error = %v, want login error", err)
	}
}

func TestPreviousDayCacheIsNotUsable(t *testing.T) {
	svc, clock := newTestService(t, campusClient(nil))
	mustRefresh(t, svc)
	clock.Advance(12 * time.Hour)
	if svc.Ready() {
		t.Fatal("Ready() = true for yesterday's snapshot")
	}
	svc.coldWait = time.Millisecond
	if _, err := svc.GetTodayClassrooms(context.Background()); !errors.Is(err, ErrRefreshWaitTimeout) {
		t.Fatalf("GetTodayClassrooms() error = %v, want cold wait timeout", err)
	}
}

func TestStaleResponseIsMarkedAndReportsLastFailure(t *testing.T) {
	svc, clock := newTestService(t, campusClient(nil))
	mustRefresh(t, svc)
	clock.Advance(freshTTL + time.Second)

	got, _ := svc.GetTodayClassrooms(context.Background())
	if !got.Stale || got.Error != nil {
		t.Fatalf("stale response = stale %v error %+v", got.Stale, got.Error)
	}
	if svc.cache.Load().Stale {
		t.Fatal("stale decoration mutated the published snapshot")
	}

	svc.lastErr = newJWError(jwErrorQuery, "jw query", nil, "down")
	got, _ = svc.GetTodayClassrooms(context.Background())
	if got.Error == nil || got.Error.Type != string(jwErrorQuery) {
		t.Fatalf("stale error = %+v, want jw_query_failed", got.Error)
	}
}

func TestColdRequestWaitsForTheRefreshLoop(t *testing.T) {
	svc, _ := newTestService(t, campusClient(nil))
	startRun(t, svc)
	got, err := svc.GetTodayClassrooms(context.Background())
	if err != nil || got == nil {
		t.Fatalf("GetTodayClassrooms() = %v, %v", got, err)
	}
}

func TestBackoffRejectsColdRequestsAndIgnoresEarlyNudges(t *testing.T) {
	var queries atomic.Int32
	failing := true
	var mu sync.Mutex
	client := &mockJWClient{queryCampus: func(_ context.Context, _ string, campusID string, _ string) ([]model.JWClassInfo, error) {
		queries.Add(1)
		mu.Lock()
		defer mu.Unlock()
		if failing {
			return nil, newJWError(jwErrorQuery, "jw query", nil, "down")
		}
		return tokenTestRows(campusID), nil
	}}
	svc, clock := newTestService(t, client)
	startRun(t, svc)
	waitFor(t, time.Second, func() bool { return svc.lastError() != nil })

	if _, err := svc.GetTodayClassrooms(context.Background()); !isJWErrorKind(err, jwErrorQuery) {
		t.Fatalf("cold request during backoff error = %v, want last refresh error", err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := queries.Load(); got != 2 {
		t.Fatalf("queries = %d, want no refresh during backoff", got)
	}

	mu.Lock()
	failing = false
	mu.Unlock()
	clock.Advance(failureBackoff(1))
	if _, err := svc.GetTodayClassrooms(context.Background()); err != nil {
		t.Fatalf("request after backoff error = %v", err)
	}
}

func TestRunStopsInFlightRefreshOnCancel(t *testing.T) {
	started := make(chan struct{}, 2)
	client := &mockJWClient{queryCampus: func(ctx context.Context, _ string, _ string, _ string) ([]model.JWClassInfo, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	svc, _ := newTestService(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}

func TestFailureBackoffLadder(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for i, w := range want {
		if got := failureBackoff(i + 1); got != w {
			t.Fatalf("failureBackoff(%d) = %v, want %v", i+1, got, w)
		}
	}
}
