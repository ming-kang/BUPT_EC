package service

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestTotalFailureBackoffBaseLadder(t *testing.T) {
	want := []time.Duration{
		30 * time.Second,
		time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		5 * time.Minute,
	}
	for i, expected := range want {
		got := totalFailureBackoffBase(i + 1)
		if got != expected {
			t.Fatalf("totalFailureBackoffBase(%d) = %v, want %v", i+1, got, expected)
		}
	}
	if totalFailureBackoffBase(0) != 30*time.Second {
		t.Fatalf("totalFailureBackoffBase(0) = %v, want 30s", totalFailureBackoffBase(0))
	}
}

func TestJitteredBackoffBoundsAndInvalidSamples(t *testing.T) {
	base30 := 30 * time.Second
	// ±min(3s, 5s) = ±3s
	if got := jitteredBackoff(base30, 0); got != 27*time.Second {
		t.Fatalf("sample=0 delay = %v, want 27s", got)
	}
	if got := jitteredBackoff(base30, 0.5); got != 30*time.Second {
		t.Fatalf("sample=0.5 delay = %v, want 30s", got)
	}
	if got := jitteredBackoff(base30, 1); got != 33*time.Second {
		t.Fatalf("sample=1 delay = %v, want 33s", got)
	}

	base5m := 5 * time.Minute
	// ±min(30s, 5s) = ±5s
	if got := jitteredBackoff(base5m, 0); got != 5*time.Minute-5*time.Second {
		t.Fatalf("5m sample=0 delay = %v", got)
	}
	if got := jitteredBackoff(base5m, 0.5); got != 5*time.Minute {
		t.Fatalf("5m sample=0.5 delay = %v", got)
	}
	if got := jitteredBackoff(base5m, 1); got != 5*time.Minute+5*time.Second {
		t.Fatalf("5m sample=1 delay = %v", got)
	}

	// Invalid samples clamp / fall back without negative delay.
	for _, sample := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -3, 2} {
		got := jitteredBackoff(base30, sample)
		minDelay := jitteredBackoff(base30, 0)
		maxDelay := jitteredBackoff(base30, 1)
		if got < minDelay || got > maxDelay {
			t.Fatalf("sample=%v delay %v outside [%v,%v]", sample, got, minDelay, maxDelay)
		}
		if got <= 0 {
			t.Fatalf("sample=%v produced non-positive delay %v", sample, got)
		}
	}
	// NaN/Inf → 0.5 mid-point.
	if got := jitteredBackoff(base30, math.NaN()); got != 30*time.Second {
		t.Fatalf("NaN sample delay = %v, want 30s", got)
	}
	if got := jitteredBackoff(base30, math.Inf(1)); got != 30*time.Second {
		t.Fatalf("+Inf sample delay = %v, want 30s", got)
	}
	// Out of range clamps to edges.
	if got := jitteredBackoff(base30, -1); got != 27*time.Second {
		t.Fatalf("negative sample delay = %v, want 27s", got)
	}
	if got := jitteredBackoff(base30, 2); got != 33*time.Second {
		t.Fatalf(">1 sample delay = %v, want 33s", got)
	}
	if got := jitteredBackoff(0, 0.5); got <= 0 {
		t.Fatalf("zero base must still yield positive delay, got %v", got)
	}
}

func TestFinishClassroomRefreshSamplesOnceAndAppliesJitter(t *testing.T) {
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	var samples atomic.Int32
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{
		Clock: clock,
		BackoffRandom: func() float64 {
			samples.Add(1)
			return 1 // max positive jitter
		},
	})

	attempt := &classroomRefreshAttempt{done: make(chan struct{})}
	svc.refreshInFlight = true
	svc.refreshAttempt = attempt
	svc.finishClassroomRefresh(attempt, classroomRefreshResult{
		kind: refreshFailed,
		err:  newJWError(jwErrorQuery, "jw query", nil, "down"),
	})
	if samples.Load() != 1 {
		t.Fatalf("random samples = %d, want 1", samples.Load())
	}
	want := fixed.Add(jitteredBackoff(totalFailureBackoffBase(1), 1))
	if !svc.nextRefreshAllowed.Equal(want) {
		t.Fatalf("nextRefreshAllowed = %v, want %v", svc.nextRefreshAllowed, want)
	}
}

func TestTotalFailureLadderEscalatesAndFullResets(t *testing.T) {
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{
		Clock:         clock,
		BackoffRandom: func() float64 { return 0.5 },
	})

	for i, wantBase := range []time.Duration{
		30 * time.Second,
		time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		5 * time.Minute,
	} {
		attempt := &classroomRefreshAttempt{done: make(chan struct{})}
		svc.refreshInFlight = true
		svc.refreshAttempt = attempt
		svc.finishClassroomRefresh(attempt, classroomRefreshResult{
			kind: refreshFailed,
			err:  newJWError(jwErrorQuery, "jw query", nil, "down"),
		})
		want := fixed.Add(wantBase)
		if !svc.nextRefreshAllowed.Equal(want) {
			t.Fatalf("failure %d next = %v, want %v", i+1, svc.nextRefreshAllowed, want)
		}
	}
	if svc.consecutiveTotalFailures != 5 {
		t.Fatalf("consecutive = %d, want 5", svc.consecutiveTotalFailures)
	}

	// Partial does not escalate ladder or use total jitter.
	partialAttempt := &classroomRefreshAttempt{done: make(chan struct{})}
	svc.refreshInFlight = true
	svc.refreshAttempt = partialAttempt
	beforeConsecutive := svc.consecutiveTotalFailures
	svc.finishClassroomRefresh(partialAttempt, classroomRefreshResult{
		kind:  refreshPartial,
		value: &model.TodayClassrooms{},
	})
	if svc.consecutiveTotalFailures != beforeConsecutive {
		t.Fatalf("partial changed consecutive from %d to %d", beforeConsecutive, svc.consecutiveTotalFailures)
	}
	if want := fixed.Add(staleRefreshBackoff); !svc.nextRefreshAllowed.Equal(want) {
		t.Fatalf("partial next = %v, want %v", svc.nextRefreshAllowed, want)
	}

	fullAttempt := &classroomRefreshAttempt{done: make(chan struct{})}
	svc.refreshInFlight = true
	svc.refreshAttempt = fullAttempt
	svc.finishClassroomRefresh(fullAttempt, classroomRefreshResult{
		kind:  refreshFull,
		value: &model.TodayClassrooms{},
	})
	if svc.consecutiveTotalFailures != 0 || !svc.nextRefreshAllowed.IsZero() {
		t.Fatalf("full did not reset: consecutive=%d next=%v", svc.consecutiveTotalFailures, svc.nextRefreshAllowed)
	}
}

type countingSuppressedMetrics struct {
	NoopMetrics
	suppressed atomic.Int32
}

func (m *countingSuppressedMetrics) ObserveRefreshSuppressed() {
	m.suppressed.Add(1)
}

func TestRefreshSuppressedDoesNotStartWorkerAndCountsOncePerCall(t *testing.T) {
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	metrics := &countingSuppressedMetrics{}
	var queryCalls atomic.Int32
	svc := newTestServiceWithOptions(t, &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			queryCalls.Add(1)
			return nil, newJWError(jwErrorQuery, "jw query", nil, "down")
		},
	}, ClassroomServiceOptions{
		Clock:         clock,
		Metrics:       metrics,
		BackoffRandom: func() float64 { return 0.5 },
	})

	// Install a total-failure backoff window without starting a worker.
	failAttempt := &classroomRefreshAttempt{done: make(chan struct{})}
	svc.refreshInFlight = true
	svc.refreshAttempt = failAttempt
	svc.finishClassroomRefresh(failAttempt, classroomRefreshResult{
		kind: refreshFailed,
		err:  newJWError(jwErrorQuery, "jw query", nil, "down"),
	})

	attempt, started := svc.startClassroomRefresh(context.Background(), clock.Now())
	if started || attempt != nil {
		t.Fatalf("expected suppression, started=%v attempt=%v", started, attempt)
	}
	if metrics.suppressed.Load() != 1 {
		t.Fatalf("suppressed metric = %d, want 1", metrics.suppressed.Load())
	}
	if queryCalls.Load() != 0 {
		t.Fatalf("query calls during suppression = %d, want 0", queryCalls.Load())
	}

	// Second suppressed caller also records a suppression observation.
	_, started = svc.startClassroomRefresh(context.Background(), clock.Now())
	if started {
		t.Fatal("second caller should also be suppressed")
	}
	if metrics.suppressed.Load() != 2 {
		t.Fatalf("suppressed metric = %d, want 2", metrics.suppressed.Load())
	}
}

func TestConcurrentCallersShareNextRefreshAllowed(t *testing.T) {
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	var samples atomic.Int32
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{
		Clock: clock,
		BackoffRandom: func() float64 {
			samples.Add(1)
			return 0
		},
	})

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt := &classroomRefreshAttempt{done: make(chan struct{})}
			// Serialize state updates the same way production does (one finish per attempt).
			// Concurrent finish on different attempts still must keep consistent ladder under lock.
			svc.refreshMu.Lock()
			svc.refreshInFlight = true
			svc.refreshAttempt = attempt
			svc.refreshMu.Unlock()
			svc.finishClassroomRefresh(attempt, classroomRefreshResult{
				kind: refreshFailed,
				err:  newJWError(jwErrorQuery, "jw query", nil, "down"),
			})
		}()
	}
	wg.Wait()

	if samples.Load() != 8 {
		t.Fatalf("samples = %d, want 8 (one per completed failure)", samples.Load())
	}
	if svc.consecutiveTotalFailures != 8 {
		t.Fatalf("consecutive = %d, want 8", svc.consecutiveTotalFailures)
	}
	// Cap base is 5m; sample 0 → 5m - 5s.
	want := fixed.Add(jitteredBackoff(totalFailureBackoffBase(8), 0))
	if !svc.nextRefreshAllowed.Equal(want) {
		t.Fatalf("next = %v, want %v", svc.nextRefreshAllowed, want)
	}

	// All callers see the same coordinator deadline.
	var readers sync.WaitGroup
	results := make(chan time.Time, 16)
	for range 16 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			results <- svc.nextRefreshAllowedAt()
		}()
	}
	readers.Wait()
	close(results)
	for got := range results {
		if !got.Equal(want) {
			t.Fatalf("reader next = %v, want %v", got, want)
		}
	}
}

func TestBackoffCrossingMidnightRejectsOldCacheThenAllowsNewDayRefresh(t *testing.T) {
	// 23:55 + (5m + 5s max jitter) → nextAllowed 00:00:05 next day.
	beforeMidnight := time.Date(2026, 7, 9, 23, 55, 0, 0, businessLocation)
	// Still before nextAllowed (00:00:05) but already a new business day.
	afterMidnight := time.Date(2026, 7, 10, 0, 0, 1, 0, businessLocation)
	clock := newFakeClock(beforeMidnight)
	var queryDays []string
	var mu sync.Mutex
	var queryCalls atomic.Int32
	svc := newTestServiceWithOptions(t, &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			queryCalls.Add(1)
			mu.Lock()
			queryDays = append(queryDays, clock.Now().In(businessLocation).Format("2006-01-02"))
			mu.Unlock()
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N101(10)",
			}}, nil
		},
	}, ClassroomServiceOptions{
		TokenOverride: "token",
		Clock:         clock,
		// Fourth failure base is 5m; sample=1 → +5s so nextAllowed is after midnight.
		BackoffRandom: func() float64 { return 1 },
	})

	// Seed yesterday cache, then force a total failure so backoff spans midnight.
	svc.cache.Store(&model.TodayClassrooms{
		Date:       beforeMidnight.Format("2006-01-02"),
		UpdatedAt:  beforeMidnight.Add(-time.Minute),
		ExpiresAt:  beforeMidnight.Add(-time.Second),
		StaleUntil: endOfDay(beforeMidnight),
		Campuses:   []model.CampusInfo{{ID: "01", Name: "西土城"}, {ID: "04", Name: "沙河"}},
	}, time.Hour)

	// Prior failures already at ladder step 3 so this completion uses the 5m base.
	svc.consecutiveTotalFailures = 3
	failAttempt := &classroomRefreshAttempt{done: make(chan struct{})}
	svc.refreshInFlight = true
	svc.refreshAttempt = failAttempt
	svc.finishClassroomRefresh(failAttempt, classroomRefreshResult{
		kind: refreshFailed,
		err:  newJWError(jwErrorQuery, "jw query", nil, "jw outage"),
	})
	nextAllowed := svc.nextRefreshAllowedAt()
	if !nextAllowed.After(endOfDay(beforeMidnight)) {
		t.Fatalf("expected nextAllowed %v to cross midnight %v", nextAllowed, endOfDay(beforeMidnight))
	}

	// Immediately after midnight, old cache is rejected; still inside backoff → no worker.
	clock.Set(afterMidnight)
	if cached, ok := svc.getCachedTodayClassroomsAt(clock.Now()); ok {
		t.Fatalf("expected yesterday cache rejected, got date=%s", cached.Date)
	}
	_, started := svc.startClassroomRefresh(context.Background(), clock.Now())
	if started {
		t.Fatal("refresh should stay suppressed before nextAllowed")
	}
	if queryCalls.Load() != 0 {
		t.Fatalf("queries during post-midnight suppression = %d", queryCalls.Load())
	}

	// Reach nextAllowed: request path can start a new-day refresh.
	clock.Set(nextAllowed)
	resp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms after nextAllowed error = %v", err)
	}
	if resp == nil || resp.Date != "2026-07-10" {
		t.Fatalf("expected new business day payload, got %#v", resp)
	}
	if queryCalls.Load() < 2 {
		t.Fatalf("expected campus queries after nextAllowed, calls=%d", queryCalls.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	for _, day := range queryDays {
		if day != "2026-07-10" {
			t.Fatalf("query day = %s, want 2026-07-10", day)
		}
	}
}

func TestFakeClockAndCoordinatorRace(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation))
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{
		Clock:         clock,
		BackoffRandom: func() float64 { return 0.5 },
	})

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clock.Advance(time.Millisecond)
			_ = clock.Now()
			_ = svc.now()
			_ = svc.nextRefreshAllowedAt()
			attempt := &classroomRefreshAttempt{done: make(chan struct{})}
			svc.refreshMu.Lock()
			svc.refreshInFlight = true
			svc.refreshAttempt = attempt
			svc.refreshMu.Unlock()
			svc.finishClassroomRefresh(attempt, classroomRefreshResult{
				kind:  refreshFull,
				value: &model.TodayClassrooms{},
			})
		}()
	}
	wg.Wait()
}
