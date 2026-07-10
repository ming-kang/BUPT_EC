package service

import (
	"context"
	"math/rand/v2"
	"time"
)

const (
	warmupRetryInitial = 30 * time.Second
	warmupRetrySecond  = time.Minute
	warmupRetryThird   = 2 * time.Minute
	warmupRetryMax     = 5 * time.Minute
	warmupJitterMin    = time.Second
	warmupJitterRange  = 4 * time.Second
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

func warmupFailureDelay(failures int) time.Duration {
	switch failures {
	case 0, 1:
		return warmupRetryInitial
	case 2:
		return warmupRetrySecond
	case 3:
		return warmupRetryThird
	default:
		return warmupRetryMax
	}
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
		target = now.Add(warmupFailureDelay(failures))
		if nextAllowed.After(target) {
			target = nextAllowed
		}
	}

	if delay := target.Sub(now); delay > 0 {
		return delay
	}
	return 0
}

// StartWarmup starts one context-cancellable scheduler for this service.
// The first refresh is attempted immediately; duplicate calls are no-ops.
func (s *ClassroomService) StartWarmup(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.backgroundMu.Lock()
	if s.backgroundStopping || s.warmupStarted {
		s.backgroundMu.Unlock()
		return
	}
	warmupCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.warmupStarted = true
	s.warmupCancel = cancel
	s.warmupDone = done
	s.backgroundMu.Unlock()

	go func() {
		defer close(done)
		s.warmupLoop(warmupCtx)
	}()
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

// WaitBackground stops the scheduler, waits for it to exit, then drains every
// refresh worker. Setting backgroundStopping before Wait prevents later Add
// calls from racing with the WaitGroup wait.
func (s *ClassroomService) WaitBackground(ctx context.Context) error {
	s.backgroundMu.Lock()
	s.backgroundStopping = true
	cancel := s.warmupCancel
	done := s.warmupDone
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
