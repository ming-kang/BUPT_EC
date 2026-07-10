package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestNextWarmupDelayRetriesBackoffAcrossMidnight(t *testing.T) {
	now := time.Date(2026, 7, 9, 23, 59, 50, 0, businessLocation)
	nextAllowed := time.Date(2026, 7, 10, 0, 0, 40, 0, businessLocation)

	got := nextWarmupDelay(now, warmupNoCache, nextAllowed, 1, 2*time.Second)
	if want := 50 * time.Second; got != want {
		t.Fatalf("nextWarmupDelay() = %v, want %v", got, want)
	}
}

func TestWarmupFailureDelayCapsAtFiveMinutes(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 1, want: 30 * time.Second},
		{failures: 2, want: time.Minute},
		{failures: 3, want: 2 * time.Minute},
		{failures: 4, want: 5 * time.Minute},
		{failures: 8, want: 5 * time.Minute},
	}
	for _, tt := range tests {
		if got := warmupFailureDelay(tt.failures); got != tt.want {
			t.Errorf("warmupFailureDelay(%d) = %v, want %v", tt.failures, got, tt.want)
		}
	}
}

func TestNextWarmupFailureCountResetsOnUsableOutcome(t *testing.T) {
	failed := classroomRefreshResult{kind: refreshFailed, err: fmt.Errorf("down")}
	if got := nextWarmupFailureCount(1, failed, true); got != 2 {
		t.Fatalf("failed count = %d, want 2", got)
	}
	if got := nextWarmupFailureCount(3, classroomRefreshResult{kind: refreshPartial}, true); got != 0 {
		t.Fatalf("partial count = %d, want 0", got)
	}
	if got := nextWarmupFailureCount(3, classroomRefreshResult{kind: refreshFull}, true); got != 0 {
		t.Fatalf("full count = %d, want 0", got)
	}
	if got := nextWarmupFailureCount(3, classroomRefreshResult{}, false); got != 3 {
		t.Fatalf("skipped count = %d, want 3", got)
	}
}

func TestNextWarmupDelayUsesCacheStatePolicy(t *testing.T) {
	noon := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	if got := nextWarmupDelay(noon, warmupPartialCache, time.Time{}, 0, time.Second); got != classroomFreshTTL {
		t.Fatalf("partial delay = %v, want %v", got, classroomFreshTTL)
	}

	nextAllowed := noon.Add(10 * time.Minute)
	if got := nextWarmupDelay(noon, warmupPartialCache, nextAllowed, 0, time.Second); got != 10*time.Minute {
		t.Fatalf("partial backoff delay = %v, want 10m", got)
	}

	beforeMidnight := time.Date(2026, 7, 9, 23, 59, 59, 0, businessLocation)
	if got := nextWarmupDelay(beforeMidnight, warmupPartialCache, time.Time{}, 0, 2*time.Second); got != 3*time.Second {
		t.Fatalf("partial midnight delay = %v, want 3s", got)
	}

	if got := nextWarmupDelay(beforeMidnight, warmupFullCache, time.Time{}, 99, 2*time.Second); got != 3*time.Second {
		t.Fatalf("full delay = %v, want next midnight plus jitter", got)
	}
}

func TestStartWarmupRunsImmediatelyAndStopsOnCancel(t *testing.T) {
	started := make(chan struct{})
	var once sync.Once
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			once.Do(func() { close(started) })
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
			}}, nil
		},
	}
	svc := newTestService(t, client)
	svc.warmupJitter = func() time.Duration { return 0 }
	ctx, cancel := context.WithCancel(context.Background())
	svc.StartWarmup(ctx)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initial warmup did not start immediately")
	}

	deadline := time.Now().Add(time.Second)
	for !svc.HasUsableTodayCache() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !svc.HasUsableTodayCache() {
		t.Fatal("initial warmup did not populate the cache")
	}

	cancel()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := svc.WaitBackground(waitCtx); err != nil {
		t.Fatalf("WaitBackground() error = %v", err)
	}
}

func TestStartWarmupWithCanceledContextDoesNotRefresh(t *testing.T) {
	called := make(chan struct{})
	var once sync.Once
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			once.Do(func() { close(called) })
			return nil, newJWError(jwErrorQuery, "jw query", nil, "unexpected")
		},
	}
	svc := newTestService(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.StartWarmup(ctx)
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := svc.WaitBackground(waitCtx); err != nil {
		t.Fatalf("WaitBackground() error = %v", err)
	}
	select {
	case <-called:
		t.Fatal("canceled warmup started a refresh")
	default:
	}
}

func TestStartWarmupSecondCallIsNoOp(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			once.Do(func() { close(started) })
			select {
			case <-release:
				return nil, newJWError(jwErrorQuery, "jw query", nil, "stopped")
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	svc := newTestService(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartWarmup(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("warmup did not start")
	}

	svc.backgroundMu.Lock()
	firstDone := svc.warmupDone
	svc.backgroundMu.Unlock()
	svc.StartWarmup(context.Background())
	svc.backgroundMu.Lock()
	secondDone := svc.warmupDone
	svc.backgroundMu.Unlock()
	if firstDone != secondDone {
		t.Fatal("second StartWarmup call created another scheduler")
	}

	cancel()
	close(release)
}

func TestWaitBackgroundPreventsNewRefreshWorkers(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			once.Do(func() { close(started) })
			select {
			case <-release:
				return nil, newJWError(jwErrorQuery, "jw query", nil, "stopped")
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	svc := newTestService(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	svc.StartWarmup(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("warmup did not start")
	}
	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	waitErr := make(chan error, 1)
	go func() { waitErr <- svc.WaitBackground(waitCtx) }()

	deadline := time.Now().Add(time.Second)
	for {
		svc.backgroundMu.Lock()
		stopping := svc.backgroundStopping
		svc.backgroundMu.Unlock()
		if stopping {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("WaitBackground did not stop new workers")
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := svc.startClassroomRefresh(context.Background(), svc.now()); ok {
		t.Fatal("refresh started after background shutdown began")
	}

	close(release)
	if err := <-waitErr; err != nil {
		t.Fatalf("WaitBackground() error = %v", err)
	}
}
