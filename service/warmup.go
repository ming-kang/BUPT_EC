package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

const (
	warmupJitterMin   = time.Second
	warmupJitterRange = 4 * time.Second
)

type warmupCacheState int

const (
	warmupNoCache warmupCacheState = iota
	warmupPartialCache
	warmupFullCache
)

func randomWarmupJitter() time.Duration {
	return warmupJitterMin + time.Duration(rand.Int64N(int64(warmupJitterRange)))
}

func nextWarmupFailureCount(current int, result classroomRefreshResult, completed bool) int {
	if !completed {
		return current
	}
	if result.kind == refreshFailed || result.err != nil {
		return current + 1
	}
	return 0
}

func nextWarmupDelay(
	now time.Time,
	cacheState warmupCacheState,
	nextAllowed time.Time,
	failures int,
	midnightJitter time.Duration,
) time.Duration {
	if midnightJitter < 0 {
		midnightJitter = 0
	}
	nextMidnight := endOfDay(now).Add(midnightJitter)
	var target time.Time

	switch cacheState {
	case warmupFullCache:
		target = nextMidnight
	case warmupPartialCache:
		target = now.Add(classroomFreshTTL)
		if nextAllowed.After(target) {
			target = nextAllowed
		}
		if nextMidnight.Before(target) {
			target = nextMidnight
			if nextAllowed.After(target) {
				target = nextAllowed
			}
		}
	default:
		target = now.Add(backoffLadder(failures))
		if nextAllowed.After(target) {
			target = nextAllowed
		}
	}

	if delay := target.Sub(now); delay > 0 {
		return delay
	}
	return 0
}

// ErrAlreadyRunning is returned by Run when a scheduler was already started
// for this service. Each service runs at most one scheduler in its lifetime.
var ErrAlreadyRunning = errors.New("classroom service already running")

// Run starts at most one warmup scheduler and blocks until ctx is canceled.
// The first refresh is attempted immediately; a pre-canceled context starts
// no refresh worker. Run returns nil once the scheduler has exited. A second
// Run call on the same service returns ErrAlreadyRunning.
func (s *ClassroomService) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.backgroundMu.Lock()
	if s.lifecycleCtx != nil {
		s.backgroundMu.Unlock()
		return ErrAlreadyRunning
	}
	lifecycleCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.lifecycleCtx = lifecycleCtx
	s.lifecycleCancel = cancel
	s.schedulerDone = done
	s.backgroundMu.Unlock()

	// The token manager keeps a reference (not a copy of cancel) so shutdown
	// cancellation has a single owner: this service's lifecycle.
	s.tokenManager.bindLifecycle(lifecycleCtx)

	defer func() {
		// Cancel under backgroundMu so a concurrent startClassroomRefresh that
		// just passed the lifecycle gate cannot race with Shutdown's drain.
		s.backgroundMu.Lock()
		cancel()
		s.backgroundMu.Unlock()
		close(done)
	}()

	s.warmupLoop(lifecycleCtx)
	return nil
}

// isLifecycleCanceledLocked reports whether the service lifecycle has been
// canceled (Run started and its context is done). A service that never ran
// (lifecycleCtx == nil) still allows on-demand refresh workers. Callers must
// hold backgroundMu.
func (s *ClassroomService) isLifecycleCanceledLocked() bool {
	return s.lifecycleCtx != nil && s.lifecycleCtx.Err() != nil
}

// currentLifecycle snapshots the lifecycle context under backgroundMu. Nil
// means Run has not been called (no cancellation bridging for workers).
func (s *ClassroomService) currentLifecycle() context.Context {
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()
	return s.lifecycleCtx
}

func (s *ClassroomService) warmupLoop(ctx context.Context) {
	failures := 0
	for {
		if ctx.Err() != nil {
			return
		}
		result, completed := s.runWarmupOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		failures = nextWarmupFailureCount(failures, result, completed)

		now := s.now()
		delay := nextWarmupDelay(
			now,
			s.warmupCacheState(now),
			s.nextRefreshAllowedAt(),
			failures,
			s.warmupJitter(),
		)
		if !waitForWarmup(ctx, delay) {
			return
		}
	}
}

func (s *ClassroomService) runWarmupOnce(ctx context.Context) (classroomRefreshResult, bool) {
	attempt, started := s.startClassroomRefresh(ctx, s.now())
	if !started {
		return classroomRefreshResult{}, false
	}
	select {
	case <-attempt.done:
		return attempt.result, true
	case <-ctx.Done():
		return classroomRefreshResult{}, false
	}
}

func (s *ClassroomService) warmupCacheState(now time.Time) warmupCacheState {
	cached, ok := s.getCachedTodayClassroomsAt(now)
	if !ok || !now.Before(cached.StaleUntil) {
		return warmupNoCache
	}
	if len(cached.PartialCampuses) > 0 || cached.Error != nil {
		return warmupPartialCache
	}
	return warmupFullCache
}

func waitForWarmup(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// Shutdown cancels the lifecycle, waits for the scheduler to exit, then
// drains every refresh worker, all bounded by ctx. Cancelling under
// backgroundMu prevents later Add calls from racing with the WaitGroup wait;
// startClassroomRefresh checks the same lock and rejects new workers. It is
// idempotent and safe on a service that never ran Run: it just drains.
func (s *ClassroomService) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.backgroundMu.Lock()
	cancel := s.lifecycleCancel
	done := s.schedulerDone
	if cancel != nil {
		cancel()
	}
	s.backgroundMu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	workersDone := make(chan struct{})
	go func() {
		s.refreshWorkers.Wait()
		close(workersDone)
	}()
	select {
	case <-workersDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
